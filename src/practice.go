package main

import (
	"errors"
	"math/rand/v2"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInsufficientEvidence = errors.New("insufficient practice evidence")
	ErrNoNeutralPack        = errors.New("no suitable neutral pack")
	ErrContextTooLong       = errors.New("practice context is too long")
)

type PracticeItem struct {
	Item               string   `json:"item"`
	Context            string   `json:"context"`
	Reason             string   `json:"reason"`
	Occurrences        int      `json:"occurrences"`
	Attempts           int      `json:"attempts"`
	CorrectAttempts    int      `json:"correctAttempts"`
	Errors             int      `json:"errors"`
	MedianMSPerScalar  *float64 `json:"medianMSPerScalar"`
	CurrentOccurrences int      `json:"currentOccurrences"`
	HistoryOccurrences int      `json:"historyOccurrences"`
}

type PracticePlan struct {
	ParentID          string         `json:"parentId"`
	Items             []PracticeItem `json:"items"`
	Tokens            []string       `json:"tokens"`
	WindowStartUnixMS int64          `json:"windowStartUnixMS"`
	SampleSessions    int            `json:"sampleSessions"`
	Origin            ResourceOrigin `json:"-"`
}

type PracticeComparison struct {
	Item               string       `json:"item"`
	Baseline           PracticeItem `json:"baseline"`
	Drill              PracticeItem `json:"drill"`
	AccuracyDelta      *float64     `json:"accuracyDelta"`
	SpeedChangePercent *float64     `json:"speedChangePercent"`
}

type PracticeDetail struct {
	ParentID          string               `json:"parentId"`
	WindowStartUnixMS int64                `json:"windowStartUnixMS"`
	SampleSessions    int                  `json:"sampleSessions"`
	Items             []PracticeItem       `json:"items"`
	Comparisons       []PracticeComparison `json:"comparisons"`
}

type practiceOccurrence struct {
	activeMS int64
	scalars  int
	attempts int
	correct  int
	errors   int
	complete bool
}

type practiceCandidate struct {
	item      string
	context   string
	current   []practiceOccurrence
	history   []practiceOccurrence
	all       []practiceOccurrence
	wrongNow  bool
	median    *float64
	errorRate float64
}

const (
	practiceHistoryDays  = 30
	practiceHistoryMax   = 20
	practiceMaxItems     = 5
	practiceWeakSlots    = 15
	practiceNeutralSlots = 10
)

