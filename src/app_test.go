package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

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
	view := model.View()
	if !strings.Contains(view.Content, "e\u0301") || strings.Contains(view.Content, "e▏\u0301") {
		t.Fatalf("renderer split a grapheme cluster: %q", view.Content)
	}
	if view.Cursor == nil || view.Cursor.X != 0 || view.Cursor.Y != 2 {
		t.Fatalf("native cursor position = %#v; want x=0 y=2", view.Cursor)
	}
}

func TestCharmExpiryClampsLateInputAtTimeLimit(t *testing.T) {
	session := testSession("alpha")
	model := appModel{
		session:   session,
		settings:  defaultRuntimeSettings(),
		timeLimit: 0,
	}
	first, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "a", Code: 'a'}))
	model = first.(appModel)
	late, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "b", Code: 'b'}))
	model = late.(appModel)
	if model.session.State != SessionExpired || model.session.ActiveNS != 0 || model.session.Typed[0] != 'a' || model.session.Typed[1] != 0 {
		t.Fatalf("late input exceeded expiry: state=%s active=%d typed=%q", model.session.State, model.session.ActiveNS, string(model.session.Typed))
	}
}

func TestCharmSettlesTimedSessionBeforeLateKey(t *testing.T) {
	session := testSession("alpha")
	session.State = SessionRunning
	model := appModel{session: session, settings: defaultRuntimeSettings(), timeLimit: time.Nanosecond}
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "a", Code: 'a'}))
	model = updated.(appModel)
	if model.session.State != SessionExpired || model.session.ActiveNS != int64(time.Nanosecond) || model.session.Typed[0] != 0 {
		t.Fatalf("late key was accepted after expiry: state=%s active=%d typed=%q", model.session.State, model.session.ActiveNS, string(model.session.Typed))
	}
}

func TestCharmGuardsResultActionsFor200Milliseconds(t *testing.T) {
	session := testSession("a")
	if err := session.Apply(SessionInput{Kind: InputText, Text: "a", AtNS: sessionNow()}); err != nil {
		t.Fatal(err)
	}
	model := appModel{
		session:         session,
		settings:        defaultRuntimeSettings(),
		timeLimit:       -1,
		resultReadyAtNS: sessionNow() + int64(time.Second),
		generateTest:    func() *Test { return testSession("next").Test },
	}
	value, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = value.(appModel)
	if cmd != nil || model.generating {
		t.Fatal("result action activated inside the 200 ms guard")
	}
	model.resultReadyAtNS = sessionNow() - int64(200*time.Millisecond) - 1
	value, cmd = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = value.(appModel)
	if cmd == nil || !model.generating {
		t.Fatal("result action remained blocked after the guard")
	}
}

