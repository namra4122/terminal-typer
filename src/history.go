package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const historySchemaVersion = 1

var (
	ErrUnavailable = errors.New("history unavailable")
	ErrBusy        = errors.New("history busy")
	ErrConflict    = errors.New("history record conflict")
)

var historyFault func(operation, path string) error

func injectHistoryFault(operation, path string) error {
	if historyFault == nil {
		return nil
	}
	return historyFault(operation, path)
}

type SaveState string

const (
	SavePending SaveState = "pending"
	SaveSaved   SaveState = "saved"
	SaveFailed  SaveState = "failed"
)

type HistoryQuery struct {
	Practice      bool
	Offset, Limit int
}

type HistoryPage struct {
	Records    []HistoryRecord
	Total      int
	Generation string
	Rebuilt    bool
}

type PrivacyPolicy struct{}

type HistoryModifiers struct {
	Punctuation    bool `json:"punctuation"`
	Numbers        bool `json:"numbers"`
	Capitalization bool `json:"capitalization"`
}

type HistorySample struct {
	EndMS    int64    `json:"endMS"`
	WindowMS int64    `json:"windowMS"`
	WPM      *float64 `json:"wpm"`
	RawWPM   *float64 `json:"rawWpm"`
	Errors   int      `json:"errors"`
}

type HistoryMeasurements struct {
	WPM             *float64        `json:"wpm"`
	RawWPM          *float64        `json:"rawWpm"`
	CPM             *float64        `json:"cpm"`
	Accuracy        *float64        `json:"accuracy"`
	Consistency     *float64        `json:"consistency"`
	ActiveMS        int64           `json:"activeMS"`
	PauseMS         int64           `json:"pauseMS"`
	ActiveNS        int64           `json:"activeNS"`
	PauseNS         int64           `json:"pauseNS"`
	Characters      CharacterTotals `json:"characters"`
	Errors          ErrorTotals     `json:"errors"`
	Attempts        int             `json:"attempts"`
	CorrectAttempts int             `json:"correctAttempts"`
	Series          []HistorySample `json:"series"`
}

type HistoryFragment struct {
	Item            string `json:"item"`
	Kind            string `json:"kind"`
	Occurrences     int    `json:"occurrences"`
	Attempts        int    `json:"attempts"`
	CorrectAttempts int    `json:"correctAttempts"`
	Errors          int    `json:"errors"`
	ActiveMS        int64  `json:"activeMS"`
	Context         string `json:"context"`
}

type HistoryAttribution struct {
	Source     string `json:"source"`
	Title      string `json:"title"`
	Speaker    string `json:"speaker"`
	Provenance string `json:"provenance"`
	License    string `json:"license"`
}

type HistoryRecord struct {
	ID                 string              `json:"id"`
	ContentID          string              `json:"contentId"`
	MetricVersion      string              `json:"metricVersion"`
	FinishedUnixMS     int64               `json:"finishedUnixMS"`
	Mode               string              `json:"mode"`
	SourceKind         string              `json:"sourceKind"`
	SourceLabel        string              `json:"sourceLabel"`
	Privacy            string              `json:"privacy"`
	PackID             string              `json:"packId"`
	PackRevision       string              `json:"packRevision"`
	PassageID          string              `json:"passageId"`
	DurationMS         int64               `json:"durationMS"`
	WordCount          int                 `json:"wordCount"`
	WordsPerGroup      int                 `json:"wordsPerGroup"`
	Groups             int                 `json:"groups"`
	Modifiers          HistoryModifiers    `json:"modifiers"`
	Difficulty         string              `json:"difficulty"`
	SkipWord           bool                `json:"skipWord"`
	AllowBackspace     bool                `json:"allowBackspace"`
	Raw                bool                `json:"raw"`
	Multi              bool                `json:"multi"`
	Outcome            string              `json:"outcome"`
	RetryOf            string              `json:"retryOf"`
	Practice           bool                `json:"practice"`
	Measurements       HistoryMeasurements `json:"measurements"`
	Fragments          []HistoryFragment   `json:"fragments"`
	Attribution        HistoryAttribution  `json:"attribution"`
	EligibilityReasons []string            `json:"eligibilityReasons"`
}