func SelectPractice(current SessionResult, recent []HistoryRecord) (PracticePlan, error) {
	if current.ID == "" || (current.Config.Source != wordSource && current.Config.Source != quoteSource) || current.Config.Pack == "" {
		return PracticePlan{}, ErrInsufficientEvidence
	}
	windowStart := current.FinishedUnixMS - int64((practiceHistoryDays*24*time.Hour)/time.Millisecond)
	if current.FinishedUnixMS <= 0 {
		windowStart = 0
	}
	candidates := make(map[string]*practiceCandidate)
	runes := []rune(current.Prompt)
	for _, observation := range current.Measurements.Words {
		if observation.Start < 0 || observation.End <= observation.Start || observation.End > len(runes) {
			continue
		}
		item := string(runes[observation.Start:observation.End])
		if item == "" {
			continue
		}
		o := practiceOccurrence{activeMS: observation.ActiveMS, scalars: utf8.RuneCountInString(item), attempts: observation.Attempts, correct: observation.CorrectAttempts, errors: observation.Errors, complete: observation.Complete}
		candidate := candidates[item]
		if candidate == nil {
			context := item
			if current.Config.Mode == quoteMode {
				context = practiceContext(current.Prompt, observation.Start, observation.End)
			}
			candidate = &practiceCandidate{item: item, context: context}
			candidates[item] = candidate
		}
		candidate.current = append(candidate.current, o)
		if observation.Errors > 0 {
			candidate.wrongNow = true
		}
	}

	// Only regular, public records from the same embedded pack participate in history.
	filtered := make([]HistoryRecord, 0, len(recent))
	for _, record := range recent {
		if record.ID == current.ID || record.Practice || (record.Outcome != "completed" && record.Outcome != "expired") || record.Privacy != "public" || record.PackID != current.Config.Pack || record.Mode != string(current.Config.Mode) ||
			(record.SourceKind != "embedded-word" && record.SourceKind != "embedded-quote") || record.FinishedUnixMS < windowStart || (current.FinishedUnixMS > 0 && record.FinishedUnixMS > current.FinishedUnixMS) {
			continue
		}
		filtered = append(filtered, record)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].FinishedUnixMS != filtered[j].FinishedUnixMS {
			return filtered[i].FinishedUnixMS > filtered[j].FinishedUnixMS
		}
		return filtered[i].ID > filtered[j].ID
	})
	if len(filtered) > 0 {
		revision := filtered[0].PackRevision
		if revision != "" {
			sameRevision := filtered[:0]
			for _, record := range filtered {
				if record.PackRevision == revision {
					sameRevision = append(sameRevision, record)
				}
			}
			filtered = sameRevision
		}
	}
	if len(filtered) > practiceHistoryMax {
		filtered = filtered[:practiceHistoryMax]
	}
	for _, record := range filtered {
		for _, fragment := range record.Fragments {
			if fragment.Item == "" {
				continue
			}
			candidate := candidates[fragment.Item]
			if candidate == nil {
				candidate = &practiceCandidate{item: fragment.Item, context: fragment.Context}
				candidates[fragment.Item] = candidate
			}
			if candidate.context == "" && fragment.Context != "" {
				candidate.context = fragment.Context
			}
			for _, observation := range fragment.Observations {
				candidate.history = append(candidate.history, practiceOccurrence{activeMS: observation.ActiveMS, scalars: observation.Scalars, attempts: observation.Attempts, correct: observation.CorrectAttempts, errors: observation.Errors, complete: true})
			}
			if len(fragment.Observations) == 0 && fragment.Errors > 0 {
				// Old v1 fragments prove wrong attempts but not a complete word.
				// Keep their counts without inventing an occurrence or speed.
				candidate.history = append(candidate.history, practiceOccurrence{
					attempts: fragment.Attempts, correct: fragment.CorrectAttempts,
					errors: fragment.Errors,
				})
			}
		}
	}
	for _, candidate := range candidates {
		candidate.all = append(candidate.all, candidate.current...)
		candidate.all = append(candidate.all, candidate.history...)
		candidate.median = occurrenceMedian(candidate.all)
		attempts, errorsCount := 0, 0
		for _, o := range candidate.all {
			attempts += o.attempts
			errorsCount += o.errors
		}
		if attempts > 0 {
			candidate.errorRate = float64(errorsCount) / float64(attempts)
		}
	}
	sampleValues := make([]float64, 0)
	for _, candidate := range candidates {
		for _, occurrence := range candidate.all {
			if occurrence.complete && occurrence.activeMS >= 0 && occurrence.scalars > 0 {
				sampleValues = append(sampleValues, float64(occurrence.activeMS)/float64(occurrence.scalars))
			}
		}
	}
	sampleMedian := median(sampleValues)
	for _, candidate := range candidates {
		complete := completeCount(candidate.all)
		hasErrors := false
		for _, occurrence := range candidate.all {
			hasErrors = hasErrors || occurrence.errors > 0
		}
		slow := complete >= 3 && candidate.median != nil && sampleMedian != nil && *candidate.median >= 1.5**sampleMedian
		if !((hasErrors && (complete >= 2 || candidate.wrongNow)) || (!hasErrors && slow)) {
			delete(candidates, candidate.item)
			continue
		}
		if candidate.context == "" {
			candidate.context = candidate.item
		}
	}
	if len(candidates) == 0 {
		return PracticePlan{}, ErrInsufficientEvidence
	}
	ordered := make([]*practiceCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		ordered = append(ordered, candidate)
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].errorRate != ordered[j].errorRate {
			return ordered[i].errorRate > ordered[j].errorRate
		}
		mi, mj := medianValue(ordered[i].median), medianValue(ordered[j].median)
		if mi != mj {
			return mi > mj
		}
		return ordered[i].item < ordered[j].item
	})
	if len(ordered) > practiceMaxItems {
		ordered = ordered[:practiceMaxItems]
	}
	items := make([]PracticeItem, 0, len(ordered))
	for _, candidate := range ordered {
		item := PracticeItem{Item: candidate.item, Context: candidate.context, Occurrences: completeCount(candidate.all), Attempts: 0, CorrectAttempts: 0, Errors: 0, MedianMSPerScalar: candidate.median, CurrentOccurrences: completeCount(candidate.current), HistoryOccurrences: completeCount(candidate.history)}
		for _, occurrence := range candidate.all {
			item.Attempts += occurrence.attempts
			item.CorrectAttempts += occurrence.correct
			item.Errors += occurrence.errors
		}
		if item.Errors > 0 {
			item.Reason = "error"
		} else {
			item.Reason = "slow"
		}
		items = append(items, item)
	}
	plan := PracticePlan{ParentID: current.ID, Items: items, WindowStartUnixMS: windowStart, SampleSessions: len(filtered) + 1}
	return plan, nil
}

