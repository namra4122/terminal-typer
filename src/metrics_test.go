package main

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestMeasureUsesUnicodeScalarsAndActiveNanoseconds(t *testing.T) {
	s := NewSession(&Test{Segments: []segment{{Text: "é!"}}}, "attempt", "prompt")
	s.State = SessionCompleted
	s.Cursor = len(s.promptRunes)
	s.Typed = append([]rune(nil), s.promptRunes...)
	s.ActiveNS = 60e9
	s.Events = []InputEvent{
		{Seq: 1, ActiveNS: 0, Kind: InputText, Text: "é", CursorBefore: 0, CursorAfter: 1},
		{Seq: 2, ActiveNS: 1, Kind: InputText, Text: "!", CursorBefore: 1, CursorAfter: 2},
	}
	got, err := Measure(*s)
	if err != nil {
		t.Fatal(err)
	}
	if got.WPM == nil || *got.WPM != 0.4 || got.CPM == nil || *got.CPM != 2 || got.RawWPM == nil || *got.RawWPM != 0.4 {
		t.Fatalf("unexpected rates: %#v", got)
	}
	if got.Attempts != 2 || got.Characters.Correct != 2 {
		t.Fatalf("unexpected totals: %#v", got)
	}
}

func TestMeasureZeroDurationIsUnavailableAndJSONNull(t *testing.T) {
	s := NewSession(&Test{Segments: []segment{{Text: "a"}}}, "attempt", "prompt")
	got, err := Measure(*s)
	if err != nil {
		t.Fatal(err)
	}
	if got.WPM != nil || got.RawWPM != nil || got.CPM != nil {
		t.Fatalf("zero-duration rates must be unavailable: %#v", got)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var values map[string]any
	if err := json.Unmarshal(encoded, &values); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"wpm", "raw_wpm", "cpm"} {
		if values[field] != nil {
			t.Fatalf("%s = %v; want null", field, values[field])
		}
	}
}

func TestMeasureNeverProducesNonFiniteSpeed(t *testing.T) {
	s := NewSession(&Test{Segments: []segment{{Text: "a"}}}, "attempt", "prompt")
	s.State = SessionCompleted
	s.Cursor = 1
	s.Typed[0] = 'a'
	s.ActiveNS = 1e6
	got, err := Measure(*s)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []*float64{got.WPM, got.RawWPM, got.CPM} {
		if v == nil || math.IsInf(*v, 0) || math.IsNaN(*v) {
			t.Fatalf("invalid short-sample rate: %v", v)
		}
	}
}

func TestMeasureKeepsCorrectedErrorsInAttemptTotals(t *testing.T) {
	s := NewSession(&Test{Segments: []segment{{Text: "cat"}}}, "attempt", "prompt")
	for _, in := range []SessionInput{
		{Kind: InputText, Text: "c", AtNS: 1},
		{Kind: InputText, Text: "x", AtNS: 2},
		{Kind: InputBackspace, AtNS: 3},
		{Kind: InputText, Text: "a", AtNS: 4},
		{Kind: InputText, Text: "t", AtNS: 5},
	} {
		if err := s.Apply(in); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Measure(*s)
	if err != nil {
		t.Fatal(err)
	}
	if got.Attempts != 4 || got.CorrectAttempts != 3 || got.Characters.Correct != 3 ||
		got.Errors.Total != 1 || got.Errors.Corrected != 1 || got.Errors.Uncorrected != 0 {
		t.Fatalf("corrected error was lost or miscounted: %#v", got)
	}
	if len(got.Words) != 1 || got.Words[0].Start != 0 || got.Words[0].End != 3 ||
		got.Words[0].Attempts != 4 || got.Words[0].CorrectAttempts != 3 ||
		got.Words[0].Errors != 1 || !got.Words[0].Complete {
		t.Fatalf("word observation mismatch: %#v", got.Words)
	}
}

func TestMeasureBuildsIntervalSeriesAndConsistency(t *testing.T) {
	s := NewSession(&Test{Segments: []segment{{Text: "ab"}}}, "attempt", "prompt")
	s.State = SessionCompleted
	s.Cursor = 2
	s.Typed = []rune("ab")
	s.ActiveNS = 2e9
	s.Events = []InputEvent{
		{Seq: 1, ActiveNS: 100e6, Kind: InputText, Text: "a", CursorBefore: 0, CursorAfter: 1},
		{Seq: 2, ActiveNS: 1100e6, Kind: InputText, Text: "b", CursorBefore: 1, CursorAfter: 2},
	}
	got, err := Measure(*s)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Series) != 2 || got.Series[0].WindowMS != 1000 || got.Series[1].WindowMS != 1000 {
		t.Fatalf("unexpected interval windows: %#v", got.Series)
	}
	if got.Consistency == nil || *got.Consistency != 100 {
		t.Fatalf("constant interval rates should have 100%% consistency: %v", got.Consistency)
	}
}