type historyEnvelope struct {
	SchemaVersion int           `json:"schemaVersion"`
	Session       HistoryRecord `json:"session"`
}

type historyStoreDocument struct {
	SchemaVersion int `json:"schemaVersion"`
}

type historyIndexEntry struct {
	ID      string        `json:"id"`
	File    string        `json:"file"`
	Size    int64         `json:"size"`
	MTimeNS int64         `json:"mtimeNS"`
	Summary HistoryRecord `json:"summary"`
}

type historyIndexDocument struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Generation    string              `json:"generation"`
	Entries       []historyIndexEntry `json:"entries"`
}

type historyLockOwner struct {
	PID           int    `json:"pid"`
	StartedUnixMS int64  `json:"startedUnixMS"`
	Token         string `json:"token"`
}

func HistoryRoot() string {
	return filepath.Join(filepath.Dir(RUNTIME_SETTINGS_DB), "history-v1")
}

func ProjectHistory(result SessionResult, origin ResourceOrigin, _ PrivacyPolicy) HistoryRecord {
	difficulty := result.Config.Difficulty
	if difficulty == "" {
		difficulty = "normal"
	}
	durationMS := int64(-1)
	if result.Config.TimeLimit >= 0 {
		durationMS = result.Config.TimeLimit.Milliseconds()
	}
	contentID := privateHistoryContentID(result.ID)
	privacy := "private"
	sourceKind := origin.Kind
	sourceLabel := privateSourceLabel(origin, result.Config.Source)
	packID, revision := "", ""
	public := origin.Embedded && (origin.Kind == "embedded-word" || origin.Kind == "embedded-quote")
	if public {
		privacy = "public"
		packID, revision = origin.PackID, origin.Revision
		sourceLabel = origin.PackID
		sum := sha256.Sum256([]byte(origin.Revision + "\x00" + result.Prompt))
		contentID = hex.EncodeToString(sum[:])
	}
	if sourceKind == "" {
		sourceKind = privateSourceKind(result.Config.Source)
	}
	measurements := projectMeasurements(result)
	record := HistoryRecord{
		ID: result.ID, ContentID: contentID, MetricVersion: result.MetricVersion,
		FinishedUnixMS: result.FinishedUnixMS, Mode: string(result.Config.Mode),
		SourceKind: sourceKind, SourceLabel: sourceLabel, Privacy: privacy,
		PackID: packID, PackRevision: revision, DurationMS: durationMS,
		WordCount: result.Config.WordCount, WordsPerGroup: result.Config.WordsPerGroup,
		Groups: result.Config.Groups, Difficulty: difficulty,
		Modifiers: HistoryModifiers(result.Config.Modifiers), SkipWord: result.SkipWord,
		AllowBackspace: result.AllowBackspace, Raw: result.Config.Raw, Multi: result.Config.Multi,
		Outcome: result.Outcome, RetryOf: result.RetryOf, Practice: result.Practice,
		Measurements: measurements, Fragments: []HistoryFragment{},
		EligibilityReasons: safeEligibilityReasons(result.EligibilityReasons),
	}
	if record.EligibilityReasons == nil {
		record.EligibilityReasons = []string{}
	}
	if public && result.Attribution != "" {
		record.Attribution.Source = result.Attribution
	}
	if public {
		record.Fragments = projectFragments(result)
	}
	return record
}

