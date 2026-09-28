package main

import (
	"errors"
	"fmt"
	"math"
	"unicode"
)

const metricVersion = "tt-metrics/1"

var ErrInvalidEvents = errors.New("invalid session events")

type CharacterTotals struct {
	Correct   int `json:"correct"`
	Incorrect int `json:"incorrect"`
	Extra     int `json:"extra"`
	Missed    int `json:"missed"`
}

type ErrorTotals struct {
	Total       int `json:"total"`
	Corrected   int `json:"corrected"`
	Uncorrected int `json:"uncorrected"`
}

type MetricSample struct {
	EndMS    int64    `json:"end_ms"`
	WindowMS int64    `json:"window_ms"`
	WPM      *float64 `json:"wpm"`
	RawWPM   *float64 `json:"raw_wpm"`
	Errors   int      `json:"errors"`
}

type WordObservation struct {
	Start           int   `json:"start"`
	End             int   `json:"end"`
	ActiveMS        int64 `json:"active_ms"`
	Attempts        int   `json:"attempts"`
	CorrectAttempts int   `json:"correct_attempts"`
	Errors          int   `json:"errors"`
	Complete        bool  `json:"complete"`
}

type Measurements struct {
	WPM             *float64          `json:"wpm"`
	RawWPM          *float64          `json:"raw_wpm"`
	CPM             *float64          `json:"cpm"`
	Accuracy        *float64          `json:"accuracy"`
	Consistency     *float64          `json:"consistency"`
	ActiveMS        int64             `json:"active_ms"`
	PauseMS         int64             `json:"pause_ms"`
	Characters      CharacterTotals   `json:"characters"`
	Errors          ErrorTotals       `json:"errors"`
	Attempts        int               `json:"attempts"`
	CorrectAttempts int               `json:"correct_attempts"`
	Series          []MetricSample    `json:"series"`
	Words           []WordObservation `json:"words"`
}

type SessionResult struct {
	ID                 string       `json:"id"`
	PromptID           string       `json:"prompt_id"`
	RetryOf            string       `json:"retry_of,omitempty"`
	MetricVersion      string       `json:"metric_version"`
	FinishedUnixMS     int64        `json:"finished_unix_ms"`
	Config             TestConfig   `json:"config"`
	Outcome            string       `json:"outcome"`
	AllowBackspace     bool         `json:"allow_backspace"`
	SkipWord           bool         `json:"skip_word"`
	Practice           bool         `json:"practice"`
	Measurements       Measurements `json:"measurements"`
	EligibilityReasons []string     `json:"eligibility_reasons"`
	Attribution        string       `json:"attribution,omitempty"`
}

func rate(count int, activeNS, divisor float64) *float64 {
	if activeNS <= 0 {
		return nil
	}
	v := float64(count) * 60e9 / (divisor * activeNS)
	return &v
}

