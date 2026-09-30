package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func historyFixture(id string, finished int64) HistoryRecord {
	one := 60.0
	return HistoryRecord{
		ID: id, ContentID: strings.Repeat("b", 32), MetricVersion: metricVersion,
		FinishedUnixMS: finished, Mode: "words", SourceKind: "private-word",
		SourceLabel: "Private word list", Privacy: "private", DurationMS: -1,
		WordCount: 2, WordsPerGroup: 2, Groups: 1, Difficulty: "normal",
		Outcome: "completed", Measurements: HistoryMeasurements{
			WPM: &one, RawWPM: &one, CPM: &one, Accuracy: &one,
			ActiveMS: 1000, ActiveNS: 1_000_000_000,
			Characters: CharacterTotals{Correct: 2}, Attempts: 2, CorrectAttempts: 2,
			Series: []HistorySample{},
		},
		Fragments: []HistoryFragment{}, EligibilityReasons: []string{},
	}
}

func TestHistorySaveIsDurableIdempotentAndChronological(t *testing.T) {
	root := filepath.Join(t.TempDir(), "history-v1")
	older := historyFixture(strings.Repeat("1", 32), 10)
	newer := historyFixture(strings.Repeat("2", 32), 20)
	for _, record := range []HistoryRecord{older, newer, older} {
		if err := SaveHistory(root, record); err != nil {
			t.Fatal(err)
		}
	}
	page, err := ReadHistory(root, HistoryQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || len(page.Records) != 2 || page.Records[0].ID != newer.ID || page.Records[1].ID != older.ID {
		t.Fatalf("history page = %#v", page)
	}
	entries, err := os.ReadDir(filepath.Join(root, "sessions"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("session files = %d, want 2", len(entries))
	}
}

func TestHistoryProjectionDoesNotRetainPrivateTextOrPath(t *testing.T) {
	secret := "SENTINEL-private-prompt"
	result := SessionResult{
		ID: strings.Repeat("a", 32), PromptID: strings.Repeat("c", 32),
		MetricVersion: metricVersion, FinishedUnixMS: 10,
		Config:  TestConfig{Mode: customMode, Source: fileSource, Pack: "/private/" + secret, TimeLimit: -1, WordCount: 2, WordsPerGroup: 2, Groups: 1},
		Outcome: "completed", Measurements: Measurements{ActiveMS: 1, Words: []WordObservation{{Start: 0, End: len(secret), Errors: 1}}},
		Prompt: secret, ActiveNS: 1_500_000, EligibilityReasons: []string{},
	}
	record := ProjectHistory(result, ResourceOrigin{Kind: "file", Path: "/private/" + secret}, PrivacyPolicy{})
	root := filepath.Join(t.TempDir(), "history-v1")
	if err := SaveHistory(root, record); err != nil {
		t.Fatal(err)
	}
	if record.Privacy != "private" || len(record.Fragments) != 0 || record.SourceLabel != "File" {
		t.Fatalf("private projection = %#v", record)
	}
	if record.ContentID == result.PromptID {
		t.Fatal("private content ID reused the prompt ID")
	}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(data), secret) || strings.Contains(string(data), "/private/") {
			t.Fatalf("private source leaked through %s: %s", path, data)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestHistoryProjectionRetainsBoundedPublicFragments(t *testing.T) {
	prompt := "alpha beta gamma"
	result := SessionResult{
		ID: strings.Repeat("d", 32), PromptID: strings.Repeat("e", 32), MetricVersion: metricVersion,
		FinishedUnixMS: 10, Config: TestConfig{Mode: wordMode, Source: wordSource, Pack: "1000en", TimeLimit: -1, WordCount: 3, WordsPerGroup: 3, Groups: 1},
		Outcome: "completed", Prompt: prompt, Measurements: Measurements{
			Words: []WordObservation{{Start: 0, End: 5, Errors: 2, Attempts: 3, CorrectAttempts: 1, ActiveMS: 100, Complete: true}, {Start: 6, End: 10}, {Start: 11, End: 16}},
		}, EligibilityReasons: []string{},
	}
	record := ProjectHistory(result, ResourceOrigin{Kind: "embedded-word", PackID: "1000en", Revision: strings.Repeat("f", 64), Embedded: true}, PrivacyPolicy{})
	if record.Privacy != "public" || record.PackID != "1000en" || len(record.ContentID) != 64 || len(record.Fragments) != 1 || record.Fragments[0].Item != "alpha" {
		t.Fatalf("public projection = %#v", record)
	}
	result.Attribution = "Public catalog"
	record = ProjectHistory(result, ResourceOrigin{Kind: "embedded-word", PackID: "1000en", Revision: strings.Repeat("f", 64), Embedded: true}, PrivacyPolicy{})
	if record.Attribution.Source != "Public catalog" {
		t.Fatalf("public attribution = %#v", record.Attribution)
	}

	result.Prompt = "alpha"
	result.Measurements.Words = []WordObservation{{Start: 0, End: 5, Errors: 1, ActiveMS: 20, Complete: true}}
	if short := ProjectHistory(result, ResourceOrigin{Kind: "embedded-word", PackID: "1000en", Revision: "rev", Embedded: true}, PrivacyPolicy{}); len(short.Fragments) != 0 {
		t.Fatalf("complete short prompt retained: %#v", short.Fragments)
	}
}

func TestHistoryProjectionRetainsCompleteCorrectAndErrorObservations(t *testing.T) {
	prompt := "alpha beta gamma delta"
	result := SessionResult{
		ID: strings.Repeat("1", 32), PromptID: strings.Repeat("2", 32), MetricVersion: metricVersion,
		FinishedUnixMS: 10, Config: TestConfig{Mode: wordMode, Source: wordSource, Pack: "1000en", TimeLimit: -1, WordCount: 4, WordsPerGroup: 4, Groups: 1},
		Outcome: "completed", Prompt: prompt, Measurements: Measurements{Words: []WordObservation{
			{Start: 0, End: 5, ActiveMS: 120, Attempts: 2, CorrectAttempts: 1, Errors: 1, Complete: true},
			{Start: 6, End: 10, ActiveMS: 900, Attempts: 1, CorrectAttempts: 1, Complete: true},
			{Start: 11, End: 16, ActiveMS: 0, Complete: false},
		}}, EligibilityReasons: []string{},
	}
	record := ProjectHistory(result, ResourceOrigin{Kind: "embedded-word", PackID: "1000en", Revision: strings.Repeat("3", 64), Embedded: true}, PrivacyPolicy{})
	if len(record.Fragments) != 2 {
		t.Fatalf("fragments = %#v", record.Fragments)
	}
	var alpha, beta HistoryFragment
	for _, fragment := range record.Fragments {
		switch fragment.Item {
		case "alpha":
			alpha = fragment
		case "beta":
			beta = fragment
		}
	}
	if alpha.Occurrences != 1 || len(alpha.Observations) != 1 || alpha.Observations[0].Scalars != 5 ||
		alpha.Observations[0].Errors != 1 {
		t.Fatalf("error observation = %#v", alpha)
	}
	if beta.Occurrences != 1 || len(beta.Observations) != 1 || beta.Errors != 0 ||
		beta.Observations[0].ActiveMS != 900 {
		t.Fatalf("correct slow observation = %#v", beta)
	}
}

func TestHistoryProjectionBoundsQuoteContext(t *testing.T) {
	prompt := "one, two three four five six"
	start := strings.Index(prompt, "three")
	result := SessionResult{
		ID: strings.Repeat("4", 32), PromptID: strings.Repeat("5", 32), MetricVersion: metricVersion,
		FinishedUnixMS: 10, Config: TestConfig{Mode: quoteMode, Source: quoteSource, Pack: "quotes", TimeLimit: -1},
		Outcome: "completed", Prompt: prompt, Measurements: Measurements{Words: []WordObservation{{
			Start: start, End: start + len("three"), ActiveMS: 250, Attempts: 1, CorrectAttempts: 1, Complete: true,
		}}}, EligibilityReasons: []string{},
	}
	record := ProjectHistory(result, ResourceOrigin{Kind: "embedded-quote", PackID: "quotes", Revision: strings.Repeat("6", 64), Embedded: true}, PrivacyPolicy{})
	if len(record.Fragments) != 1 {
		t.Fatalf("fragments = %#v", record.Fragments)
	}
	context := record.Fragments[0].Context
	if context == "" || len(strings.Fields(context)) > 5 || !strings.Contains(context, "three") || !strings.Contains(context, "one,") {
		t.Fatalf("bounded quote context = %q", context)
	}
}

func TestHistoryProjectionDoesNotRetainWholeShortQuoteContext(t *testing.T) {
	prompt := "one two three four five"
	result := SessionResult{
		ID: strings.Repeat("0", 32), PromptID: strings.Repeat("1", 32), MetricVersion: metricVersion,
		FinishedUnixMS: 10, Config: TestConfig{Mode: quoteMode, Source: quoteSource, Pack: "quotes", TimeLimit: -1},
		Outcome: "completed", Prompt: prompt, Measurements: Measurements{Words: []WordObservation{{
			Start: 8, End: 13, ActiveMS: 100, Attempts: 1, CorrectAttempts: 1, Complete: true,
		}}}, EligibilityReasons: []string{},
	}
	record := ProjectHistory(result, ResourceOrigin{Kind: "embedded-quote", PackID: "quotes", Revision: strings.Repeat("2", 64), Embedded: true}, PrivacyPolicy{})
	if len(record.Fragments) != 0 {
		t.Fatalf("short quote context retained whole prompt: %#v", record.Fragments)
	}
}

func TestHistoryPracticeDetailPersistsOnlyInPracticeQuery(t *testing.T) {
	root := filepath.Join(t.TempDir(), "history-v1")
	record := historyFixture(strings.Repeat("7", 32), 10)
	record.Mode = "practice"
	record.SourceKind = "embedded-word"
	record.SourceLabel = "1000en"
	record.PackID = "1000en"
	record.PackRevision = strings.Repeat("8", 64)
	record.Privacy = "public"
	record.Practice = true
	median := 12.5
	record.PracticeDetail = &PracticeDetail{
		ParentID: strings.Repeat("9", 32), WindowStartUnixMS: 1, SampleSessions: 2,
		Items:       []PracticeItem{{Item: "alpha", Context: "", Occurrences: 3, Attempts: 3, CorrectAttempts: 2, Errors: 1, MedianMSPerScalar: &median, CurrentOccurrences: 1, HistoryOccurrences: 2}},
		Comparisons: []PracticeComparison{{Item: "alpha", Baseline: PracticeItem{Item: "alpha"}, Drill: PracticeItem{Item: "alpha"}}},
	}
	if err := SaveHistory(root, record); err != nil {
		t.Fatal(err)
	}
	regular, err := ReadHistory(root, HistoryQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if regular.Total != 0 {
		t.Fatalf("regular history included practice: %#v", regular)
	}
	practice, err := ReadHistory(root, HistoryQuery{Practice: true})
	if err != nil || practice.Total != 1 || practice.Records[0].PracticeDetail == nil ||
		practice.Records[0].PracticeDetail.Items[0].Item != "alpha" {
		t.Fatalf("practice history = %#v, %v", practice, err)
	}
	index, err := os.ReadFile(filepath.Join(root, "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(index), "practiceDetail") {
		t.Fatalf("index retained practice detail: %s", index)
	}
	sessionData, err := os.ReadFile(filepath.Join(root, "sessions", record.ID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(sessionData), `"practiceDetail"`) ||
		!strings.Contains(string(sessionData), `"parentId"`) ||
		strings.Contains(string(sessionData), `"ParentID"`) {
		t.Fatalf("practice detail JSON keys = %s", sessionData)
	}
}

func TestHistoryRejectsInvalidPracticeNestedObservations(t *testing.T) {
	record := historyFixture(strings.Repeat("a", 32), 10)
	record.SourceKind = "embedded-word"
	record.SourceLabel = "1000en"
	record.PackID = "1000en"
	record.PackRevision = strings.Repeat("b", 64)
	record.Privacy = "public"
	record.Fragments = []HistoryFragment{{
		Item: "alpha", Kind: "word", Occurrences: 1, Attempts: 1, CorrectAttempts: 1,
		Observations: []HistoryOccurrence{{ActiveMS: -1, Scalars: 5, Attempts: 1, CorrectAttempts: 1}},
	}}
	if err := SaveHistory(filepath.Join(t.TempDir(), "history-v1"), record); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("invalid nested observation write = %v", err)
	}
}

func TestHistoryRejectsPrivatePracticeDetail(t *testing.T) {
	record := historyFixture(strings.Repeat("f", 32), 10)
	record.Practice = true
	record.Mode = "practice"
	record.PracticeDetail = &PracticeDetail{
		ParentID: strings.Repeat("1", 32), Items: []PracticeItem{{Item: "secret"}},
		Comparisons: []PracticeComparison{},
	}
	if err := SaveHistory(filepath.Join(t.TempDir(), "history-v1"), record); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("private practice detail write = %v", err)
	}
}

func TestHistoryProjectionCapsOccurrencesPerItem(t *testing.T) {
	words := make([]string, 0, 56)
	observations := make([]WordObservation, 0, 55)
	offset := 0
	for i := range 55 {
		words = append(words, "alpha")
		observations = append(observations, WordObservation{
			Start: offset, End: offset + len("alpha"), ActiveMS: int64(i + 1),
			Attempts: 1, CorrectAttempts: 1, Complete: true,
		})
		offset += len("alpha") + 1
	}
	words = append(words, "omega")
	result := SessionResult{
		ID: strings.Repeat("c", 32), PromptID: strings.Repeat("d", 32), MetricVersion: metricVersion,
		FinishedUnixMS: 10, Config: TestConfig{Mode: wordMode, Source: wordSource, Pack: "1000en", TimeLimit: -1},
		Outcome: "completed", Prompt: strings.Join(words, " "), Measurements: Measurements{Words: observations},
		EligibilityReasons: []string{},
	}
	record := ProjectHistory(result, ResourceOrigin{Kind: "embedded-word", PackID: "1000en", Revision: strings.Repeat("e", 64), Embedded: true}, PrivacyPolicy{})
	if len(record.Fragments) != 1 || len(record.Fragments[0].Observations) != 50 ||
		record.Fragments[0].Observations[0].ActiveMS != 6 || record.Fragments[0].Observations[49].ActiveMS != 55 {
		t.Fatalf("bounded observations = %#v", record.Fragments)
	}
}

func TestHistoryRejectsConflictBusyAndInvalidQueries(t *testing.T) {
	root := filepath.Join(t.TempDir(), "history-v1")
	record := historyFixture(strings.Repeat("3", 32), 10)
	if err := SaveHistory(root, record); err != nil {
		t.Fatal(err)
	}
	changed := record
	changed.FinishedUnixMS++
	if err := SaveHistory(root, changed); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflict error = %v", err)
	}
	if err := os.Mkdir(filepath.Join(root, ".write-lock"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadHistory(root, HistoryQuery{}); !errors.Is(err, ErrBusy) {
		t.Fatalf("busy error = %v", err)
	}
	if _, err := ReadHistory(filepath.Join(t.TempDir(), "history-v1"), HistoryQuery{Limit: 101}); err == nil {
		t.Fatal("invalid query succeeded")
	}
}

func TestHistoryCorruptionPreservesBytesAndNamesAffectedPath(t *testing.T) {
	for _, fixture := range []struct {
		name string
		data string
	}{
		{"malformed", "{"},
		{"unknown-record-version", `{"schemaVersion":2,"session":{}}`},
		{"missing-required-field", `{"schemaVersion":1,"session":{}}`},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "history-v1")
			record := historyFixture(strings.Repeat("4", 32), 10)
			if err := SaveHistory(root, record); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "sessions", strings.Repeat("5", 32)+".json")
			if err := os.WriteFile(path, []byte(fixture.data), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := ReadHistory(root, HistoryQuery{})
			if !errors.Is(err, ErrUnavailable) || !strings.Contains(err.Error(), path) {
				t.Fatalf("unavailable error = %v", err)
			}
			got, readErr := os.ReadFile(path)
			if readErr != nil || string(got) != fixture.data {
				t.Fatalf("corrupt bytes changed: %q, %v", got, readErr)
			}
			if err := SaveHistory(root, historyFixture(strings.Repeat("6", 32), 20)); !errors.Is(err, ErrUnavailable) {
				t.Fatalf("write to corrupt store = %v", err)
			}
		})
	}
}

func TestHistoryRebuildsMissingAndStaleIndex(t *testing.T) {
	for _, mutate := range []struct {
		name string
		fn   func(string) error
	}{
		{"missing", func(path string) error { return os.Remove(path) }},
		{"malformed", func(path string) error { return os.WriteFile(path, []byte("{"), 0600) }},
		{"stale", func(path string) error {
			return os.WriteFile(path, []byte(`{"schemaVersion":1,"generation":"old","entries":[]}`), 0600)
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "history-v1")
			if err := SaveHistory(root, historyFixture(strings.Repeat("7", 32), 10)); err != nil {
				t.Fatal(err)
			}
			if err := mutate.fn(filepath.Join(root, "index.json")); err != nil {
				t.Fatal(err)
			}
			page, err := ReadHistory(root, HistoryQuery{})
			if err != nil || page.Total != 1 || page.Generation == "" || !page.Rebuilt {
				t.Fatalf("rebuilt page = %#v, %v", page, err)
			}
		})
	}
}

func TestHistoryUsesPrivatePOSIXPermissions(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("Windows inherits user-profile ACLs")
	}
	root := filepath.Join(t.TempDir(), "history-v1")
	record := historyFixture(strings.Repeat("8", 32), 10)
	if err := SaveHistory(root, record); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{root, filepath.Join(root, "sessions")} {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0700 {
			t.Fatalf("directory %s mode = %v, %v", path, info.Mode().Perm(), err)
		}
	}
	for _, path := range []string{filepath.Join(root, "store.json"), filepath.Join(root, "index.json"), filepath.Join(root, "sessions", record.ID+".json")} {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("file %s mode = %v, %v", path, info.Mode().Perm(), err)
		}
	}
}