func projectMeasurements(result SessionResult) HistoryMeasurements {
	m := result.Measurements
	series := make([]HistorySample, len(m.Series))
	for i, sample := range m.Series {
		series[i] = HistorySample{EndMS: sample.EndMS, WindowMS: sample.WindowMS, WPM: sample.WPM, RawWPM: sample.RawWPM, Errors: sample.Errors}
	}
	return HistoryMeasurements{
		WPM: m.WPM, RawWPM: m.RawWPM, CPM: m.CPM, Accuracy: m.Accuracy, Consistency: m.Consistency,
		ActiveMS: m.ActiveMS, PauseMS: m.PauseMS, ActiveNS: result.ActiveNS, PauseNS: result.PauseNS,
		Characters: m.Characters, Errors: m.Errors, Attempts: m.Attempts,
		CorrectAttempts: m.CorrectAttempts, Series: series,
	}
}
func privateHistoryContentID(resultID string) string {
	sum := sha256.Sum256([]byte("tt-private-content-v1\x00" + resultID))
	return hex.EncodeToString(sum[:16])
}

func safeEligibilityReasons(reasons []string) []string {
	safe := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		switch reason {
		case "retry", "test is not PB-eligible":
			safe = append(safe, reason)
		default:
			safe = append(safe, "ineligible")
		}
	}
	return safe
}

func projectFragments(result SessionResult) []HistoryFragment {
	promptRunes := []rune(result.Prompt)
	trimmedPrompt := strings.TrimSpace(result.Prompt)
	fragments := make([]HistoryFragment, 0, 20)
	for _, word := range result.Measurements.Words {
		if word.Errors <= 0 || word.Start < 0 || word.End > len(promptRunes) || word.Start >= word.End {
			continue
		}
		item := string(promptRunes[word.Start:word.End])
		if utf8.RuneCountInString(item) > 64 || item == trimmedPrompt {
			continue
		}
		fragments = append(fragments, HistoryFragment{
			Item: item, Kind: "word", Occurrences: 1, Attempts: word.Attempts,
			CorrectAttempts: word.CorrectAttempts, Errors: word.Errors, ActiveMS: word.ActiveMS,
		})
		if len(fragments) == 20 {
			break
		}
	}
	if len(fragments) > 0 {
		retained := make(map[string]bool, len(fragments))
		for _, fragment := range fragments {
			retained[fragment.Item] = true
		}
		allWordsRetained := true
		for _, word := range strings.Fields(trimmedPrompt) {
			if !retained[word] {
				allWordsRetained = false
				break
			}
		}
		if allWordsRetained {
			return []HistoryFragment{}
		}
	}
	sort.SliceStable(fragments, func(i, j int) bool {
		if fragments[i].Errors != fragments[j].Errors {
			return fragments[i].Errors > fragments[j].Errors
		}
		return fragments[i].Item < fragments[j].Item
	})
	return fragments
}

func privateSourceKind(source testSource) string {
	switch source {
	case wordSource:
		return "private-word"
	case quoteSource:
		return "private-quote"
	case fileSource:
		return "file"
	case stdinSource:
		return "stdin"
	default:
		return "private-word"
	}
}

func privateSourceLabel(origin ResourceOrigin, source testSource) string {
	switch origin.Kind {
	case "file":
		return "File"
	case "stdin":
		return "Standard input"
	case "private-quote":
		return "Private quote list"
	case "private-word":
		return "Private word list"
	}
	switch source {
	case fileSource:
		return "File"
	case stdinSource:
		return "Standard input"
	case quoteSource:
		return "Private quote list"
	default:
		return "Private word list"
	}
}

func opaqueHistoryID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err == nil {
		return hex.EncodeToString(b[:])
	}
	return strings.Repeat("0", 32)
}

func SaveHistory(root string, record HistoryRecord) error {
	if err := validateHistoryRecord(record); err != nil {
		return fmt.Errorf("%w: record: %v", ErrUnavailable, err)
	}
	release, err := acquireHistoryLock(root)
	if err != nil {
		return err
	}
	defer release()
	if err := ensureHistoryStore(root); err != nil {
		return err
	}
	records, _, err := scanHistorySessions(root)
	if err != nil {
		return err
	}
	data, err := marshalHistoryJSON(historyEnvelope{SchemaVersion: historySchemaVersion, Session: record})
	if err != nil {
		return err
	}
	path := filepath.Join(root, "sessions", record.ID+".json")
	if existing, readErr := os.ReadFile(path); readErr == nil {
		if bytes.Equal(existing, data) {
			return nil
		}
		return fmt.Errorf("%w: %s", ErrConflict, path)
	} else if !os.IsNotExist(readErr) {
		return readErr
	}
	if err := atomicWrite(path, data, false); err != nil {
		return err
	}
	records = append(records, record)
	entries, err := historyEntries(root, records)
	if err == nil {
		_, _ = writeHistoryIndex(root, entries)
	}
	return nil
}

