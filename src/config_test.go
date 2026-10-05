package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

const version1Fixture = `{
  "version": 1,
  "settings": {
    "showWPM": true,
    "skipWord": false,
    "allowBackspace": false,
    "blockCursor": true,
    "boldTypedText": true,
    "highlight": "next-only"
  }
}
`

func TestDefaultConfigurationHasRequiredVersion2Values(t *testing.T) {
	configuration := DefaultConfiguration()
	if configuration.Version != 2 {
		t.Fatalf("version = %d, want 2", configuration.Version)
	}
	wantTest := SavedTest{
		Mode:            "timed",
		Pack:            "1000en",
		DurationSeconds: 30,
		Count:           50,
		Difficulty:      "normal",
	}
	if !reflect.DeepEqual(configuration.Test, wantTest) {
		t.Fatalf("default test = %#v, want %#v", configuration.Test, wantTest)
	}
	if !reflect.DeepEqual(configuration.Settings, defaultRuntimeSettings()) {
		t.Fatalf("default runtime settings = %#v, want %#v", configuration.Settings, defaultRuntimeSettings())
	}
	if configuration.Appearance != (Appearance{Theme: "tt-dark"}) {
		t.Fatalf("default appearance = %#v", configuration.Appearance)
	}
	if configuration.DiscoveredHints == nil || len(configuration.DiscoveredHints) != 0 {
		t.Fatalf("default discoveredHints = %#v, want empty array", configuration.DiscoveredHints)
	}
}

func TestValidateSavedTestChecksModeSpecificBoundariesAndPrivatePacks(t *testing.T) {
	base := DefaultConfiguration().Test
	cases := []struct {
		name    string
		change  func(*SavedTest)
		wantErr bool
	}{
		{name: "five seconds", change: func(test *SavedTest) { test.DurationSeconds = 5 }},
		{name: "fifteen-second preset", change: func(test *SavedTest) { test.DurationSeconds = 15 }},
		{name: "thirty-second preset", change: func(test *SavedTest) { test.DurationSeconds = 30 }},
		{name: "sixty-second preset", change: func(test *SavedTest) { test.DurationSeconds = 60 }},
		{name: "one-hundred-twenty-second preset", change: func(test *SavedTest) { test.DurationSeconds = 120 }},
		{name: "three thousand six hundred seconds", change: func(test *SavedTest) { test.DurationSeconds = 3600 }},
		{name: "four seconds", change: func(test *SavedTest) { test.DurationSeconds = 4 }, wantErr: true},
		{name: "three thousand six hundred one seconds", change: func(test *SavedTest) { test.DurationSeconds = 3601 }, wantErr: true},
		{name: "one word", change: func(test *SavedTest) { test.Mode, test.Count = "count", 1 }},
		{name: "five hundred words", change: func(test *SavedTest) { test.Mode, test.Count = "count", 500 }},
		{name: "ten-word preset", change: func(test *SavedTest) { test.Mode, test.Count = "count", 10 }},
		{name: "twenty-five-word preset", change: func(test *SavedTest) { test.Mode, test.Count = "count", 25 }},
		{name: "fifty-word preset", change: func(test *SavedTest) { test.Mode, test.Count = "count", 50 }},
		{name: "one-hundred-word preset", change: func(test *SavedTest) { test.Mode, test.Count = "count", 100 }},
		{name: "zero words", change: func(test *SavedTest) { test.Mode, test.Count = "count", 0 }, wantErr: true},
		{name: "five hundred one words", change: func(test *SavedTest) { test.Mode, test.Count = "count", 501 }, wantErr: true},
		{name: "authored quote", change: func(test *SavedTest) { test.Mode, test.Pack = "quote", "en" }},
		{name: "unknown mode", change: func(test *SavedTest) { test.Mode = "private" }, wantErr: true},
		{name: "path is not a pack", change: func(test *SavedTest) { test.Pack = "../1000en" }, wantErr: true},
		{name: "unknown pack", change: func(test *SavedTest) { test.Pack = "not-installed" }, wantErr: true},
		{name: "future difficulty", change: func(test *SavedTest) { test.Difficulty = "expert" }, wantErr: true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			test := base
			testCase.change(&test)
			err := ValidateSavedTest(test)
			if (err != nil) != testCase.wantErr {
				t.Fatalf("ValidateSavedTest(%#v) error = %v, wantErr %t", test, err, testCase.wantErr)
			}
		})
	}
}

