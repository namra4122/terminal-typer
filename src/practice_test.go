package main

import (
	"errors"
	"strings"
	"testing"
)

func practiceResultFixture() SessionResult {
	prompt := "alpha beta alpha beta gamma"
	return SessionResult{
		ID: "current", FinishedUnixMS: 40 * 24 * 60 * 60 * 1000,
		Config: TestConfig{Mode: wordMode, Source: wordSource, Pack: "1000en"}, Prompt: prompt,
		Measurements: Measurements{Words: []WordObservation{
			{Start: 0, End: 5, ActiveMS: 500, Attempts: 3, CorrectAttempts: 1, Errors: 2, Complete: true},
			{Start: 6, End: 10, ActiveMS: 100, Attempts: 1, CorrectAttempts: 1, Complete: true},
			{Start: 11, End: 16, ActiveMS: 450, Attempts: 2, CorrectAttempts: 1, Errors: 1, Complete: true},
			{Start: 17, End: 21, ActiveMS: 100, Attempts: 1, CorrectAttempts: 1, Complete: true},
			{Start: 22, End: 27, ActiveMS: 200, Attempts: 1, CorrectAttempts: 1, Complete: true},
		}},
	}
}

func TestSelectPracticeRanksCurrentErrorsAndExplainsEvidence(t *testing.T) {
	plan, err := SelectPractice(practiceResultFixture(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Items) != 1 || plan.Items[0].Item != "alpha" || plan.Items[0].Reason != "error" {
		t.Fatalf("selected items = %#v", plan.Items)
	}
	if plan.Items[0].CurrentOccurrences != 2 || plan.Items[0].Occurrences != 2 || plan.Items[0].Errors != 3 {
		t.Fatalf("alpha evidence = %#v", plan.Items[0])
	}
	if plan.SampleSessions != 1 || plan.ParentID != "current" {
		t.Fatalf("sample metadata = %#v", plan)
	}
}

func TestSelectPracticeRejectsPrivateSource(t *testing.T) {
	result := practiceResultFixture()
	result.Config.Source = fileSource
	result.Config.Pack = "/private/secret"
	if _, err := SelectPractice(result, nil); !errors.Is(err, ErrInsufficientEvidence) {
		t.Fatalf("private selection error = %v", err)
	}
}

func TestSelectPracticeKeepsFiveWordQuoteContext(t *testing.T) {
	result := SessionResult{
		ID: "quote", FinishedUnixMS: 1_000_000, Config: TestConfig{Mode: quoteMode, Source: quoteSource, Pack: "en"},
		Prompt: "one, two three four three",
		Measurements: Measurements{
			Words: []WordObservation{
				{Start: 0, End: 4, ActiveMS: 100, Attempts: 1, CorrectAttempts: 1, Complete: true},
				{Start: 5, End: 8, ActiveMS: 100, Attempts: 1, CorrectAttempts: 1, Complete: true},
				{Start: 9, End: 14, ActiveMS: 100, Attempts: 2, CorrectAttempts: 1, Errors: 1, Complete: true},
				{Start: 20, End: 25, ActiveMS: 100, Attempts: 1, CorrectAttempts: 1, Complete: true},
			},
		},
	}
	plan, err := SelectPractice(result, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range plan.Items {
		if len(strings.Fields(item.Context)) > 5 || !strings.Contains(item.Context, item.Item) {
			t.Fatalf("quote context = %#v", item)
		}
	}
}

func TestBuildPracticeUsesExactWeakNeutralRatio(t *testing.T) {
	plan := PracticePlan{ParentID: "parent", Items: []PracticeItem{{Item: "alpha", Context: "alpha"}, {Item: "beta", Context: "beta"}}}
	neutral := []string{"neutral0", "neutral1", "neutral2", "neutral3", "neutral4", "neutral5", "neutral6", "neutral7", "neutral8", "neutral9"}
	test, err := BuildPractice(plan, neutral, 19)
	if err != nil {
		t.Fatal(err)
	}
	words := strings.Fields(test.Segments[0].Text)
	if len(words) != 25 || !test.EligibleForHistory || test.EligibleForPB {
		t.Fatalf("practice test = %#v words=%d", test, len(words))
	}
	weak, neutralCount := 0, 0
	for _, word := range words {
		if word == "alpha" || word == "beta" {
			weak++
		}
		for _, n := range neutral {
			if word == n {
				neutralCount++
				break
			}
		}
	}
	if weak != 15 || neutralCount != 10 {
		t.Fatalf("slot ratio weak=%d neutral=%d words=%v", weak, neutralCount, words)
	}
}

func TestComparePracticeLeavesSparseSpeedUnavailable(t *testing.T) {
	plan := PracticePlan{Items: []PracticeItem{{Item: "alpha", Occurrences: 3, Attempts: 3, CorrectAttempts: 3, MedianMSPerScalar: float64Ptr(10)}}}
	result := SessionResult{Prompt: "alpha alpha", Measurements: Measurements{Words: []WordObservation{
		{Start: 0, End: 5, ActiveMS: 50, Attempts: 1, CorrectAttempts: 1, Complete: true},
		{Start: 6, End: 11, ActiveMS: 50, Attempts: 1, CorrectAttempts: 1, Complete: true},
	}}}
	comparisons := ComparePractice(plan, result)
	if len(comparisons) != 1 || comparisons[0].SpeedChangePercent != nil {
		t.Fatalf("sparse comparison = %#v", comparisons)
	}
	if comparisons[0].AccuracyDelta == nil || *comparisons[0].AccuracyDelta != -0 {
		t.Fatalf("accuracy comparison = %#v", comparisons[0])
	}
}

func float64Ptr(v float64) *float64 { return &v }

func historyPracticeFixture(id string, finished int64, revision string, item string, activeMS int64) HistoryRecord {
	return HistoryRecord{
		ID: id, FinishedUnixMS: finished, Mode: string(wordMode), SourceKind: "embedded-word",
		Privacy: "public", PackID: "1000en", PackRevision: revision, Outcome: "completed",
		Fragments: []HistoryFragment{{Item: item, Kind: "word", Occurrences: 1, Attempts: 1, CorrectAttempts: 1, ActiveMS: activeMS,
			Observations: []HistoryOccurrence{{ActiveMS: activeMS, Scalars: len(item), Attempts: 1, CorrectAttempts: 1}}}},
	}
}

func TestSelectPracticeMergesBoundedHistoryAndDeduplicatesCurrent(t *testing.T) {
	current := practiceResultFixture()
	current.FinishedUnixMS = 40 * 24 * 60 * 60 * 1000
	current.Prompt = "alpha"
	current.Measurements.Words = []WordObservation{{Start: 0, End: 5, ActiveMS: 100, Attempts: 2, CorrectAttempts: 1, Errors: 1, Complete: true}}
	history := []HistoryRecord{
		historyPracticeFixture("old", current.FinishedUnixMS-31*24*60*60*1000, "r", "alpha", 100),
		historyPracticeFixture("current", current.FinishedUnixMS-24*60*60*1000, "r", "alpha", 100),
		historyPracticeFixture("recent", current.FinishedUnixMS-24*60*60*1000, "r", "alpha", 100),
	}
	plan, err := SelectPractice(current, history)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Items) != 1 || plan.Items[0].HistoryOccurrences != 1 || plan.SampleSessions != 2 {
		t.Fatalf("bounded history = %#v", plan)
	}
}

func TestSelectPracticeKeepsOnePackRevision(t *testing.T) {
	current := practiceResultFixture()
	current.FinishedUnixMS = 10 * 24 * 60 * 60 * 1000
	current.Prompt = "alpha"
	current.Measurements.Words = []WordObservation{{Start: 0, End: 5, ActiveMS: 100, Attempts: 2, CorrectAttempts: 1, Errors: 1, Complete: true}}
	records := []HistoryRecord{
		historyPracticeFixture("new", current.FinishedUnixMS-1000, "new", "alpha", 200),
		historyPracticeFixture("old", current.FinishedUnixMS-2000, "old", "alpha", 900),
	}
	plan, err := SelectPractice(current, records)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Items[0].HistoryOccurrences != 1 || plan.Items[0].MedianMSPerScalar == nil || *plan.Items[0].MedianMSPerScalar != 30 {
		t.Fatalf("revision merge = %#v", plan.Items[0])
	}
}

func TestSelectPracticeUsesLegacyMistakeCountsWithoutInventingSpeed(t *testing.T) {
	current := practiceResultFixture()
	current.Prompt = "alpha"
	current.Measurements.Words = []WordObservation{{Start: 0, End: 5, Attempts: 2, CorrectAttempts: 1, Errors: 1}}
	legacy := historyPracticeFixture("legacy", current.FinishedUnixMS-1000, "r", "alpha", 400)
	legacy.Fragments[0] = HistoryFragment{Item: "alpha", Kind: "word", Occurrences: 1, Attempts: 3, CorrectAttempts: 1, Errors: 2, ActiveMS: 400}
	plan, err := SelectPractice(current, []HistoryRecord{legacy})
	if err != nil {
		t.Fatal(err)
	}
	item := plan.Items[0]
	if item.Errors != 3 || item.Attempts != 5 || item.HistoryOccurrences != 0 || item.MedianMSPerScalar != nil {
		t.Fatalf("legacy evidence invented complete timing: %#v", item)
	}
}

func TestSelectPracticeAcceptsCurrentWrongAttemptWithoutCompleteOccurrence(t *testing.T) {
	current := practiceResultFixture()
	current.Prompt = "alpha"
	current.Measurements.Words = []WordObservation{{Start: 0, End: 5, Attempts: 1, Errors: 1}}
	plan, err := SelectPractice(current, nil)
	if err != nil || len(plan.Items) != 1 || plan.Items[0].Occurrences != 0 || plan.Items[0].Reason != "error" {
		t.Fatalf("wrong attempt selection = %#v, %v", plan, err)
	}
}

func TestSelectPracticeRequiresSlowMedianAgainstSample(t *testing.T) {
	current := practiceResultFixture()
	current.Prompt = "slow slow slow fast fast fast"
	current.Measurements.Words = []WordObservation{
		{Start: 0, End: 4, ActiveMS: 300, Attempts: 1, CorrectAttempts: 1, Complete: true},
		{Start: 5, End: 9, ActiveMS: 300, Attempts: 1, CorrectAttempts: 1, Complete: true},
		{Start: 10, End: 14, ActiveMS: 300, Attempts: 1, CorrectAttempts: 1, Complete: true},
		{Start: 15, End: 19, ActiveMS: 100, Attempts: 1, CorrectAttempts: 1, Complete: true},
		{Start: 20, End: 24, ActiveMS: 100, Attempts: 1, CorrectAttempts: 1, Complete: true},
		{Start: 25, End: 29, ActiveMS: 100, Attempts: 1, CorrectAttempts: 1, Complete: true},
	}
	plan, err := SelectPractice(current, nil)
	if err != nil || len(plan.Items) != 1 || plan.Items[0].Item != "slow" || plan.Items[0].Reason != "slow" {
		t.Fatalf("slow selection = %#v, %v", plan, err)
	}
}

func TestBuildPracticeRejectsNeutralAndLongContext(t *testing.T) {
	plan := PracticePlan{Items: []PracticeItem{{Item: "alpha", Context: "alpha beta gamma delta epsilon zeta"}}}
	if _, err := BuildPractice(plan, []string{"neutral"}, 1); !errors.Is(err, ErrContextTooLong) {
		t.Fatalf("long context error = %v", err)
	}
	if _, err := BuildPractice(PracticePlan{Items: plan.Items[:0]}, []string{"neutral"}, 1); !errors.Is(err, ErrNoNeutralPack) {
		t.Fatalf("empty curriculum error = %v", err)
	}
	if _, err := BuildPractice(PracticePlan{Items: []PracticeItem{{Item: "alpha"}}}, nil, 1); !errors.Is(err, ErrNoNeutralPack) {
		t.Fatalf("empty neutral error = %v", err)
	}
}

func TestBuildPracticeSeedIsDeterministic(t *testing.T) {
	plan := PracticePlan{ParentID: "parent", Items: []PracticeItem{{Item: "alpha"}, {Item: "beta"}}}
	neutral := []string{"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine"}
	first, err := BuildPractice(plan, neutral, 77)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildPractice(plan, neutral, 77)
	if err != nil || first.Segments[0].Text != second.Segments[0].Text {
		t.Fatalf("seeded generation = %v, %v", first, second)
	}
}

func TestComparePracticeReportsDenseAccuracyAndSpeed(t *testing.T) {
	plan := PracticePlan{Items: []PracticeItem{{Item: "alpha", Occurrences: 3, Attempts: 3, CorrectAttempts: 1, MedianMSPerScalar: float64Ptr(100)}}}
	result := SessionResult{Prompt: "alpha alpha alpha", Measurements: Measurements{Words: []WordObservation{
		{Start: 0, End: 5, ActiveMS: 50, Attempts: 2, CorrectAttempts: 2, Complete: true},
		{Start: 6, End: 11, ActiveMS: 50, Attempts: 2, CorrectAttempts: 1, Errors: 1, Complete: true},
		{Start: 12, End: 17, ActiveMS: 50, Attempts: 2, CorrectAttempts: 1, Errors: 1, Complete: true},
	}}}
	comparison := ComparePractice(plan, result)[0]
	if comparison.SpeedChangePercent == nil || *comparison.SpeedChangePercent != 90 || comparison.AccuracyDelta == nil || *comparison.AccuracyDelta <= 0 {
		t.Fatalf("dense comparison = %#v", comparison)
	}
}