func ReadHistory(root string, query HistoryQuery) (HistoryPage, error) {
	if query.Offset < 0 || query.Limit < 0 || query.Limit > 100 {
		return HistoryPage{}, fmt.Errorf("invalid history query")
	}
	if query.Limit == 0 {
		query.Limit = 25
	}
	release, err := acquireHistoryLock(root)
	if err != nil {
		return HistoryPage{}, err
	}
	defer release()
	if err := ensureHistoryStore(root); err != nil {
		return HistoryPage{}, err
	}
	records, entries, err := scanHistorySessions(root)
	if err != nil {
		return HistoryPage{}, err
	}
	generation := validIndexGeneration(filepath.Join(root, "index.json"), entries)
	rebuilt := generation == ""
	if rebuilt {
		generation, err = writeHistoryIndex(root, entries)
		if err != nil {
			return HistoryPage{}, err
		}
	}
	filtered := records[:0]
	for _, record := range records {
		if record.Practice == query.Practice {
			filtered = append(filtered, record)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].FinishedUnixMS != filtered[j].FinishedUnixMS {
			return filtered[i].FinishedUnixMS > filtered[j].FinishedUnixMS
		}
		return filtered[i].ID > filtered[j].ID
	})
	total := len(filtered)
	if query.Offset >= total {
		return HistoryPage{Records: []HistoryRecord{}, Total: total, Generation: generation, Rebuilt: rebuilt}, nil
	}
	end := query.Offset + query.Limit
	if end > total {
		end = total
	}
	pageRecords := append([]HistoryRecord(nil), filtered[query.Offset:end]...)
	return HistoryPage{Records: pageRecords, Total: total, Generation: generation, Rebuilt: rebuilt}, nil
}