func BuildPractice(p PracticePlan, neutral []string, seed int64) (*Test, error) {
	if len(p.Items) == 0 || len(neutral) == 0 {
		return nil, ErrNoNeutralPack
	}
	for _, word := range neutral {
		if len(strings.Fields(word)) != 1 || word == "" {
			return nil, ErrNoNeutralPack
		}
	}
	weakBlocks := make([][]string, 0, practiceWeakSlots)
	for offset := 0; totalWords(weakBlocks) < practiceWeakSlots; {
		item := p.Items[offset%len(p.Items)]
		offset++
		context := strings.Fields(item.Context)
		if len(context) == 0 {
			context = []string{item.Item}
		}
		if len(context) > 5 {
			return nil, ErrContextTooLong
		}
		remaining := practiceWeakSlots - totalWords(weakBlocks)
		if len(context) <= remaining {
			weakBlocks = append(weakBlocks, context)
			continue
		}
		fallback := strings.Fields(item.Item)
		if len(fallback) != 1 || len(fallback) > remaining {
			return nil, ErrContextTooLong
		}
		weakBlocks = append(weakBlocks, fallback)
	}
	neutralBlocks := make([][]string, practiceNeutralSlots)
	for i := range neutralBlocks {
		neutralBlocks[i] = []string{neutral[i%len(neutral)]}
	}
	blocks := append(append([][]string{}, weakBlocks...), neutralBlocks...)
	rng := rand.New(rand.NewPCG(uint64(seed), uint64(seed)^0x9e3779b97f4a7c15))
	for attempt := 0; attempt < 128; attempt++ {
		rng.Shuffle(len(blocks), func(i, j int) { blocks[i], blocks[j] = blocks[j], blocks[i] })
		if blocksHaveDistinctBoundaries(blocks) {
			break
		}
	}
	tokens := make([]string, 0, practiceWeakSlots+practiceNeutralSlots)
	for _, block := range blocks {
		tokens = append(tokens, block...)
	}
	if len(tokens) != 25 {
		return nil, ErrContextTooLong
	}
	origin := p.Origin
	if origin.Kind == "embedded-quote" || origin.Kind == "" {
		_, resolved, err := ResolveResource("words", "1000en")
		if err != nil {
			if origin.Kind == "" {
				return nil, ErrNoNeutralPack
			}
		} else {
			origin = resolved
		}
	}
	cfg := TestConfig{Mode: wordMode, Source: wordSource, Pack: origin.PackID, TimeLimit: -1, WordCount: len(tokens), WordsPerGroup: len(tokens), Groups: 1, Difficulty: "normal"}
	if cfg.Pack == "" {
		return nil, ErrNoNeutralPack
	}
	return &Test{Config: cfg, SourceID: "practice:" + p.ParentID, Origin: origin, Segments: []segment{{Text: strings.Join(tokens, " ")}}, EligibleForHistory: true, EligibleForPB: false}, nil
}