func TestCommitConfigurationWritesExactRequiredVersion2Document(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	draft := DefaultConfiguration()
	draft.Settings.ShowWPM = true
	draft.Test.Modifiers = TestModifiers{Punctuation: true, Numbers: true, Capitalization: true}
	draft.Appearance.Focus = true
	draft.DiscoveredHints = []string{"configure", "settings"}
	if err := CommitConfiguration(path, draft, []string{"settings", "test", "appearance", "discoveredHints"}); err != nil {
		t.Fatalf("commit configuration: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read configuration: %v", err)
	}
	const want = `{
  "version": 2,
  "settings": {
    "showWPM": true,
    "skipWord": true,
    "allowBackspace": true,
    "blockCursor": false,
    "boldTypedText": false,
    "highlight": "current-and-next"
  },
  "test": {
    "mode": "timed",
    "pack": "1000en",
    "durationSeconds": 30,
    "count": 50,
    "modifiers": {
      "punctuation": true,
      "numbers": true,
      "capitalization": true
    },
    "difficulty": "normal"
  },
  "appearance": {
    "theme": "tt-dark",
    "focus": true,
    "showErrors": false,
    "reducedMotion": false,
    "keySound": "",
    "errorSound": "",
    "completionSound": "",
    "pbSound": ""
  },
  "discoveredHints": [
    "configure",
    "settings"
  ]
}
`
	if string(data) != want {
		t.Fatalf("configuration bytes:\n%s\nwant:\n%s", data, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat configuration: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("configuration mode = %04o, want 0600", info.Mode().Perm())
	}

	loaded, err := LoadConfiguration(path)
	if err != nil {
		t.Fatalf("load committed configuration: %v", err)
	}
	if !reflect.DeepEqual(loaded, draft) {
		t.Fatalf("loaded configuration = %#v, want %#v", loaded, draft)
	}
}

func TestLoadConfigurationRequiresEveryVersion2Field(t *testing.T) {
	complete, err := json.Marshal(configurationToJSON(DefaultConfiguration()))
	if err != nil {
		t.Fatalf("marshal complete fixture: %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(complete, &document); err != nil {
		t.Fatalf("decode complete fixture: %v", err)
	}
	cases := []struct {
		name   string
		remove func(map[string]any)
	}{
		{name: "settings", remove: func(document map[string]any) { delete(document, "settings") }},
		{name: "test", remove: func(document map[string]any) { delete(document, "test") }},
		{name: "appearance", remove: func(document map[string]any) { delete(document, "appearance") }},
		{name: "discovered hints", remove: func(document map[string]any) { delete(document, "discoveredHints") }},
		{name: "test modifiers", remove: func(document map[string]any) { delete(document["test"].(map[string]any), "modifiers") }},
		{name: "appearance sound", remove: func(document map[string]any) { delete(document["appearance"].(map[string]any), "pbSound") }},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var fixture map[string]any
			if err := json.Unmarshal(complete, &fixture); err != nil {
				t.Fatal(err)
			}
			testCase.remove(fixture)
			data, err := json.Marshal(fixture)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "settings.json")
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			got, err := LoadConfiguration(path)
			if err == nil {
				t.Fatal("LoadConfiguration accepted a document with a missing field")
			}
			if !reflect.DeepEqual(got, DefaultConfiguration()) {
				t.Fatalf("fallback = %#v, want defaults", got)
			}
			unchanged, readErr := os.ReadFile(path)
			if readErr != nil || string(unchanged) != string(data) {
				t.Fatalf("invalid document was changed: data=%q error=%v", unchanged, readErr)
			}
		})
	}
}