func TestMeasureIncludesFinalPartialWindowOnlyAt250Milliseconds(t *testing.T) {
	s := NewSession(&Test{Segments: []segment{{Text: "a"}}}, "attempt", "prompt")
	s.State = SessionExpired
	s.ActiveNS = 1249e6
	got, err := Measure(*s)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Series) != 1 {
		t.Fatalf("sub-250ms tail added a sample: %#v", got.Series)
	}
	s.ActiveNS = 1250e6
	got, err = Measure(*s)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Series) != 2 || got.Series[1].WindowMS != 250 || got.Series[1].EndMS != 1250 {
		t.Fatalf("250ms tail was not included exactly: %#v", got.Series)
	}
}

func TestFinishResultCapturesRetryPreferencesAndPBExclusion(t *testing.T) {
	s := NewSession(&Test{Config: TestConfig{Mode: wordMode}, EligibleForPB: true}, "retry", "prompt")
	s.State = SessionExpired
	s.RetryOf = "previous"
	s.AllowBackspace = true
	result, err := FinishResult(*s, 1234)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Practice || !result.AllowBackspace || result.SkipWord || result.FinishedUnixMS != 1234 {
		t.Fatalf("result omitted attempt policy or retry exclusion: %#v", result)
	}
	if len(result.EligibilityReasons) != 1 || result.EligibilityReasons[0] != "retry" {
		t.Fatalf("unexpected eligibility reasons: %#v", result.EligibilityReasons)
	}
}

func TestMeasureUsesIncrementalLiveCountersAfterCorrection(t *testing.T) {
	s := NewSession(&Test{Segments: []segment{{Text: "cat"}}}, "attempt", "prompt")
	for _, input := range []SessionInput{
		{Kind: InputText, Text: "x", AtNS: 1},
		{Kind: InputBackspace, AtNS: 2},
		{Kind: InputText, Text: "c", AtNS: 3},
	} {
		if err := s.Apply(input); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Measure(*s)
	if err != nil {
		t.Fatal(err)
	}
	if got.Characters.Correct != 1 || got.Attempts != 2 || got.CorrectAttempts != 1 ||
		got.Errors.Total != 1 || got.Errors.Corrected != 1 || got.Errors.Uncorrected != 0 {
		t.Fatalf("live metrics lost correction history: %#v", got)
	}
}

func TestWordBoundaryOverflowIsExtraAndBackspaceRemovesItFirst(t *testing.T) {
	s := NewSession(&Test{Segments: []segment{{Text: "cat dog"}}}, "attempt", "prompt")
	at := int64(1)
	typeKey := func(text string) {
		if err := s.Apply(SessionInput{Kind: InputText, Text: text, AtNS: at}); err != nil {
			t.Fatal(err)
		}
		at++
	}
	for _, text := range []string{"c", "a", "t", "x"} {
		typeKey(text)
	}
	if s.Cursor != 3 || len(s.ExtraByWord[3]) != 1 || s.ExtraByWord[3][0] != 'x' {
		t.Fatalf("word overflow advanced canonical cursor or was lost: cursor=%d extras=%#v", s.Cursor, s.ExtraByWord)
	}
	live, err := Measure(*s)
	if err != nil {
		t.Fatal(err)
	}
	if live.Characters.Extra != 1 || live.Errors.Total != 1 || live.Attempts != 4 {
		t.Fatalf("live overflow metrics incorrect: %#v", live)
	}
	if err := s.Apply(SessionInput{Kind: InputBackspace, AtNS: at}); err != nil {
		t.Fatal(err)
	}
	at++
	if s.Cursor != 3 || len(s.ExtraByWord) != 0 {
		t.Fatalf("Backspace did not remove overflow before canonical input: cursor=%d extras=%#v", s.Cursor, s.ExtraByWord)
	}
	for _, text := range []string{" ", "d", "o", "g"} {
		typeKey(text)
	}
	got, err := Measure(*s)
	if err != nil {
		t.Fatal(err)
	}
	if got.Characters.Extra != 0 || got.Errors.Total != 1 || got.Errors.Corrected != 1 {
		t.Fatalf("removed overflow was not retained in error history: %#v", got)
	}
}

func TestWordObservationIncludesDelimiterActiveTime(t *testing.T) {
	s := NewSession(&Test{Segments: []segment{{Text: "ab cd"}}}, "attempt", "prompt")
	for _, input := range []SessionInput{
		{Kind: InputText, Text: "a", AtNS: 1},
		{Kind: InputText, Text: "b", AtNS: 100_000_001},
		{Kind: InputText, Text: " ", AtNS: 900_000_001},
	} {
		if err := s.Apply(input); err != nil {
			t.Fatal(err)
		}
	}
	s.State = SessionExpired
	got, err := Measure(*s)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Words) != 2 || got.Words[0].ActiveMS != 900 || !got.Words[0].Complete {
		t.Fatalf("word timing ended before delimiter: %#v", got.Words)
	}
}