func ComparePractice(p PracticePlan, result SessionResult) []PracticeComparison {
	words := result.Measurements.Words
	runes := []rune(result.Prompt)
	comparisons := make([]PracticeComparison, 0, len(p.Items))
	for _, baseline := range p.Items {
		drill := PracticeItem{Item: baseline.Item}
		values := make([]float64, 0)
		for _, word := range words {
			if word.Start < 0 || word.End <= word.Start || word.End > len(runes) || string(runes[word.Start:word.End]) != baseline.Item {
				continue
			}
			if word.Complete {
				drill.Occurrences++
				if word.ActiveMS >= 0 && utf8.RuneCountInString(baseline.Item) > 0 {
					values = append(values, float64(word.ActiveMS)/float64(utf8.RuneCountInString(baseline.Item)))
				}
			}
			drill.Attempts += word.Attempts
			drill.CorrectAttempts += word.CorrectAttempts
			drill.Errors += word.Errors
		}
		drill.MedianMSPerScalar = median(values)
		var accuracyDelta *float64
		if baseline.Attempts > 0 && drill.Attempts > 0 {
			v := float64(drill.CorrectAttempts)*100/float64(drill.Attempts) - float64(baseline.CorrectAttempts)*100/float64(baseline.Attempts)
			accuracyDelta = &v
		}
		var speedChange *float64
		if baseline.Occurrences >= 3 && drill.Occurrences >= 3 && baseline.MedianMSPerScalar != nil && *baseline.MedianMSPerScalar > 0 && drill.MedianMSPerScalar != nil {
			v := (*baseline.MedianMSPerScalar - *drill.MedianMSPerScalar) / *baseline.MedianMSPerScalar * 100
			speedChange = &v
		}
		comparisons = append(comparisons, PracticeComparison{Item: baseline.Item, Baseline: baseline, Drill: drill, AccuracyDelta: accuracyDelta, SpeedChangePercent: speedChange})
	}
	return comparisons
}

func practiceContext(prompt string, start, end int) string {
	words := strings.Fields(prompt)
	if len(words) == 0 {
		return ""
	}
	runes := []rune(prompt)
	before := strings.Fields(string(runes[:start]))
	idx := len(before)
	if idx >= len(words) {
		idx = len(words) - 1
	}
	lo := idx - 2
	if lo < 0 {
		lo = 0
	}
	hi := lo + 5
	if hi > len(words) {
		hi = len(words)
	}
	return strings.Join(words[lo:hi], " ")
}

func occurrenceMedian(occurrences []practiceOccurrence) *float64 {
	values := make([]float64, 0, len(occurrences))
	for _, occurrence := range occurrences {
		if occurrence.complete && occurrence.activeMS >= 0 && occurrence.scalars > 0 {
			values = append(values, float64(occurrence.activeMS)/float64(occurrence.scalars))
		}
	}
	return median(values)
}
func median(values []float64) *float64 {
	if len(values) == 0 {
		return nil
	}
	sort.Float64s(values)
	v := values[len(values)/2]
	if len(values)%2 == 0 {
		v = (values[len(values)/2-1] + values[len(values)/2]) / 2
	}
	return &v
}
func medianValue(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}
func completeCount(values []practiceOccurrence) int {
	n := 0
	for _, value := range values {
		if value.complete {
			n++
		}
	}
	return n
}
func totalWords(blocks [][]string) int {
	n := 0
	for _, block := range blocks {
		n += len(block)
	}
	return n
}
func blocksHaveDistinctBoundaries(blocks [][]string) bool {
	for i := 1; i < len(blocks); i++ {
		if len(blocks[i-1]) > 0 && len(blocks[i]) > 0 && blocks[i-1][len(blocks[i-1])-1] == blocks[i][0] {
			return false
		}
	}
	return true
}