func TestLoadConfigurationMigratesVersion1WithVerifiedBackup(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "settings.json")
	if err := os.WriteFile(path, []byte(version1Fixture), 0644); err != nil {
		t.Fatalf("write version-1 fixture: %v", err)
	}

	configuration, err := LoadConfiguration(path)
	if err != nil {
		t.Fatalf("migrate configuration: %v", err)
	}
	wantSettings := runtimeSettings{
		ShowWPM: true, SkipWord: false, AllowBackspace: false,
		BlockCursor: true, BoldTypedText: true, Highlight: highlightNextOnly,
	}
	if !reflect.DeepEqual(configuration.Settings, wantSettings) {
		t.Fatalf("migrated settings = %#v, want %#v", configuration.Settings, wantSettings)
	}
	wantDefaults := DefaultConfiguration()
	if !reflect.DeepEqual(configuration.Test, wantDefaults.Test) || configuration.Appearance != wantDefaults.Appearance {
		t.Fatalf("new fields were not explicit defaults: %#v", configuration)
	}

	backups, err := filepath.Glob(filepath.Join(root, "backups", "settings-v1-*.json"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups = %v, error = %v, want one", backups, err)
	}
	if name := filepath.Base(backups[0]); len(name) != len("settings-v1-")+32+len(".json") {
		t.Fatalf("backup name = %q, want 32-hex identifier", name)
	}
	backup, err := os.ReadFile(backups[0])
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(backup) != version1Fixture {
		t.Fatalf("backup bytes changed:\n%s", backup)
	}
	info, err := os.Stat(backups[0])
	if err != nil {
		t.Fatalf("stat backup: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("backup mode = %v, want 0600", info.Mode())
	}

	restoredPath := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(restoredPath, backup, 0600); err != nil {
		t.Fatalf("restore backup in isolated root: %v", err)
	}
	restored, err := loadPersistedSettings(restoredPath)
	if err != nil {
		t.Fatalf("baseline settings reader rejected restored backup: %v", err)
	}
	if !reflect.DeepEqual(restored, wantSettings) {
		t.Fatalf("restored settings = %#v, want %#v", restored, wantSettings)
	}
}

func TestMigrationFailureLeavesOriginalVersion1Bytes(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "settings.json")
	if err := os.WriteFile(path, []byte(version1Fixture), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "backups"), []byte("block backup directory"), 0600); err != nil {
		t.Fatal(err)
	}

	configuration, err := LoadConfiguration(path)
	if err == nil || !strings.Contains(err.Error(), "original preserved") {
		t.Fatalf("migration error = %v, want preservation guidance", err)
	}
	if !reflect.DeepEqual(configuration, DefaultConfiguration()) {
		t.Fatalf("failed migration returned %#v, want read-only defaults", configuration)
	}
	unchanged, readErr := os.ReadFile(path)
	if readErr != nil || string(unchanged) != version1Fixture {
		t.Fatalf("original after failed migration = %q, error = %v", unchanged, readErr)
	}
}

func TestCorruptAndUnknownConfigurationsAreReadOnly(t *testing.T) {
	valid := strings.TrimSpace(mustConfigurationJSON(t, DefaultConfiguration()))
	unknownField := strings.TrimSuffix(valid, "}") + `,"future":true}`
	cases := map[string]string{
		"corrupt":         `{`,
		"unknown version": `{"version":99}`,
		"unknown field":   unknownField,
	}
	for name, fixture := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			if err := os.WriteFile(path, []byte(fixture), 0600); err != nil {
				t.Fatal(err)
			}
			configuration, err := LoadConfiguration(path)
			if err == nil || !strings.Contains(err.Error(), "fix or move the file") {
				t.Fatalf("load error = %v, want recovery guidance", err)
			}
			if !reflect.DeepEqual(configuration, DefaultConfiguration()) {
				t.Fatalf("fallback configuration = %#v", configuration)
			}
			unchanged, readErr := os.ReadFile(path)
			if readErr != nil || string(unchanged) != fixture {
				t.Fatalf("preserved bytes = %q, error = %v", unchanged, readErr)
			}
			if err := savePersistedSettings(path, defaultRuntimeSettings()); err == nil {
				t.Fatal("settings writer overwrote an unreadable configuration")
			}
			unchanged, readErr = os.ReadFile(path)
			if readErr != nil || string(unchanged) != fixture {
				t.Fatalf("settings writer changed preserved bytes = %q, error = %v", unchanged, readErr)
			}
		})
	}
}

func TestCommitConfigurationDirtyMergePreservesConcurrentEdits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	initial := DefaultConfiguration()
	if err := CommitConfiguration(path, initial, []string{"settings", "test", "appearance", "discoveredHints"}); err != nil {
		t.Fatalf("seed configuration: %v", err)
	}

	settingsDraft := initial
	settingsDraft.Settings.ShowWPM = true
	testDraft := initial
	testDraft.Test.Mode = "count"
	testDraft.Test.Count = 25
	start := make(chan struct{})
	errors := make(chan error, 2)
	var writers sync.WaitGroup
	writers.Add(2)
	go func() {
		defer writers.Done()
		<-start
		errors <- CommitConfiguration(path, settingsDraft, []string{"settings.showWPM"})
	}()
	go func() {
		defer writers.Done()
		<-start
		errors <- CommitConfiguration(path, testDraft, []string{"test"})
	}()
	close(start)
	writers.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("concurrent commit: %v", err)
		}
	}

	got, err := LoadConfiguration(path)
	if err != nil {
		t.Fatalf("load merged configuration: %v", err)
	}
	if !got.Settings.ShowWPM || got.Test.Mode != "count" || got.Test.Count != 25 {
		t.Fatalf("dirty merge lost an edit: %#v", got)
	}
}