// Measure projects final character outcomes and accepted printable attempts
// from a session. Newlines are structural and never measured.
func Measure(s Session) (Measurements, error) {
	if s.ActiveNS < 0 || s.PauseNS < 0 {
		return Measurements{}, ErrInvalidEvents
	}
	m := Measurements{ActiveMS: s.ActiveNS / 1e6, PauseMS: s.PauseNS / 1e6, Series: []MetricSample{}}
	if s.State != SessionCompleted && s.State != SessionExpired {
		m.Characters.Correct = s.liveCorrect
		for _, extra := range s.ExtraByWord {
			m.Characters.Extra += len(extra)
		}
		m.Attempts = s.liveAttempts
		m.CorrectAttempts = s.liveCorrectAttempts
		m.Errors = ErrorTotals{
			Total:       s.liveErrors,
			Corrected:   s.liveCorrectedErrors,
			Uncorrected: s.liveErrors - s.liveCorrectedErrors,
		}
		m.WPM = rate(m.Characters.Correct, float64(s.ActiveNS), 5)
		m.RawWPM = rate(m.Attempts, float64(s.ActiveNS), 5)
		m.CPM = rate(m.Characters.Correct, float64(s.ActiveNS), 1)
		if m.Attempts > 0 {
			v := float64(m.CorrectAttempts) * 100 / float64(m.Attempts)
			m.Accuracy = &v
		}
		return m, nil
	}
	p := s.promptRunes
	for i, want := range p {
		if want == '\n' {
			continue
		}
		if i < s.Cursor && i < len(s.Typed) && s.Typed[i] == want {
			m.Characters.Correct++
		} else if i < s.Cursor && i < len(s.Typed) && s.Typed[i] != 0 {
			m.Characters.Incorrect++
		} else if i < s.Cursor || s.State == SessionCompleted {
			m.Characters.Missed++
		}
	}
	for _, extra := range s.ExtraByWord {
		m.Characters.Extra += len(extra)
	}
	errorSlots := make(map[int]bool)
	extraErrorSlots := make(map[int]int)
	skipped := 0
	for i, ev := range s.Events {
		if ev.Seq != uint64(i+1) || ev.ActiveNS < 0 || ev.ActiveNS > s.ActiveNS || (i > 0 && ev.ActiveNS < s.Events[i-1].ActiveNS) {
			return Measurements{}, ErrInvalidEvents
		}
		switch ev.Kind {
		case InputText:
			runes := []rune(ev.Text)
			if len(runes) != 1 || ev.CursorBefore < 0 || ev.CursorBefore >= len(p) || p[ev.CursorBefore] == '\n' {
				return Measurements{}, ErrInvalidEvents
			}
			slot := ev.CursorBefore
			overflow := p[slot] == ' ' && runes[0] != ' ' && slot > 0 && !unicode.IsSpace(p[slot-1])
			m.Attempts++
			if overflow {
				m.Errors.Total++
				extraErrorSlots[slot]++
				continue
			}
			if errorSlots[slot] {
				m.Errors.Corrected++
				delete(errorSlots, slot)
			}
			if runes[0] == p[slot] {
				m.CorrectAttempts++
			} else {
				m.Errors.Total++
				errorSlots[slot] = true
			}
		case InputBackspace:
			if ev.CursorAfter == ev.CursorBefore && extraErrorSlots[ev.CursorBefore] > 0 {
				extraErrorSlots[ev.CursorBefore]--
				m.Errors.Corrected++
			} else {
				for slot := ev.CursorAfter; slot < ev.CursorBefore; slot++ {
					if errorSlots[slot] {
						m.Errors.Corrected++
						delete(errorSlots, slot)
					}
				}
			}
		case InputDeleteWord:
			if extraErrorSlots[ev.CursorBefore] > 0 {
				m.Errors.Corrected += extraErrorSlots[ev.CursorBefore]
				delete(extraErrorSlots, ev.CursorBefore)
			}
			for slot := ev.CursorAfter; slot < ev.CursorBefore; slot++ {
				if errorSlots[slot] {
					m.Errors.Corrected++
					delete(errorSlots, slot)
				}
			}
		case InputSkip:
			for slot := ev.CursorBefore; slot < ev.CursorAfter && slot < len(p); slot++ {
				if p[slot] != ' ' && p[slot] != '\n' {
					if errorSlots[slot] {
						m.Errors.Corrected++
						delete(errorSlots, slot)
					}
					skipped++
					errorSlots[slot] = true
				}
			}
			if ev.CursorAfter > ev.CursorBefore && ev.CursorAfter <= len(p) && p[ev.CursorAfter-1] == ' ' {
				m.Attempts++
				m.CorrectAttempts++
			}
		case InputPause, InputResume, InputNext, InputPrevious:
		default:
			return Measurements{}, ErrInvalidEvents
		}
	}
	unrecordedMissed := m.Characters.Missed - skipped
	if unrecordedMissed < 0 {
		unrecordedMissed = 0
	}
	m.Errors.Total += skipped + unrecordedMissed
	m.Errors.Uncorrected = m.Errors.Total - m.Errors.Corrected
	if m.Errors.Uncorrected < 0 {
		return Measurements{}, ErrInvalidEvents
	}
	m.WPM = rate(m.Characters.Correct, float64(s.ActiveNS), 5)
	m.RawWPM = rate(m.Attempts, float64(s.ActiveNS), 5)
	m.CPM = rate(m.Characters.Correct, float64(s.ActiveNS), 1)
	if m.Attempts > 0 {
		v := float64(m.CorrectAttempts) * 100 / float64(m.Attempts)
		m.Accuracy = &v
	}
	m.Series, m.Consistency = metricSeries(s)
	m.Words = wordObservations(s)
	return m, nil
}