func acquireHistoryLock(root string) (func(), error) {
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	if err := os.Chmod(root, 0700); err != nil {
		return nil, err
	}
	lockPath := filepath.Join(root, ".write-lock")
	deadline := time.Now().Add(100 * time.Millisecond)
	for {
		err := os.Mkdir(lockPath, 0700)
		if err == nil {
			owner := historyLockOwner{PID: os.Getpid(), StartedUnixMS: time.Now().UTC().UnixMilli(), Token: opaqueHistoryID()}
			data, marshalErr := marshalHistoryJSON(owner)
			if marshalErr != nil {
				_ = os.Remove(lockPath)
				return nil, marshalErr
			}
			if writeErr := os.WriteFile(filepath.Join(lockPath, "owner.json"), data, 0600); writeErr != nil {
				_ = os.RemoveAll(lockPath)
				return nil, writeErr
			}
			return func() { _ = os.RemoveAll(lockPath) }, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}
		if !time.Now().Before(deadline) {
			return nil, fmt.Errorf("%w: %s; verify no tt writer is active, then remove this lock", ErrBusy, lockPath)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func ensureHistoryStore(root string) error {
	sessions := filepath.Join(root, "sessions")
	if err := os.MkdirAll(sessions, 0700); err != nil {
		return err
	}
	if err := os.Chmod(sessions, 0700); err != nil {
		return err
	}
	path := filepath.Join(root, "store.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		encoded, marshalErr := marshalHistoryJSON(historyStoreDocument{SchemaVersion: historySchemaVersion})
		if marshalErr != nil {
			return marshalErr
		}
		return atomicWrite(path, encoded, true)
	}
	if err != nil {
		return fmt.Errorf("%w: %s: %v", ErrUnavailable, path, err)
	}
	var store historyStoreDocument
	if err := json.Unmarshal(data, &store); err != nil || store.SchemaVersion != historySchemaVersion {
		return fmt.Errorf("%w: %s: unsupported or malformed store", ErrUnavailable, path)
	}
	return nil
}

func scanHistorySessions(root string) ([]HistoryRecord, []historyIndexEntry, error) {
	dir := filepath.Join(root, "sessions")
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %s: %v", ErrUnavailable, dir, err)
	}
	records := make([]HistoryRecord, 0, len(files))
	entries := make([]historyIndexEntry, 0, len(files))
	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), ".history-") {
			path := filepath.Join(dir, file.Name())
			if err := os.Remove(path); err != nil {
				return nil, nil, fmt.Errorf("%w: cannot remove interrupted write %s: %v", ErrUnavailable, path, err)
			}
			continue
		}
		if file.IsDir() {
			return nil, nil, fmt.Errorf("%w: %s: unexpected directory", ErrUnavailable, filepath.Join(dir, file.Name()))
		}
		if !validHistoryFilename(file.Name()) {
			return nil, nil, fmt.Errorf("%w: %s: invalid session filename", ErrUnavailable, filepath.Join(dir, file.Name()))
		}
		path := filepath.Join(dir, file.Name())
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, nil, fmt.Errorf("%w: %s: %v", ErrUnavailable, path, readErr)
		}
		if err := validateHistoryJSON(data); err != nil {
			return nil, nil, fmt.Errorf("%w: %s: invalid record: %v", ErrUnavailable, path, err)
		}
		var envelope historyEnvelope
		if err := json.Unmarshal(data, &envelope); err != nil || envelope.SchemaVersion != historySchemaVersion {
			return nil, nil, fmt.Errorf("%w: %s: unsupported or malformed record", ErrUnavailable, path)
		}
		if err := validateHistoryRecord(envelope.Session); err != nil || envelope.Session.ID+".json" != file.Name() {
			return nil, nil, fmt.Errorf("%w: %s: invalid record: %v", ErrUnavailable, path, err)
		}
		info, statErr := file.Info()
		if statErr != nil {
			return nil, nil, fmt.Errorf("%w: %s: %v", ErrUnavailable, path, statErr)
		}
		records = append(records, envelope.Session)
		entries = append(entries, historyIndexEntry{ID: envelope.Session.ID, File: file.Name(), Size: info.Size(), MTimeNS: info.ModTime().UnixNano(), Summary: historySummary(envelope.Session)})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].File < entries[j].File })
	return records, entries, nil
}