func TestConcurrentCommitsMigrateLatestVersion1SettingsOnce(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "settings.json")
	if err := os.WriteFile(path, []byte(version1Fixture), 0600); err != nil {
		t.Fatal(err)
	}

	settingsDraft := DefaultConfiguration()
	settingsDraft.Settings.ShowWPM = false
	testDraft := DefaultConfiguration()
	testDraft.Test.Mode = "count"
	testDraft.Test.Count = 25
	start := make(chan struct{})
	errors := make(chan error, 2)
	var writers sync.WaitGroup
	writers.Add(2)
	go func() {
		defer writers.Done()
		<-start
		errors <- CommitConfiguration(path, settingsDraft, []string{"settings.showWPM"})
	}()
	go func() {
		defer writers.Done()
		<-start
		errors <- CommitConfiguration(path, testDraft, []string{"test"})
	}()
	close(start)
	writers.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("concurrent migration commit: %v", err)
		}
	}

	got, err := LoadConfiguration(path)
	if err != nil {
		t.Fatalf("load migrated configuration: %v", err)
	}
	if got.Settings.ShowWPM || got.Settings.SkipWord || got.Settings.AllowBackspace ||
		!got.Settings.BlockCursor || !got.Settings.BoldTypedText || got.Settings.Highlight != highlightNextOnly {
		t.Fatalf("version-1 runtime fields were lost: %#v", got.Settings)
	}
	if got.Test.Mode != "count" || got.Test.Count != 25 {
		t.Fatalf("concurrent test edit was lost: %#v", got.Test)
	}
	backups, err := filepath.Glob(filepath.Join(root, "backups", "settings-v1-*.json"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("migration backups = %v, error = %v, want one", backups, err)
	}
}

func TestCompatibilitySettingsSavePreservesRememberedTest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	configuration := DefaultConfiguration()
	configuration.Test.Mode = "count"
	configuration.Test.Count = 25
	configuration.Appearance.Focus = true
	configuration.DiscoveredHints = []string{"configure"}
	if err := CommitConfiguration(path, configuration, []string{"test", "appearance", "discoveredHints"}); err != nil {
		t.Fatalf("seed configuration: %v", err)
	}

	settings := defaultRuntimeSettings()
	settings.ShowWPM = true
	settings.Highlight = highlightOff
	if err := savePersistedSettings(path, settings); err != nil {
		t.Fatalf("compatibility settings save: %v", err)
	}
	got, err := LoadConfiguration(path)
	if err != nil {
		t.Fatalf("load after compatibility save: %v", err)
	}
	if !reflect.DeepEqual(got.Settings, settings) {
		t.Fatalf("saved settings = %#v, want %#v", got.Settings, settings)
	}
	if !reflect.DeepEqual(got.Test, configuration.Test) || got.Appearance != configuration.Appearance ||
		!reflect.DeepEqual(got.DiscoveredHints, configuration.DiscoveredHints) {
		t.Fatalf("compatibility save erased version-2 fields: %#v", got)
	}

	emptyHints := got
	emptyHints.DiscoveredHints = []string{}
	if err := CommitConfiguration(path, emptyHints, []string{"discoveredHints"}); err != nil {
		t.Fatalf("clear discovered hints: %v", err)
	}
	cleared, err := LoadConfiguration(path)
	if err != nil {
		t.Fatalf("load cleared hints: %v", err)
	}
	if cleared.DiscoveredHints == nil || len(cleared.DiscoveredHints) != 0 {
		t.Fatalf("cleared discoveredHints = %#v, want required empty array", cleared.DiscoveredHints)
	}
}
func TestCommitFailurePreservesOriginalConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	configuration := DefaultConfiguration()
	if err := CommitConfiguration(path, configuration, []string{"test"}); err != nil {
		t.Fatalf("seed configuration: %v", err)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	draft := configuration
	draft.Settings.ShowWPM = true
	if err := CommitConfiguration(path, draft, []string{"settings.notAField"}); err == nil {
		t.Fatal("commit accepted an unknown dirty path")
	}
	unchanged, err := os.ReadFile(path)
	if err != nil || string(unchanged) != string(original) {
		t.Fatalf("configuration changed after invalid commit: data=%q error=%v", unchanged, err)
	}

	if err := os.Remove(path + ".lock"); err != nil {
		t.Fatalf("remove lock file to install blocker: %v", err)
	}
	if err := os.Mkdir(path+".lock", 0700); err != nil {
		t.Fatalf("create lock blocker: %v", err)
	}
	if err := CommitConfiguration(path, draft, []string{"settings.showWPM"}); err == nil {
		t.Fatal("commit unexpectedly acquired a directory as its lock file")
	}
	unchanged, err = os.ReadFile(path)
	if err != nil || string(unchanged) != string(original) {
		t.Fatalf("configuration changed after lock failure: data=%q error=%v", unchanged, err)
	}
}

func mustConfigurationJSON(t *testing.T, configuration Configuration) string {
	t.Helper()
	data, err := marshalConfiguration(configuration)
	if err != nil {
		t.Fatalf("marshal configuration: %v", err)
	}
	return string(data)
}