func metricSeries(s Session) ([]MetricSample, *float64) {
	const windowNS = int64(1e9)
	if s.ActiveNS < int64(250e6) {
		return []MetricSample{}, nil
	}
	full := int(s.ActiveNS / windowNS)
	remainder := s.ActiveNS % windowNS
	count := full
	if remainder >= int64(250e6) {
		count++
	}
	attempts := make([]int, count)
	correct := make([]int, count)
	errorsByWindow := make([]int, count)
	for _, ev := range s.Events {
		index := int(ev.ActiveNS / windowNS)
		if index >= count {
			index = count - 1
		}
		if ev.Kind == InputSkip {
			for slot := ev.CursorBefore; slot < ev.CursorAfter && slot < len(s.promptRunes); slot++ {
				if s.promptRunes[slot] != ' ' && s.promptRunes[slot] != '\n' {
					errorsByWindow[index]++
				}
			}
			if ev.CursorAfter > ev.CursorBefore && ev.CursorAfter <= len(s.promptRunes) && s.promptRunes[ev.CursorAfter-1] == ' ' {
				attempts[index]++
				correct[index]++
			}
			continue
		}
		if ev.Kind != InputText {
			continue
		}
		runes := []rune(ev.Text)
		if len(runes) != 1 || ev.CursorBefore < 0 || ev.CursorBefore >= len(s.promptRunes) {
			continue
		}
		attempts[index]++
		if runes[0] == s.promptRunes[ev.CursorBefore] {
			correct[index]++
		} else {
			errorsByWindow[index]++
		}
	}
	series := make([]MetricSample, 0, count)
	rawRates := make([]float64, 0, count)
	for i := 0; i < count; i++ {
		duration := windowNS
		if i == full && remainder > 0 && remainder >= int64(250e6) {
			duration = remainder
		}
		endNS := int64(i+1) * windowNS
		if endNS > s.ActiveNS {
			endNS = s.ActiveNS
		}
		sample := MetricSample{EndMS: endNS / 1e6, WindowMS: duration / 1e6, Errors: errorsByWindow[i]}
		sample.WPM = rate(correct[i], float64(duration), 5)
		sample.RawWPM = rate(attempts[i], float64(duration), 5)
		if sample.RawWPM != nil {
			rawRates = append(rawRates, *sample.RawWPM)
		} else {
			rawRates = append(rawRates, 0)
		}
		series = append(series, sample)
	}
	if len(rawRates) < 2 {
		return series, nil
	}
	mean := 0.0
	for _, rate := range rawRates {
		mean += rate
	}
	mean /= float64(len(rawRates))
	if mean == 0 {
		return series, nil
	}
	variance := 0.0
	for _, rate := range rawRates {
		delta := rate - mean
		variance += delta * delta
	}
	consistency := math.Max(0, 100*(1-math.Sqrt(variance/float64(len(rawRates)))/mean))
	return series, &consistency
}

func roundHalfUp(value float64) float64 {
	return math.Floor(value*100+0.500000000001) / 100
}

func metricDisplay(value *float64, suffix string) string {
	if value == nil {
		return "—"
	}
	return fmt.Sprintf("%.2f%s", roundHalfUp(*value), suffix)
}

