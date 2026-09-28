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
			Words: []WordObservation{{Start: 0, End: 5, Errors: 2, Attempts: 3, CorrectAttempts: 1}, {Start: 6, End: 10}, {Start: 11, End: 16}},
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
	result.Measurements.Words = []WordObservation{{Start: 0, End: 5, Errors: 1}}
	if short := ProjectHistory(result, ResourceOrigin{Kind: "embedded-word", PackID: "1000en", Revision: "rev", Embedded: true}, PrivacyPolicy{}); len(short.Fragments) != 0 {
		t.Fatalf("complete short prompt retained: %#v", short.Fragments)
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