func TestHistoryAtomicFailuresPreserveCommittedRecords(t *testing.T) {
	for _, operation := range []string{"create", "write", "sync", "rename", "dir-sync"} {
		t.Run(operation, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "history-v1")
			committed := historyFixture(strings.Repeat("9", 32), 10)
			if err := SaveHistory(root, committed); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "sessions", committed.ID+".json")
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			targetID := strings.Repeat("a", 31) + "b"
			historyFault = func(got, path string) error {
				if got == operation && strings.HasSuffix(path, targetID+".json") {
					return errors.New("injected " + operation)
				}
				return nil
			}
			err = SaveHistory(root, historyFixture(targetID, 20))
			historyFault = nil
			if err == nil {
				t.Fatalf("%s failure was ignored", operation)
			}
			after, readErr := os.ReadFile(path)
			if readErr != nil || string(after) != string(before) {
				t.Fatalf("committed record changed after %s: %v", operation, readErr)
			}
			if err := SaveHistory(root, historyFixture(targetID, 20)); err != nil {
				t.Fatalf("retry after %s: %v", operation, err)
			}
			page, err := ReadHistory(root, HistoryQuery{})
			if err != nil || page.Total != 2 {
				t.Fatalf("history after retry = %#v, %v", page, err)
			}
		})
	}
}