func wordObservations(s Session) []WordObservation {
	prompt := s.promptRunes
	words := make([]WordObservation, 0)
	wordByScalar := make([]int, len(prompt))
	for i := range wordByScalar {
		wordByScalar[i] = -1
	}
	for start := 0; start < len(prompt); {
		for start < len(prompt) && unicode.IsSpace(prompt[start]) {
			start++
		}
		if start == len(prompt) {
			break
		}
		end := start
		for end < len(prompt) && !unicode.IsSpace(prompt[end]) {
			end++
		}
		index := len(words)
		words = append(words, WordObservation{Start: start, End: end, Complete: s.Cursor >= end})
		for slot := start; slot < end; slot++ {
			wordByScalar[slot] = index
		}
		start = end
	}
	firstNS := make([]int64, len(words))
	lastNS := make([]int64, len(words))
	for i := range firstNS {
		firstNS[i] = -1
	}
	for _, ev := range s.Events {
		if ev.Kind == InputSkip {
			for slot := ev.CursorBefore; slot < ev.CursorAfter && slot < len(wordByScalar); slot++ {
				index := wordByScalar[slot]
				if index >= 0 {
					words[index].Errors++
					if firstNS[index] < 0 {
						firstNS[index] = ev.ActiveNS
					}
					lastNS[index] = ev.ActiveNS
				}
			}
			if ev.CursorAfter > 1 && ev.CursorAfter <= len(prompt) && prompt[ev.CursorAfter-1] == ' ' {
				index := wordByScalar[ev.CursorAfter-2]
				if index >= 0 {
					if firstNS[index] < 0 {
						firstNS[index] = ev.ActiveNS
					}
					lastNS[index] = ev.ActiveNS
				}
			}
			continue
		}
		if ev.Kind != InputText || ev.CursorBefore < 0 || ev.CursorBefore >= len(wordByScalar) {
			continue
		}
		runes := []rune(ev.Text)
		index := wordByScalar[ev.CursorBefore]
		if index < 0 && ev.CursorBefore > 0 && prompt[ev.CursorBefore] == ' ' {
			index = wordByScalar[ev.CursorBefore-1]
			if index >= 0 {
				if firstNS[index] < 0 {
					firstNS[index] = ev.ActiveNS
				}
				lastNS[index] = ev.ActiveNS
				if len(runes) == 1 && runes[0] == ' ' {
					continue
				}
			}
		}
		if index < 0 {
			continue
		}
		word := &words[index]
		word.Attempts++
		if len(runes) == 1 && runes[0] == prompt[ev.CursorBefore] {
			word.CorrectAttempts++
		} else {
			word.Errors++
		}
		if firstNS[index] < 0 {
			firstNS[index] = ev.ActiveNS
		}
		lastNS[index] = ev.ActiveNS
	}
	for i := range words {
		if firstNS[i] >= 0 {
			words[i].ActiveMS = (lastNS[i] - firstNS[i]) / 1e6
		}
	}
	return words
}

func FinishResult(s Session, finishedUnixMS int64) (SessionResult, error) {
	if s.State != SessionCompleted && s.State != SessionExpired {
		return SessionResult{}, ErrInvalidEvents
	}
	measurements, err := Measure(s)
	if err != nil {
		return SessionResult{}, err
	}
	result := SessionResult{
		ID: s.AttemptID, PromptID: s.PromptID, RetryOf: s.RetryOf,
		MetricVersion: metricVersion, FinishedUnixMS: finishedUnixMS,
		Config: TestConfig{}, Outcome: string(s.State), Practice: s.RetryOf != "",
		AllowBackspace: s.AllowBackspace, SkipWord: s.SkipWord,
		Measurements: measurements, EligibilityReasons: []string{},
	}
	if s.Test != nil {
		result.Config = s.Test.Config
		result.Attribution = s.Test.Attribution
		result.Practice = result.Practice || !s.Test.EligibleForPB
	}
	if s.RetryOf != "" {
		result.EligibilityReasons = append(result.EligibilityReasons, "retry")
	}
	if s.Test != nil && !s.Test.EligibleForPB {
		result.EligibilityReasons = append(result.EligibilityReasons, "test is not PB-eligible")
	}
	return result, nil
}