func TestFinishedMetricsRetainWordBoundaryOverflow(t *testing.T) {
	s := NewSession(&Test{Segments: []segment{{Text: "cat dog"}}}, "attempt", "prompt")
	for i, text := range []string{"c", "a", "t", "x", " ", "d", "o", "g"} {
		if err := s.Apply(SessionInput{Kind: InputText, Text: text, AtNS: int64(i + 1)}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Measure(*s)
	if err != nil {
		t.Fatal(err)
	}
	if got.Characters.Extra != 1 || got.Characters.Correct != 7 ||
		got.Attempts != 8 || got.Errors.Total != 1 || got.Errors.Uncorrected != 1 {
		t.Fatalf("finished overflow metrics mismatch: %#v", got)
	}
}

func TestMeasureMatchesSixtySecondSpeedFixture(t *testing.T) {
	prompt := strings.Repeat("a", 300)
	s := NewSession(&Test{Segments: []segment{{Text: prompt}}}, "attempt", "prompt")
	for i := 0; i < 300; i++ {
		character := "a"
		if i >= 250 {
			character = "b"
		}
		if err := s.Apply(SessionInput{Kind: InputText, Text: character, AtNS: int64(i+1) * 200e6}); err != nil {
			t.Fatal(err)
		}
	}
	s.ActiveNS = 60e9
	got, err := Measure(*s)
	if err != nil {
		t.Fatal(err)
	}
	if got.WPM == nil || *got.WPM != 50 || got.RawWPM == nil || *got.RawWPM != 60 || got.CPM == nil || *got.CPM != 250 {
		t.Fatalf("60-second golden speeds mismatch: %#v", got)
	}
}

func TestFinishResultSerializesDeterministicallyForFixedSession(t *testing.T) {
	s := NewSession(&Test{Segments: []segment{{Text: "a b"}}, Config: TestConfig{Mode: wordMode, WordCount: 2}}, "attempt", "prompt")
	for i, text := range []string{"a", " "} {
		if err := s.Apply(SessionInput{Kind: InputText, Text: text, AtNS: int64(i + 1)}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Apply(SessionInput{Kind: InputSkip, AtNS: 3}); err != nil {
		t.Fatal(err)
	}
	first, err := FinishResult(*s, 12345)
	if err != nil {
		t.Fatal(err)
	}
	second, err := FinishResult(s.Snapshot(), 12345)
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := json.Marshal(second)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("same session replay changed result JSON:\n%s\n%s", firstJSON, secondJSON)
	}
}
