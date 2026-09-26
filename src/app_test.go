package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestCharmKeepsGraphemeClusterTogetherAtScalarCursor(t *testing.T) {
	session := testSession("e\u0301 x")
	if err := session.Apply(SessionInput{Kind: InputText, Text: "e", AtNS: 1}); err != nil {
		t.Fatal(err)
	}
	settings := defaultRuntimeSettings()
	settings.Highlight = highlightOff
	model := appModel{session: session, settings: settings}
	content := model.View().Content
	if !strings.Contains(content, "▏e\u0301") || strings.Contains(content, "e▏\u0301") {
		t.Fatalf("cursor split a grapheme cluster: %q", content)
	}
}

func TestCharmGeneratedGroupsCompleteWithBasicMetrics(t *testing.T) {
	cfg := resolveTestConfig(testOptions{StdinIsTerminal: true, WordsPerGroup: 10, Groups: 2, TimeoutSeconds: -1})
	generated := newTestGenerator(cfg, nil)()
	model := appModel{
		session:   NewSession(generated, "attempt", "prompt"),
		saved:     defaultRuntimeSettings(),
		settings:  defaultRuntimeSettings(),
		width:     80,
		height:    24,
		timeLimit: -1,
	}
	for _, r := range model.session.prompt() {
		if r == '\n' {
			continue
		}
		modelValue, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: string(r), Code: r}))
		model = modelValue.(appModel)
	}
	if model.session.State != SessionCompleted || len(model.session.Typed) != len(model.session.prompt()) || cfg.WordCount != 20 || cfg.Groups != 2 {
		t.Fatalf("generated typing journey incomplete: prompt=%q state=%s cursor=%d events=%#v words=%d groups=%d", string(model.session.prompt()), model.session.State, model.session.Cursor, model.session.Events, cfg.WordCount, cfg.Groups)
	}
	result := model.View().Content
	for _, metric := range []string{"WPM", "CPM", "Accuracy"} {
		if !strings.Contains(result, metric) {
			t.Fatalf("basic result missing %q: %s", metric, result)
		}
	}
}

func TestCharmRestartSafeguardPreservesThenRetriesPrompt(t *testing.T) {
	s := testSession("alpha beta")
	for i, r := range []rune("alpha") {
		if err := s.Apply(SessionInput{Kind: InputText, Text: string(r), AtNS: int64(i + 1)}); err != nil {
			t.Fatal(err)
		}
	}
	model := appModel{session: s, meaningful: true, saved: defaultRuntimeSettings(), settings: defaultRuntimeSettings(), timeLimit: -1}
	first, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model = first.(appModel)
	if !model.restartPending || model.session.State != SessionPaused || model.session.Cursor != 5 {
		t.Fatalf("first Escape changed attempt: pending=%v state=%s cursor=%d", model.restartPending, model.session.State, model.session.Cursor)
	}
	previousAttempt, promptID := model.session.AttemptID, model.session.PromptID
	second, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model = second.(appModel)
	if model.restartPending || model.session.AttemptID == previousAttempt || model.session.PromptID != promptID || model.session.RetryOf != previousAttempt || model.session.Cursor != 0 {
		t.Fatalf("second Escape retry = %#v", model.session)
	}
}

func TestCharmSettingsSaveFailureKeepsDraftAndOverlay(t *testing.T) {
	originalPath := RUNTIME_SETTINGS_DB
	defer func() { RUNTIME_SETTINGS_DB = originalPath }()
	blockedPath := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blockedPath, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	RUNTIME_SETTINGS_DB = filepath.Join(blockedPath, "settings.json")
	saved := defaultRuntimeSettings()
	draft := saved
	draft.ShowWPM = true
	model := appModel{
		session:       testSession("alpha beta"),
		saved:         saved,
		settings:      saved,
		draftSettings: draft,
		settingsOpen:  true,
		dirty:         map[int]bool{settingShowWPM: true},
	}
	cmd := model.closeSettings(sessionNow())
	if cmd == nil || !model.savingSettings {
		t.Fatal("settings save command was not scheduled")
	}
	updated, _ := model.Update(cmd())
	model = updated.(appModel)
	if !model.settingsOpen || model.saved != saved || model.settings != saved || model.draftSettings != draft || model.message == "" {
		t.Fatalf("failed save changed settings state: open=%v saved=%#v active=%#v draft=%#v message=%q", model.settingsOpen, model.saved, model.settings, model.draftSettings, model.message)
	}
}

func TestCharmNavigationCachesTestsAndMarksRetries(t *testing.T) {
	firstTest := &Test{Config: TestConfig{TimeLimit: -1}, Segments: []segment{{Text: "first"}}}
	secondTest := &Test{Config: TestConfig{TimeLimit: -1}, Segments: []segment{{Text: "second"}}}
	generated := 0
	model := appModel{
		session:      NewSession(firstTest, "attempt-first", "prompt-first"),
		saved:        defaultRuntimeSettings(),
		settings:     defaultRuntimeSettings(),
		tests:        []*testEntry{{test: firstTest, promptID: "prompt-first"}},
		attempts:     map[string]string{"prompt-first": "attempt-first"},
		generateTest: func() *Test { generated++; return secondTest },
		timeLimit:    -1,
	}
	value, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}))
	model = value.(appModel)
	if cmd == nil || !model.generating {
		t.Fatal("next test did not request asynchronous generation")
	}
	value, _ = model.Update(cmd())
	model = value.(appModel)
	if model.testIndex != 1 || model.session.Test != secondTest || generated != 1 {
		t.Fatalf("next test not cached: index=%d test=%p generated=%d", model.testIndex, model.session.Test, generated)
	}
	secondAttempt := model.session.AttemptID
	value, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}))
	model = value.(appModel)
	if model.testIndex != 0 || model.session.PromptID != "prompt-first" || model.session.RetryOf != "attempt-first" {
		t.Fatalf("previous prompt was not retried: %#v", model.session)
	}
	value, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}))
	model = value.(appModel)
	if model.testIndex != 1 || model.session.RetryOf != secondAttempt || generated != 1 {
		t.Fatalf("cached next prompt was not retried: index=%d retry=%q generated=%d", model.testIndex, model.session.RetryOf, generated)
	}
}