func TestHistoryLockSerializesReadersAndWriters(t *testing.T) {
	root := filepath.Join(t.TempDir(), "history-v1")
	if err := SaveHistory(root, historyFixture(strings.Repeat("b", 32), 10)); err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(root, ".write-lock")
	if err := os.Mkdir(lock, 0700); err != nil {
		t.Fatal(err)
	}
	if err := SaveHistory(root, historyFixture(strings.Repeat("c", 32), 20)); !errors.Is(err, ErrBusy) {
		t.Fatalf("competing writer error = %v", err)
	}
	if _, err := ReadHistory(root, HistoryQuery{}); !errors.Is(err, ErrBusy) {
		t.Fatalf("competing reader error = %v", err)
	}
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}
	if err := SaveHistory(root, historyFixture(strings.Repeat("c", 32), 20)); err != nil {
		t.Fatalf("writer after release: %v", err)
	}
}

func TestHistoryUnknownStoreVersionIsUnavailable(t *testing.T) {
	root := filepath.Join(t.TempDir(), "history-v1")
	if err := SaveHistory(root, historyFixture(strings.Repeat("d", 32), 10)); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "store.json")
	data := []byte("{\"schemaVersion\":2}\n")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadHistory(root, HistoryQuery{}); !errors.Is(err, ErrUnavailable) || !strings.Contains(err.Error(), path) {
		t.Fatalf("unknown store error = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(data) {
		t.Fatalf("store bytes changed: %q, %v", got, err)
	}
}