func validateHistoryRecord(record HistoryRecord) error {
	if !validHexID(record.ID, 32) || !(validHexID(record.ContentID, 32) || validHexID(record.ContentID, 64)) {
		return errors.New("invalid id or contentId")
	}
	if record.MetricVersion != metricVersion || record.FinishedUnixMS < 0 {
		return errors.New("invalid metric version or finish time")
	}
	validMode := map[string]bool{"words": true, "quote": true, "custom": true, "timed": true, "count": true, "practice": true}
	validSource := map[string]bool{"embedded-word": true, "embedded-quote": true, "approved-local": true, "private-word": true, "private-quote": true, "file": true, "stdin": true}
	if !validMode[record.Mode] || !validSource[record.SourceKind] || (record.Privacy != "public" && record.Privacy != "private") || record.SourceLabel == "" {
		return errors.New("invalid mode or source")
	}
	if record.DurationMS < -1 || record.WordCount < 0 || record.WordsPerGroup < 0 || record.Groups < 0 || record.Difficulty != "normal" {
		return errors.New("invalid configuration")
	}
	if record.Outcome != "completed" && record.Outcome != "expired" {
		return errors.New("invalid outcome")
	}
	if record.RetryOf != "" && !validHexID(record.RetryOf, 32) {
		return errors.New("invalid retry id")
	}
	if record.Measurements.ActiveMS < 0 || record.Measurements.PauseMS < 0 || record.Measurements.ActiveNS < 0 || record.Measurements.PauseNS < 0 ||
		record.Measurements.Attempts < 0 || record.Measurements.CorrectAttempts < 0 || len(record.Fragments) > 20 {
		return errors.New("invalid measurements")
	}
	if len(record.Fragments) > 0 &&
		(record.Privacy != "public" || (record.SourceKind != "embedded-word" && record.SourceKind != "embedded-quote")) {
		return errors.New("private source contains retained fragments")
	}
	for _, value := range []*float64{record.Measurements.WPM, record.Measurements.RawWPM, record.Measurements.CPM, record.Measurements.Accuracy, record.Measurements.Consistency} {
		if value != nil && (math.IsNaN(*value) || math.IsInf(*value, 0)) {
			return errors.New("non-finite measurement")
		}
	}
	for _, fragment := range record.Fragments {
		if (fragment.Kind != "word" && fragment.Kind != "pair") || fragment.Item == "" || utf8.RuneCountInString(fragment.Item) > 64 || len(strings.Fields(fragment.Context)) > 5 {
			return errors.New("invalid fragment")
		}
	}
	if record.Measurements.Series == nil || record.Fragments == nil || record.EligibilityReasons == nil {
		return errors.New("required array is missing")
	}
	return nil
}

func validHexID(value string, length int) bool {
	if len(value) != length {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validHistoryFilename(name string) bool {
	return len(name) == 37 && strings.HasSuffix(name, ".json") && validHexID(strings.TrimSuffix(name, ".json"), 32)
}

func historySummary(record HistoryRecord) HistoryRecord {
	record.Measurements.Series = nil
	record.Fragments = nil
	return record
}

func historyEntries(root string, records []HistoryRecord) ([]historyIndexEntry, error) {
	entries := make([]historyIndexEntry, 0, len(records))
	for _, record := range records {
		name := record.ID + ".json"
		info, err := os.Stat(filepath.Join(root, "sessions", name))
		if err != nil {
			return nil, err
		}
		entries = append(entries, historyIndexEntry{ID: record.ID, File: name, Size: info.Size(), MTimeNS: info.ModTime().UnixNano(), Summary: historySummary(record)})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].File < entries[j].File })
	return entries, nil
}

func validIndexGeneration(path string, entries []historyIndexEntry) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var index historyIndexDocument
	if json.Unmarshal(data, &index) != nil || index.SchemaVersion != historySchemaVersion || !validHexID(index.Generation, 32) || len(index.Entries) != len(entries) {
		return ""
	}
	for i := range entries {
		got, want := index.Entries[i], entries[i]
		if got.ID != want.ID || got.File != want.File || got.Size != want.Size || got.MTimeNS != want.MTimeNS || !validHistoryFilename(got.File) {
			return ""
		}
	}
	return index.Generation
}

func writeHistoryIndex(root string, entries []historyIndexEntry) (string, error) {
	generation := opaqueHistoryID()
	data, err := marshalHistoryIndex(historyIndexDocument{SchemaVersion: historySchemaVersion, Generation: generation, Entries: entries})
	if err != nil {
		return "", err
	}
	if err := atomicWrite(filepath.Join(root, "index.json"), data, true); err != nil {
		return "", err
	}
	return generation, nil
}