func TestCharmScrollsLongPromptAroundCurrentWord(t *testing.T) {
	var prompt strings.Builder
	for i := range 100 {
		fmt.Fprintf(&prompt, "word%03d ", i)
	}
	session := testSession(strings.TrimSpace(prompt.String()))
	for i := range 90 {
		if err := session.Apply(SessionInput{Kind: InputSkip, AtNS: int64(i + 1)}); err != nil {
			t.Fatal(err)
		}
	}
	settings := defaultRuntimeSettings()
	settings.Highlight = highlightOff
	model := appModel{
		session:  session,
		settings: settings,
		width:    52,
		height:   14,
	}
	view := model.View()
	if !strings.Contains(view.Content, "word090") || strings.Contains(view.Content, "word000") || !strings.Contains(view.Content, "…") {
		t.Fatalf("viewport did not follow the active word: %q", view.Content)
	}
	if view.Cursor == nil || view.Cursor.X < 0 || view.Cursor.X >= model.width || view.Cursor.Y >= model.height {
		t.Fatalf("native cursor escaped the visible viewport: %#v", view.Cursor)
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
	if strings.Contains(result, "NaN") || strings.Contains(result, "Inf") {
		t.Fatalf("basic result contains non-finite metrics: %s", result)

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

func TestCharmSettingsSaveCommitsMergedDraftAndResumes(t *testing.T) {
	originalPath := RUNTIME_SETTINGS_DB
	defer func() { RUNTIME_SETTINGS_DB = originalPath }()
	RUNTIME_SETTINGS_DB = filepath.Join(t.TempDir(), "settings.json")
	saved := defaultRuntimeSettings()
	draft := saved
	draft.ShowWPM = true
	session := testSession("alpha beta")
	now := sessionNow()
	if err := session.Apply(SessionInput{Kind: InputText, Text: "a", AtNS: now}); err != nil {
		t.Fatal(err)
	}
	if err := session.Apply(SessionInput{Kind: InputPause, Reason: "settings", AtNS: sessionNow()}); err != nil {
		t.Fatal(err)
	}
	model := appModel{
		session:       session,
		saved:         saved,
		settings:      saved,
		draftSettings: draft,
		settingsOpen:  true,
		dirty:         map[int]bool{settingShowWPM: true},
	}
	updated, _ := model.Update(model.saveSettings()())
	model = updated.(appModel)
	persisted, err := loadPersistedSettings(RUNTIME_SETTINGS_DB)
	if err != nil {
		t.Fatal(err)
	}
	if model.settingsOpen || model.saved != draft || model.settings != draft || model.session.State != SessionRunning || model.session.PauseReasons["settings"] || persisted != draft {
		t.Fatalf("settings save failed to commit and resume: open=%v active=%#v saved=%#v session=%#v persisted=%#v", model.settingsOpen, model.settings, model.saved, model.session, persisted)
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

func TestCharmResizePreservesAttemptAcrossMinimumSize(t *testing.T) {
	session := testSession("alpha beta")
	if err := session.Apply(SessionInput{Kind: InputText, Text: "a", AtNS: 1}); err != nil {
		t.Fatal(err)
	}
	promptID := session.PromptID
	typed := append([]rune(nil), session.Typed...)
	model := appModel{session: session, settings: defaultRuntimeSettings()}
	for _, size := range []struct {
		width, height int
		tooSmall      bool
	}{
		{80, 24, false},
		{52, 14, false},
		{40, 10, true},
		{80, 24, false},
	} {
		value, _ := model.Update(tea.WindowSizeMsg{Width: size.width, Height: size.height})
		model = value.(appModel)
		if model.tooSmall != size.tooSmall {
			t.Fatalf("size %dx%d tooSmall=%v", size.width, size.height, model.tooSmall)
		}
	}
	if model.session.PromptID != promptID || model.session.Cursor != 1 || !reflect.DeepEqual(model.session.Typed, typed) || model.session.State != SessionRunning {
		t.Fatalf("resize changed active attempt: %#v", model.session)
	}
}

func TestCharmRouteRejectsUnsupportedInvocation(t *testing.T) {
	config := TestConfig{Source: wordSource}
	allowed := map[string]bool{
		"n": true, "g": true, "t": true, "showwpm": true, "noskip": true,
		"nobackspace": true, "blockcursor": true, "bold": true,
		"nohighlight": true, "highlight1": true, "highlight2": true,
	}
	if !charmInvocationSupported(config, allowed, true) {
		t.Fatal("supported word-test invocation did not select Charm")
	}
	if charmInvocationSupported(config, allowed, false) {
		t.Fatal("non-terminal stdin selected Charm")
	}
	for _, source := range []testSource{quoteSource, stdinSource, fileSource} {
		config.Source = source
		if charmInvocationSupported(config, nil, true) {
			t.Fatalf("source %q selected Charm", source)
		}
	}
	config.Source = wordSource
	for _, name := range []string{"start", "w", "v", "words", "quotes", "notheme", "oneshot", "noreport", "csv", "json", "raw", "multi", "theme", "list", "sound", "error-sound"} {
		if charmInvocationSupported(config, map[string]bool{name: true}, true) {
			t.Fatalf("unsupported flag %q selected Charm", name)
		}
	}
}

func TestCharmResultsFreezeMetricsAndRetrySamePrompt(t *testing.T) {
	session := testSession("a")
	session.AttemptID = "first-attempt"
	if err := session.Apply(SessionInput{Kind: InputText, Text: "a", AtNS: 1}); err != nil {
		t.Fatal(err)
	}
	model := appModel{session: session, settings: defaultRuntimeSettings()}
	model.markResultReady(2)
	if model.result == nil || model.result.Measurements.Characters.Correct != 1 {
		t.Fatalf("completion did not freeze a result: %#v", model.result)
	}
	frozen := model.result
	model.markResultReady(3)
	if model.result != frozen {
		t.Fatal("duplicate completion replaced the frozen result")
	}
	session.Typed[0] = 'x'
	if model.result.Measurements.Characters.Correct != 1 {
		t.Fatal("frozen result changed with live session state")
	}
	view := model.View()
	if !strings.Contains(view.Content, "Results") || !strings.Contains(view.Content, "Enter: Next") || !strings.Contains(view.Content, "r: Retry") ||
		!strings.Contains(view.Content, "Final speeds can differ from interval speeds after corrections.") {
		t.Fatalf("Results view lacks metric/action content: %q", view.Content)
	}
	model.resultReadyAtNS = sessionNow() - int64(200*time.Millisecond) - 1
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "r", Code: 'r'}))
	model = updated.(appModel)
	if model.session.PromptID != "prompt" || model.session.RetryOf != "first-attempt" || model.session.State != SessionReady || string(model.session.prompt()) != "a" {
		t.Fatalf("retry did not restart the identical prompt: %#v", model.session)
	}
}

func TestCharmResultsRoundAccuracyToTwoDecimals(t *testing.T) {
	prompt := strings.Repeat("a", 300)
	session := testSession(prompt)
	for i := 0; i < 300; i++ {
		character := "a"
		if i >= 275 {
			character = "b"
		}
		if err := session.Apply(SessionInput{Kind: InputText, Text: character, AtNS: int64(i + 1)}); err != nil {
			t.Fatal(err)
		}
	}
	session.ActiveNS = 60e9
	model := appModel{session: session, settings: defaultRuntimeSettings()}
	model.markResultReady(60e9)
	if model.result == nil {
		t.Fatalf("failed to freeze result: %q", model.testError)
	}
	view := model.View().Content
	if !strings.Contains(view, "91.67%") {
		t.Fatalf("Results accuracy not rounded to two decimals: %q", view)
	}
}

func TestCharmLabelsShortSampleAndWaitsForOneSecondLiveSpeed(t *testing.T) {
	session := testSession("a b")
	if err := session.Apply(SessionInput{Kind: InputText, Text: "a", AtNS: 1}); err != nil {
		t.Fatal(err)
	}
	session.ActiveNS = 999_999_999
	settings := defaultRuntimeSettings()
	settings.ShowWPM = true
	model := appModel{session: session, settings: settings}
	if view := model.View().Content; strings.Contains(view, "WPM") {
		t.Fatalf("live speed appeared before one active second: %q", view)
	}
	session.ActiveNS = int64(time.Second)
	if view := model.View().Content; !strings.Contains(view, "WPM") {
		t.Fatalf("live speed remained unavailable at one active second: %q", view)
	}
	completed := testSession("a")
	if err := completed.Apply(SessionInput{Kind: InputText, Text: "a", AtNS: 1}); err != nil {
		t.Fatal(err)
	}
	completed.ActiveNS = int64(time.Millisecond)
	resultModel := appModel{session: completed, settings: settings}
	resultModel.markResultReady(1)
	if view := resultModel.View().Content; !strings.Contains(view, "short sample") {
		t.Fatalf("short final sample was not labeled: %q", view)
	}
}