func TestHistoryPaginatesAndSeparatesPractice(t *testing.T) {
	root := filepath.Join(t.TempDir(), "history-v1")
	for i := 0; i < 27; i++ {
		record := historyFixture(fmt.Sprintf("%032x", i+1), int64(i))
		if err := SaveHistory(root, record); err != nil {
			t.Fatal(err)
		}
	}
	practice := historyFixture(strings.Repeat("e", 32), 100)
	practice.Practice = true
	if err := SaveHistory(root, practice); err != nil {
		t.Fatal(err)
	}
	first, err := ReadHistory(root, HistoryQuery{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := ReadHistory(root, HistoryQuery{Offset: 25, Limit: 25})
	if err != nil {
		t.Fatal(err)
	}
	practicePage, err := ReadHistory(root, HistoryQuery{Practice: true})
	if err != nil {
		t.Fatal(err)
	}
	if first.Total != 27 || len(first.Records) != 25 || len(second.Records) != 2 ||
		first.Records[0].FinishedUnixMS != 26 || practicePage.Total != 1 || practicePage.Records[0].ID != practice.ID {
		t.Fatalf("pages = first %#v, second %#v, practice %#v", first, second, practicePage)
	}
}

func TestHistoryPracticeWindowPagesMatchingPackBeforeLimit(t *testing.T) {
	root := filepath.Join(t.TempDir(), "history-v1")
	match := historyFixture(strings.Repeat("a", 32), 1)
	match.Privacy, match.SourceKind, match.SourceLabel = "public", "embedded-word", "1000en"
	match.PackID, match.PackRevision = "1000en", strings.Repeat("b", 64)
	if err := SaveHistory(root, match); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 21; i++ {
		other := historyFixture(fmt.Sprintf("%032x", i+1), int64(i+2))
		if err := SaveHistory(root, other); err != nil {
			t.Fatal(err)
		}
	}
	page, err := ReadHistory(root, HistoryQuery{Limit: 20, PackID: "1000en", PackRevision: strings.Repeat("b", 64), Mode: "words"})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Records) != 1 || page.Records[0].ID != match.ID {
		t.Fatalf("matching history after unrelated results: %#v", page)
	}
}

func TestHistoryReadsExistingPracticeRetryWithPublicMistakeFragment(t *testing.T) {
	root := filepath.Join(t.TempDir(), "history-v1")
	retry := historyFixture(strings.Repeat("c", 32), 1)
	retry.Privacy, retry.SourceKind, retry.SourceLabel = "public", "embedded-word", "1000en"
	retry.PackID, retry.PackRevision = "1000en", strings.Repeat("d", 64)
	retry.Practice = true
	retry.Fragments = []HistoryFragment{{Item: "mistyped", Kind: "word", Occurrences: 1, Attempts: 2, CorrectAttempts: 1, Errors: 1}}
	if err := SaveHistory(root, retry); err != nil {
		t.Fatal(err)
	}
	if err := SaveHistory(root, historyFixture(strings.Repeat("e", 32), 2)); err != nil {
		t.Fatalf("legacy retry blocked subsequent saves: %v", err)
	}
	page, err := ReadHistory(root, HistoryQuery{Practice: true})
	if err != nil || page.Total != 1 || page.Records[0].Fragments[0].Item != "mistyped" {
		t.Fatalf("legacy practice history = %#v, %v", page, err)
	}
}

func TestHistoryRemovesInterruptedTemporaryWrites(t *testing.T) {
	root := filepath.Join(t.TempDir(), "history-v1")
	if err := SaveHistory(root, historyFixture(strings.Repeat("f", 32), 10)); err != nil {
		t.Fatal(err)
	}
	orphan := filepath.Join(root, "sessions", ".history-interrupted")
	if err := os.WriteFile(orphan, []byte("partial secret"), 0600); err != nil {
		t.Fatal(err)
	}
	page, err := ReadHistory(root, HistoryQuery{})
	if err != nil || page.Total != 1 {
		t.Fatalf("history after interrupted write = %#v, %v", page, err)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatalf("interrupted temp remains: %v", err)
	}
}

func TestHistoryRejectsPrivateFragments(t *testing.T) {
	record := historyFixture(strings.Repeat("a", 32), 10)
	record.Fragments = []HistoryFragment{{Item: "secret", Kind: "word", Occurrences: 1}}
	if err := SaveHistory(filepath.Join(t.TempDir(), "history-v1"), record); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("private fragment write = %v", err)
	}
}