func marshalHistoryIndex(index historyIndexDocument) ([]byte, error) {
	data, err := json.Marshal(index)
	if err != nil {
		return nil, err
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	entries, _ := document["entries"].([]any)
	for _, rawEntry := range entries {
		entry, _ := rawEntry.(map[string]any)
		summary, _ := entry["summary"].(map[string]any)
		delete(summary, "fragments")
		measurements, _ := summary["measurements"].(map[string]any)
		delete(measurements, "series")
	}
	return marshalHistoryJSON(document)
}

func marshalHistoryJSON(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func validateHistoryJSON(data []byte) error {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(data, &envelope); err != nil {
		return err
	}
	for _, key := range []string{"schemaVersion", "session"} {
		if _, ok := envelope[key]; !ok {
			return fmt.Errorf("missing %s", key)
		}
	}
	var session map[string]json.RawMessage
	if err := json.Unmarshal(envelope["session"], &session); err != nil {
		return err
	}
	for _, key := range []string{
		"id", "contentId", "metricVersion", "finishedUnixMS", "mode", "sourceKind", "sourceLabel", "privacy",
		"packId", "packRevision", "passageId", "durationMS", "wordCount", "wordsPerGroup", "groups",
		"modifiers", "difficulty", "skipWord", "allowBackspace", "raw", "multi", "outcome", "retryOf",
		"practice", "measurements", "fragments", "attribution", "eligibilityReasons",
	} {
		if _, ok := session[key]; !ok {
			return fmt.Errorf("missing session.%s", key)
		}
	}
	var measurements map[string]json.RawMessage
	if err := json.Unmarshal(session["measurements"], &measurements); err != nil {
		return err
	}
	for _, key := range []string{
		"wpm", "rawWpm", "cpm", "accuracy", "consistency", "activeMS", "pauseMS", "activeNS", "pauseNS",
		"characters", "errors", "attempts", "correctAttempts", "series",
	} {
		if _, ok := measurements[key]; !ok {
			return fmt.Errorf("missing measurements.%s", key)
		}
	}
	for _, nested := range []struct {
		name string
		raw  json.RawMessage
		keys []string
	}{
		{"modifiers", session["modifiers"], []string{"punctuation", "numbers", "capitalization"}},
		{"characters", measurements["characters"], []string{"correct", "incorrect", "extra", "missed"}},
		{"errors", measurements["errors"], []string{"total", "corrected", "uncorrected"}},
		{"attribution", session["attribution"], []string{"source", "title", "speaker", "provenance", "license"}},
	} {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(nested.raw, &object); err != nil {
			return err
		}
		for _, key := range nested.keys {
			if _, ok := object[key]; !ok {
				return fmt.Errorf("missing %s.%s", nested.name, key)
			}
		}
	}
	var series []map[string]json.RawMessage
	if err := json.Unmarshal(measurements["series"], &series); err != nil {
		return err
	}
	for _, sample := range series {
		for _, key := range []string{"endMS", "windowMS", "wpm", "rawWpm", "errors"} {
			if _, ok := sample[key]; !ok {
				return fmt.Errorf("missing series.%s", key)
			}
		}
	}
	var fragments []map[string]json.RawMessage
	if err := json.Unmarshal(session["fragments"], &fragments); err != nil {
		return err
	}
	for _, fragment := range fragments {
		for _, key := range []string{"item", "kind", "occurrences", "attempts", "correctAttempts", "errors", "activeMS", "context"} {
			if _, ok := fragment[key]; !ok {
				return fmt.Errorf("missing fragment.%s", key)
			}
		}
	}
	return nil
}

func atomicWrite(path string, data []byte, replace bool) error {
	if err := injectHistoryFault("create", path); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".history-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	remove := true
	defer func() {
		if remove {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := injectHistoryFault("write", path); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := injectHistoryFault("sync", path); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := injectHistoryFault("rename", path); err != nil {
		return err
	}
	if runtime.GOOS == "windows" && replace {
		backup := path + ".backup"
		_ = os.Remove(backup)
		if _, statErr := os.Stat(path); statErr == nil {
			if err := os.Rename(path, backup); err != nil {
				return err
			}
		}
		if err := os.Rename(tmpPath, path); err != nil {
			_ = os.Rename(backup, path)
			return err
		}
		_ = os.Remove(backup)
	} else if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	remove = false
	if runtime.GOOS != "windows" {
		if err := injectHistoryFault("dir-sync", path); err != nil {
			return err
		}
		parent, err := os.Open(dir)
		if err != nil {
			return err
		}
		err = parent.Sync()
		closeErr := parent.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}
