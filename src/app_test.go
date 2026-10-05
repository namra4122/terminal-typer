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

func TestCharmViewSetsTerminalWideColors(t *testing.T) {
	model := appModel{
		session:  testSession("alpha beta"),
		settings: defaultRuntimeSettings(),
		width:    80,
		height:   24,
	}

	view := model.View()
	if view.ForegroundColor != charmForegroundColor {
		t.Fatalf("terminal foreground = %v; want %v", view.ForegroundColor, charmForegroundColor)
	}
	if view.BackgroundColor != charmBackgroundColor {
		t.Fatalf("terminal background = %v; want %v", view.BackgroundColor, charmBackgroundColor)
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

func TestCharmRouteAcceptsPresentationFlagsAndRejectsLegacyOnlyInput(t *testing.T) {
	config := TestConfig{Source: wordSource}
	allowed := map[string]bool{
		"n": true, "g": true, "t": true, "showwpm": true, "noskip": true,
		"nobackspace": true, "blockcursor": true, "bold": true,
		"nohighlight": true, "highlight1": true, "highlight2": true,
		"theme": true, "oneshot": true, "noreport": true, "csv": true, "json": true,
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
			t.Fatalf("source %q selected explicit Charm compatibility route", source)
		}
	}
	config.Source = wordSource
	for _, name := range []string{"start", "w", "v", "words", "quotes", "notheme", "raw", "multi", "list", "sound", "error-sound"} {
		if charmInvocationSupported(config, map[string]bool{name: true}, true) {
			t.Fatalf("legacy-only flag %q selected Charm", name)
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

func historyAppModel(t *testing.T, root string) appModel {
	t.Helper()
	test := &Test{
		Config: TestConfig{
			Mode: wordMode, Source: wordSource, Pack: "1000en", TimeLimit: -1,
			WordCount: 1, WordsPerGroup: 1, Groups: 1,
		},
		Origin:             ResourceOrigin{Kind: "embedded-word", PackID: "1000en", Revision: strings.Repeat("f", 64), Embedded: true},
		Segments:           []segment{{Text: "a"}},
		EligibleForHistory: true,
		EligibleForPB:      true,
	}
	session := NewSession(test, strings.Repeat("1", 32), strings.Repeat("2", 32))
	if err := session.Apply(SessionInput{Kind: InputText, Text: "a", AtNS: 1}); err != nil {
		t.Fatal(err)
	}
	return appModel{
		session: session, settings: defaultRuntimeSettings(), saved: defaultRuntimeSettings(),
		timeLimit: -1, historyRoot: root,
		tests:        []*testEntry{{test: test, promptID: session.PromptID}},
		attempts:     map[string]string{session.PromptID: session.AttemptID},
		generateTest: func() *Test { return test },
	}
}

func TestCharmCompletionSavesAndBrowsesDurableHistory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "history-v1")
	model := historyAppModel(t, root)
	cmd := model.markResultReady(2)
	if cmd == nil || model.saveState != SavePending || !strings.Contains(model.View().Content, "Saving") {
		t.Fatalf("completion save state = %q, view=%q", model.saveState, model.View().Content)
	}
	value, _ := model.Update(cmd())
	model = value.(appModel)
	if model.saveState != SaveSaved || !strings.Contains(model.View().Content, "Saved") {
		t.Fatalf("saved state = %q, view=%q", model.saveState, model.View().Content)
	}
	model.resultReadyAtNS = sessionNow() - int64(time.Second)
	value, cmd = model.Update(tea.KeyPressMsg(tea.Key{Text: "h", Code: 'h'}))
	model = value.(appModel)
	if cmd == nil || !model.historyOpen || !model.historyLoading {
		t.Fatalf("history did not open: %#v", model)
	}
	value, _ = model.Update(cmd())
	model = value.(appModel)
	if model.historyLoading || model.historyPage.Total != 1 || !strings.Contains(model.View().Content, "History · Regular") {
		t.Fatalf("history view = %#v, %q", model.historyPage, model.View().Content)
	}
	resultID := model.result.ID
	value, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model = value.(appModel)
	if model.historyOpen || model.result == nil || model.result.ID != resultID || model.session.State != SessionCompleted {
		t.Fatalf("Escape did not return to the same result: %#v", model)
	}
}

func TestCharmFailedSaveKeepsResultAndRetriesSameID(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	model := historyAppModel(t, filepath.Join(blocked, "history-v1"))
	cmd := model.markResultReady(2)
	resultID := model.result.ID
	value, _ := model.Update(cmd())
	model = value.(appModel)
	if model.saveState != SaveFailed || model.result == nil || model.result.ID != resultID ||
		!strings.Contains(model.View().Content, "not stored") || !strings.Contains(model.View().Content, "Retry save") {
		t.Fatalf("failed save state = %#v, %q", model, model.View().Content)
	}
	model.resultReadyAtNS = sessionNow() - int64(time.Second)
	model.historyRoot = filepath.Join(t.TempDir(), "history-v1")
	value, retry := model.Update(tea.KeyPressMsg(tea.Key{Text: "s", Code: 's'}))
	model = value.(appModel)
	if retry == nil || model.saveState != SavePending || model.historyRecord.ID != resultID {
		t.Fatalf("retry changed result identity: state=%q record=%#v", model.saveState, model.historyRecord)
	}
	value, _ = model.Update(retry())
	model = value.(appModel)
	if model.saveState != SaveSaved {
		t.Fatalf("retry state = %q error=%q", model.saveState, model.saveError)
	}
}

func TestCharmHistoryErrorNamesPathAndDoesNotBlockNextTest(t *testing.T) {
	root := filepath.Join(t.TempDir(), "history-v1")
	model := historyAppModel(t, root)
	if err := SaveHistory(root, ProjectHistory(*func() *SessionResult {
		model.markResultReady(2)
		return model.result
	}(), model.session.Test.Origin, PrivacyPolicy{})); err != nil {
		t.Fatal(err)
	}
	badPath := filepath.Join(root, "sessions", strings.Repeat("3", 32)+".json")
	if err := os.WriteFile(badPath, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	model.resultReadyAtNS = sessionNow() - int64(time.Second)
	value, load := model.Update(tea.KeyPressMsg(tea.Key{Text: "h", Code: 'h'}))
	model = value.(appModel)
	value, _ = model.Update(load())
	model = value.(appModel)
	if !strings.Contains(model.View().Content, badPath) {
		t.Fatalf("history error did not identify path: %q", model.View().Content)
	}
	value, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model = value.(appModel)
	value, next := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = value.(appModel)
	if next == nil || !model.generating {
		t.Fatalf("history failure blocked a fresh test: %#v", model)
	}
}

func TestCharmIgnoresStaleHistorySaveMessage(t *testing.T) {
	model := historyAppModel(t, t.TempDir())
	model.markResultReady(2)
	model.saveState = SavePending
	value, _ := model.Update(historySavedMsg{id: strings.Repeat("9", 32)})
	model = value.(appModel)
	if model.saveState != SavePending {
		t.Fatalf("stale save changed state to %q", model.saveState)
	}
}

func TestCharmTracksPendingSaveAcrossNextTest(t *testing.T) {
	model := historyAppModel(t, filepath.Join(t.TempDir(), "history-v1"))
	save := model.markResultReady(2)
	resultID := model.historyRecord.ID
	model.resultReadyAtNS = sessionNow() - int64(time.Second)
	value, next := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = value.(appModel)
	value, _ = model.Update(next())
	model = value.(appModel)
	if _, pending := model.pendingHistory[resultID]; !pending || model.result != nil {
		t.Fatalf("next test discarded pending result: pending=%#v result=%#v", model.pendingHistory, model.result)
	}
	value, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	model = value.(appModel)
	if model.quitting || !model.exitRequested {
		t.Fatalf("quit did not wait for pending save: quitting=%v requested=%v", model.quitting, model.exitRequested)
	}
	value, _ = model.Update(save())
	model = value.(appModel)
	if !model.quitting || len(model.pendingHistory) != 0 {
		t.Fatalf("completed pending save did not finish exit: quitting=%v pending=%#v", model.quitting, model.pendingHistory)
	}
}

func TestCharmIgnoresStaleHistoryLoadRequest(t *testing.T) {
	model := historyAppModel(t, t.TempDir())
	model.historyOpen = true
	model.historyLoading = true
	model.historyRequestID = 2
	model.historyPage = HistoryPage{Total: 1}
	value, _ := model.Update(historyLoadedMsg{requestID: 1, page: HistoryPage{Total: 99}})
	model = value.(appModel)
	if !model.historyLoading || model.historyPage.Total != 1 {
		t.Fatalf("stale load replaced current page: loading=%v page=%#v", model.historyLoading, model.historyPage)
	}
	value, _ = model.Update(historyLoadedMsg{requestID: 2, page: HistoryPage{Total: 2}})
	model = value.(appModel)
	if model.historyLoading || model.historyPage.Total != 2 {
		t.Fatalf("current load was ignored: loading=%v page=%#v", model.historyLoading, model.historyPage)
	}
}
func TestCharmPracticeReviewLoadsAsynchronouslyAndEscapePreservesResult(t *testing.T) {
	root := filepath.Join(t.TempDir(), "history-v1")
	test := &Test{
		Config:             TestConfig{Mode: wordMode, Source: wordSource, Pack: "1000en", TimeLimit: -1, WordCount: 1, WordsPerGroup: 1, Groups: 1},
		Origin:             ResourceOrigin{Kind: "embedded-word", PackID: "1000en", Revision: strings.Repeat("a", 64), Embedded: true},
		Segments:           []segment{{Text: "alpha"}},
		EligibleForHistory: true, EligibleForPB: true,
	}
	session := NewSession(test, strings.Repeat("1", 32), strings.Repeat("2", 32))
	if err := session.Apply(SessionInput{Kind: InputText, Text: "a", AtNS: 1}); err != nil {
		t.Fatal(err)
	}
	for i, r := range []rune("lpha") {
		if err := session.Apply(SessionInput{Kind: InputText, Text: string(r), AtNS: int64(i) + 2}); err != nil {
			t.Fatal(err)
		}
	}
	model := appModel{
		session: session, settings: defaultRuntimeSettings(), saved: defaultRuntimeSettings(),
		historyRoot: root, timeLimit: -1,
	}
	model.markResultReady(2)
	if model.result == nil {
		t.Fatalf("result unavailable after completion: state=%s err=%q", session.State, model.testError)
	}
	originID := model.result.ID
	model.resultReadyAtNS = sessionNow() - int64(time.Second)
	value, load := model.Update(tea.KeyPressMsg(tea.Key{Text: "p", Code: 'p'}))
	model = value.(appModel)
	if load == nil || !model.practiceReview || !model.practiceLoading {
		t.Fatalf("practice review did not start asynchronously: state=%s result=%v ready=%d now=%d review=%v loading=%v cmd=%v", model.session.State, model.result != nil, model.resultReadyAtNS, sessionNow(), model.practiceReview, model.practiceLoading, load != nil)
	}
	value, _ = model.Update(load())
	model = value.(appModel)
	if !strings.Contains(model.View().Content, "Practice") || !strings.Contains(model.View().Content, "insufficient") {
		t.Fatalf("degraded review did not explain evidence: %q", model.View().Content)
	}
	value, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model = value.(appModel)
	if model.practiceReview || model.result == nil || model.result.ID != originID || model.session != session {
		t.Fatalf("Escape did not preserve originating result: review=%v result=%#v", model.practiceReview, model.result)
	}
}
func TestCharmPracticeReviewDismissRestoreCancelKeepsRegularHistory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "history-v1")
	test := &Test{
		Config:   TestConfig{Mode: wordMode, Source: wordSource, Pack: "1000en", TimeLimit: -1, WordCount: 2, WordsPerGroup: 2, Groups: 1},
		Origin:   ResourceOrigin{Kind: "embedded-word", PackID: "1000en", Revision: strings.Repeat("b", 64), Embedded: true},
		Segments: []segment{{Text: "alpha beta"}}, EligibleForHistory: true, EligibleForPB: true,
	}
	session := NewSession(test, strings.Repeat("3", 32), strings.Repeat("4", 32))
	input := append([]rune{'x'}, []rune("lpha beta")...)
	for i, r := range input {
		if err := session.Apply(SessionInput{Kind: InputText, Text: string(r), AtNS: int64(i + 1)}); err != nil {
			t.Fatal(err)
		}
	}
	model := appModel{session: session, settings: defaultRuntimeSettings(), saved: defaultRuntimeSettings(), historyRoot: root, timeLimit: -1}
	save := model.markResultReady(sessionNow())
	if save == nil || model.result == nil {
		t.Fatalf("mistake result did not settle: state=%s result=%v", session.State, model.result != nil)
	}
	value, _ := model.Update(save())
	model = value.(appModel)
	model.resultReadyAtNS = sessionNow() - int64(time.Second)
	originID := model.result.ID
	value, load := model.Update(tea.KeyPressMsg(tea.Key{Text: "p", Code: 'p'}))
	model = value.(appModel)
	if load == nil || !model.practiceLoading {
		t.Fatal("practice history read was not asynchronous")
	}
	value, _ = model.Update(load())
	model = value.(appModel)
	if model.practicePlan == nil || len(model.practicePlan.Items) == 0 {
		t.Fatalf("mistake did not produce a practice candidate: %q", model.View().Content)
	}
	before := len(model.practicePlan.Items)
	value, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "delete"}))
	model = value.(appModel)
	if len(model.practicePlan.Items) != before-1 {
		t.Fatalf("Delete did not dismiss candidate: before=%d after=%d", before, len(model.practicePlan.Items))
	}
	value, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "r", Code: 'r'}))
	model = value.(appModel)
	if len(model.practicePlan.Items) != before {
		t.Fatalf("r did not restore candidate: got %d want %d", len(model.practicePlan.Items), before)
	}
	value, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model = value.(appModel)
	if model.practiceReview || model.result == nil || model.result.ID != originID || model.practiceActive {
		t.Fatalf("cancel changed regular result: review=%v result=%#v active=%v", model.practiceReview, model.result, model.practiceActive)
	}
	practicePage, err := ReadHistory(root, HistoryQuery{Practice: true})
	if err != nil {
		t.Fatal(err)
	}
	if practicePage.Total != 0 {
		t.Fatalf("cancel persisted practice history: %d records", practicePage.Total)
	}
}
func TestCharmPracticeRestorePreservesEvidenceRanking(t *testing.T) {
	ranked := []PracticeItem{{Item: "frequent", Reason: "error"}, {Item: "slower", Reason: "slow"}}
	model := appModel{
		session:           NewSession(&Test{Segments: []segment{{Text: "frequent slower"}}}, strings.Repeat("a", 32), strings.Repeat("b", 32)),
		practiceReview:    true,
		practicePlan:      &PracticePlan{Items: append([]PracticeItem(nil), ranked...)},
		practiceAllItems:  append([]PracticeItem(nil), ranked...),
		practiceDismissed: make(map[string]bool),
	}
	value, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "delete"}))
	model = value.(appModel)
	value, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "r", Code: 'r'}))
	model = value.(appModel)
	if len(model.practicePlan.Items) != 2 || model.practicePlan.Items[0].Item != "frequent" ||
		model.practicePlan.Items[1].Item != "slower" {
		t.Fatalf("restore changed ranked curriculum: %#v", model.practicePlan.Items)
	}
}

func TestCharmPracticeCompletesPersistsRepeatsAndEscapes(t *testing.T) {
	root := filepath.Join(t.TempDir(), "history-v1")
	originTest := &Test{
		Config:   TestConfig{Mode: wordMode, Source: wordSource, Pack: "1000en", TimeLimit: -1, WordCount: 1, WordsPerGroup: 1, Groups: 1},
		Origin:   ResourceOrigin{Kind: "embedded-word", PackID: "1000en", Revision: "", Embedded: true},
		Segments: []segment{{Text: "baseline"}}, EligibleForHistory: true, EligibleForPB: true,
	}
	originSession := NewSession(originTest, strings.Repeat("5", 32), strings.Repeat("6", 32))
	for i, r := range []rune("baseline") {
		if err := originSession.Apply(SessionInput{Kind: InputText, Text: string(r), AtNS: int64(i + 1)}); err != nil {
			t.Fatal(err)
		}
	}
	baseline, err := FinishResult(originSession.Snapshot(), time.Now().UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	plan := PracticePlan{
		ParentID: baseline.ID, Origin: originTest.Origin, SampleSessions: 2,
		Items: []PracticeItem{{Item: "alpha", Context: "alpha", Reason: "error", Occurrences: 2, Attempts: 2, CorrectAttempts: 1, Errors: 1, CurrentOccurrences: 1, HistoryOccurrences: 1}},
	}
	model := appModel{
		session: originSession, result: &baseline, settings: defaultRuntimeSettings(), saved: defaultRuntimeSettings(),
		historyRoot: root, practiceReview: true, practicePlan: &plan,
		practiceAllItems: append([]PracticeItem(nil), plan.Items...), practiceOriginResult: &baseline,
		practiceOriginSession: originSession, practiceOriginTest: originTest, practiceOriginTestIndex: 0,
		timeLimit: -1,
	}
	value, build := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = value.(appModel)
	if build == nil || !model.generating {
		t.Fatal("Enter did not schedule explicit practice start")
	}
	value, _ = model.Update(build())
	model = value.(appModel)
	if model.session == nil || !model.practiceActive || model.session.AttemptID == baseline.ID {
		t.Fatalf("practice attempt did not start: active=%v session=%#v", model.practiceActive, model.session)
	}
	firstAttempt := model.session.AttemptID
	var saveCmd tea.Cmd
	for _, r := range []rune(model.session.prompt()) {
		value, saveCmd = model.Update(tea.KeyPressMsg(tea.Key{Text: string(r), Code: r}))
		model = value.(appModel)
	}
	if saveCmd != nil {
		value, _ = model.Update(saveCmd())
		model = value.(appModel)
	}
	if model.result == nil || !model.practiceActive || model.session.State != SessionCompleted {
		t.Fatalf("practice completion did not produce results: state=%s result=%v", model.session.State, model.result != nil)
	}
	if model.historyRecord.PracticeDetail == nil || !model.historyRecord.Practice {
		t.Fatalf("practice detail was not attached: %#v", model.historyRecord)
	}
	practicePage, err := ReadHistory(root, HistoryQuery{Practice: true})
	if err != nil {
		t.Fatal(err)
	}
	if practicePage.Total != 1 || !practicePage.Records[0].Practice {
		t.Fatalf("practice history projection = %#v", practicePage)
	}
	model.resultReadyAtNS = sessionNow() - int64(time.Second)
	value, again := model.Update(tea.KeyPressMsg(tea.Key{Text: "p", Code: 'p'}))
	model = value.(appModel)
	if again == nil || !model.generating {
		t.Fatal("practice again did not schedule a fresh attempt")
	}
	value, _ = model.Update(again())
	model = value.(appModel)
	if model.session.AttemptID == firstAttempt {
		t.Fatal("practice again reused the completed attempt")
	}
	for _, r := range []rune(model.session.prompt()) {
		value, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: string(r), Code: r}))
		model = value.(appModel)
	}
	escapeModel := model
	value, _ = escapeModel.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	escapeModel = value.(appModel)
	if escapeModel.practiceActive || escapeModel.practiceReview || escapeModel.session != originSession || escapeModel.result.ID != baseline.ID {
		t.Fatalf("Escape did not restore originating result: active=%v review=%v session=%p result=%v", escapeModel.practiceActive, escapeModel.practiceReview, escapeModel.session, escapeModel.result.ID)
	}
	value, regular := model.Update(tea.KeyPressMsg(tea.Key{Text: "n", Code: 'n'}))
	model = value.(appModel)
	if regular == nil || !model.generating {
		t.Fatal("Return to regular test did not schedule fresh generation")
	}
	value, _ = model.Update(regular())
	model = value.(appModel)
	if model.practiceActive || model.session.Test == nil || model.session.Test.Config != originTest.Config {
		t.Fatalf("regular return did not restore configuration: active=%v test=%#v", model.practiceActive, model.session.Test)
	}
}

func timedAppModel(t *testing.T) appModel {
	t.Helper()
	stream, err := newDeterministicWordStream([]string{"alpha", "beta", "gamma", "delta"}, 17, 29)
	if err != nil {
		t.Fatal(err)
	}
	test := &Test{
		Config: TestConfig{
			Mode: timedMode, Source: wordSource, Pack: "1000en",
			TimeLimit: 30 * time.Second, WordCount: 200, WordsPerGroup: 200, Groups: 1,
			Difficulty: "normal",
		},
		SourceID: "words:1000en",
		Origin: ResourceOrigin{
			Kind: "embedded-word", PackID: "1000en", Revision: strings.Repeat("a", 64), Embedded: true,
		},
		Segments:           []segment{{Text: strings.Join(stream.next(200), " ")}},
		EligibleForHistory: true,
		EligibleForPB:      true,
		wordStream:         stream,
	}
	configuration := DefaultConfiguration()
	session := NewSession(test, "attempt", "prompt")
	now := sessionNow()
	session.State = SessionRunning
	session.StartedAtNS = now
	session.LastAtNS = now
	session.SkipWord = configuration.Settings.SkipWord
	session.AllowBackspace = configuration.Settings.AllowBackspace
	return appModel{
		session: session, saved: configuration.Settings, settings: configuration.Settings,
		draftSettings: configuration.Settings, configuration: configuration,
		timeLimit: test.Config.TimeLimit, tests: []*testEntry{{test: test, promptID: "prompt"}},
		attempts: map[string]string{"prompt": "attempt"},
	}
}

func promptBurst(prompt string, completedWords int) string {
	cut := 0
	for range completedWords {
		next := strings.IndexByte(prompt[cut:], ' ')
		if next < 0 {
			return prompt
		}
		cut += next + 1
	}
	return prompt[:cut]
}

func TestCharmTimedFastInputRefillsWithoutDroppingEventsAndRetryKeepsStream(t *testing.T) {
	model := timedAppModel(t)
	firstBurst := promptBurst(model.session.promptText, 151)
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: firstBurst}))
	model = updated.(appModel)
	if model.session.State != SessionRunning || model.session.Test.Config.WordCount != 300 {
		t.Fatalf("timed refill state=%s words=%d cursor=%d events=%d", model.session.State, model.session.Test.Config.WordCount, model.session.Cursor, len(model.session.Events))
	}
	if len(model.session.Events) != len([]rune(firstBurst)) {
		t.Fatalf("processed %d input events, want %d", len(model.session.Events), len([]rune(firstBurst)))
	}
	generatedPrefix := model.session.promptText
	previousAttempt := model.session.AttemptID
	if err := model.restartCurrentAttempt(); err != nil {
		t.Fatal(err)
	}
	if model.session.RetryOf != previousAttempt || model.session.promptText != generatedPrefix {
		t.Fatal("retry did not rewind the complete generated timed stream")
	}
	secondBurst := promptBurst(model.session.promptText, 251)
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: secondBurst}))
	model = updated.(appModel)
	if model.session.Test.Config.WordCount != 400 || !strings.HasPrefix(model.session.promptText, generatedPrefix+" ") {
		t.Fatalf("faster retry did not continue cached stream: words=%d", model.session.Test.Config.WordCount)
	}
}

func configurableAppModel(t *testing.T, configuration Configuration) appModel {
	t.Helper()
	cfg, err := ResolveLaunch(configuration, testOptions{StdinIsTerminal: true, TimeoutSeconds: -1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	generate, err := prepareTestGenerator(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	test := generate()
	session := NewSession(test, "attempt", "prompt")
	session.AllowBackspace = configuration.Settings.AllowBackspace
	session.SkipWord = configuration.Settings.SkipWord
	return appModel{
		session: session, saved: configuration.Settings, settings: configuration.Settings,
		draftSettings: configuration.Settings, configuration: configuration,
		timeLimit: cfg.TimeLimit, tests: []*testEntry{{test: test, promptID: "prompt"}},
		attempts: map[string]string{"prompt": "attempt"}, generateTest: generate,
	}
}

func commitTestConfiguration(t *testing.T, path string, configuration Configuration) {
	t.Helper()
	if err := CommitConfiguration(path, configuration, []string{"settings", "test", "appearance", "discoveredHints"}); err != nil {
		t.Fatal(err)
	}
}

func TestCharmConfigurePreviewAndCancelLeaveBytesAndSessionUnchanged(t *testing.T) {
	originalPath := RUNTIME_SETTINGS_DB
	defer func() { RUNTIME_SETTINGS_DB = originalPath }()
	RUNTIME_SETTINGS_DB = filepath.Join(t.TempDir(), "settings.json")
	configuration := DefaultConfiguration()
	commitTestConfiguration(t, RUNTIME_SETTINGS_DB, configuration)
	before, err := os.ReadFile(RUNTIME_SETTINGS_DB)
	if err != nil {
		t.Fatal(err)
	}
	model := configurableAppModel(t, configuration)
	originalSession, originalPrompt := model.session, model.session.promptText
	model.openConfigure(configureTestTab, sessionNow())
	model.configureSelected = 1
	model.updateConfigure("space", sessionNow())
	if model.draftConfiguration.Test.DurationSeconds != 60 {
		t.Fatalf("duration preview = %d, want 60", model.draftConfiguration.Test.DurationSeconds)
	}
	model.configureSelected = configureSpecificRows(configureTestTab) + 1
	model.updateConfigure("enter", sessionNow())
	if !model.configurePreview || !strings.Contains(model.View().Content, "Preview does not save") {
		t.Fatalf("preview view = %q", model.View().Content)
	}
	model.updateConfigure("escape", sessionNow())
	model.updateConfigure("escape", sessionNow())
	after, err := os.ReadFile(RUNTIME_SETTINGS_DB)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) || model.session != originalSession || model.session.promptText != originalPrompt ||
		model.session.PauseReasons["configure"] {
		t.Fatal("preview/cancel changed saved bytes or the active session")
	}
}

func TestCharmConfigureFailedStartPreservesSessionAndConfiguration(t *testing.T) {
	originalPath := RUNTIME_SETTINGS_DB
	defer func() { RUNTIME_SETTINGS_DB = originalPath }()
	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	RUNTIME_SETTINGS_DB = filepath.Join(blocked, "settings.json")
	configuration := DefaultConfiguration()
	model := configurableAppModel(t, configuration)
	for i, character := range []rune("alpha") {
		if err := model.session.Apply(SessionInput{Kind: InputText, Text: string(character), AtNS: int64(i + 1)}); err != nil {
			t.Fatal(err)
		}
	}
	model.refreshMeaningful()
	attempt, cursor, prompt := model.session.AttemptID, model.session.Cursor, model.session.promptText
	model.openConfigure(configureTestTab, sessionNow())
	model.draftConfiguration.Test.Mode = "count"
	model.draftConfiguration.Test.Count = 25
	model.draftConfiguration.Test.Pack = "1000en"
	model.markConfigurationDirty("test")
	if command := model.requestConfigurationStart(); command != nil || model.configureConfirm != "start" {
		t.Fatal("meaningful replacement did not require confirmation")
	}
	command := model.updateConfigure("enter", sessionNow())
	if command == nil || !model.savingConfiguration {
		t.Fatal("confirmed Start did not schedule the transaction")
	}
	updated, _ := model.Update(command())
	model = updated.(appModel)
	if !model.configureOpen || model.configureMessage == "" || model.session.AttemptID != attempt ||
		model.session.Cursor != cursor || model.session.promptText != prompt || !reflect.DeepEqual(model.configuration, configuration) {
		t.Fatalf("failed Start changed active state: %#v", model)
	}
}

func TestCharmConfigureStartPersistsCountAndStartsFreshContent(t *testing.T) {
	originalPath := RUNTIME_SETTINGS_DB
	defer func() { RUNTIME_SETTINGS_DB = originalPath }()
	RUNTIME_SETTINGS_DB = filepath.Join(t.TempDir(), "settings.json")
	configuration := DefaultConfiguration()
	model := configurableAppModel(t, configuration)
	oldPrompt := model.session.promptText
	model.openConfigure(configureTestTab, sessionNow())
	model.draftConfiguration.Test.Mode = "count"
	model.draftConfiguration.Test.Count = 25
	model.draftConfiguration.Test.Pack = "1000en"
	model.markConfigurationDirty("test")
	command := model.requestConfigurationStart()
	if command == nil {
		t.Fatal("Start did not schedule configuration commit")
	}
	updated, _ := model.Update(command())
	model = updated.(appModel)
	persisted, err := LoadConfiguration(RUNTIME_SETTINGS_DB)
	if err != nil {
		t.Fatal(err)
	}
	if model.configureOpen || model.session.Test.Config.Mode != wordMode || model.session.Test.Config.WordCount != 25 ||
		len(strings.Fields(model.session.promptText)) != 25 || model.session.promptText == oldPrompt ||
		persisted.Test.Mode != "count" || persisted.Test.Count != 25 {
		t.Fatalf("successful Start = model %#v persisted %#v", model, persisted)
	}
}

func TestCharmResetAllConfirmsRestoresDefaultsAndLeavesHistoryUntouched(t *testing.T) {
	originalPath := RUNTIME_SETTINGS_DB
	defer func() { RUNTIME_SETTINGS_DB = originalPath }()
	root := t.TempDir()
	RUNTIME_SETTINGS_DB = filepath.Join(root, "settings.json")
	configuration := DefaultConfiguration()
	configuration.Test.Mode = "count"
	configuration.Test.Count = 25
	configuration.Settings.ShowWPM = true
	configuration.Appearance.Focus = true
	configuration.DiscoveredHints = []string{hintConfigure}
	commitTestConfiguration(t, RUNTIME_SETTINGS_DB, configuration)
	historyPath := filepath.Join(root, "history-v1", "sentinel")
	if err := os.MkdirAll(filepath.Dir(historyPath), 0700); err != nil {
		t.Fatal(err)
	}
	historyBytes := []byte("history stays")
	if err := os.WriteFile(historyPath, historyBytes, 0600); err != nil {
		t.Fatal(err)
	}
	model := configurableAppModel(t, configuration)
	model.openConfigure(configureDataTab, sessionNow())
	model.configureSelected = 2
	if command := model.updateConfigure("enter", sessionNow()); command != nil || model.configureConfirm != "reset-all" {
		t.Fatal("reset all did not ask for confirmation")
	}
	command := model.updateConfigure("enter", sessionNow())
	if command == nil {
		t.Fatal("confirmed reset all did not schedule a transaction")
	}
	updated, _ := model.Update(command())
	model = updated.(appModel)
	persisted, err := LoadConfiguration(RUNTIME_SETTINGS_DB)
	if err != nil {
		t.Fatal(err)
	}
	afterHistory, err := os.ReadFile(historyPath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(persisted, DefaultConfiguration()) || !reflect.DeepEqual(afterHistory, historyBytes) ||
		!reflect.DeepEqual(model.configuration, DefaultConfiguration()) {
		t.Fatalf("reset all persisted=%#v history=%q active=%#v", persisted, afterHistory, model.configuration)
	}
}

func TestCharmConfigureGroupsOnlyExposeImplementedActions(t *testing.T) {
	model := configurableAppModel(t, DefaultConfiguration())
	model.openConfigure(configureTestTab, sessionNow())
	view := model.View().Content
	for _, group := range configureTabLabels {
		if !strings.Contains(view, group) {
			t.Fatalf("Configure is missing %q group: %q", group, view)
		}
	}
	if !strings.Contains(view, "Reset current section") || !strings.Contains(view, "Preview") ||
		!strings.Contains(view, "Start") || !strings.Contains(view, "Cancel") {
		t.Fatalf("Configure is missing footer actions: %q", view)
	}
	for range configureTabCount - 1 {
		model.updateConfigure("tab", sessionNow())
	}
	if model.configureTab != configureHelpTab || !strings.Contains(model.View().Content, "Dismiss first-run hints") {
		t.Fatalf("Tab journey did not reach Help: tab=%d view=%q", model.configureTab, model.View().Content)
	}
}

func TestCharmNoReportOneShotReturnsMachineResultWithoutResultScreen(t *testing.T) {
	session := testSession("a")
	if err := session.Apply(SessionInput{Kind: InputText, Text: "a", AtNS: 1}); err != nil {
		t.Fatal(err)
	}
	model := appModel{
		session: session, saved: defaultRuntimeSettings(), settings: defaultRuntimeSettings(),
		launchOptions: CharmLaunchOptions{OneShot: true, NoReport: true},
	}
	command := model.markResultReady(2)
	if command == nil || !model.quitting || !model.successfulExit || len(model.outputResults) != 1 {
		t.Fatalf("one-shot no-report completion = %#v", model)
	}
	if strings.Contains(model.View().Content, "Results") {
		t.Fatalf("no-report exposed the result screen: %q", model.View().Content)
	}
}

func TestCharmDismissedHintsPersistOnIntentionalStart(t *testing.T) {
	originalPath := RUNTIME_SETTINGS_DB
	defer func() { RUNTIME_SETTINGS_DB = originalPath }()
	RUNTIME_SETTINGS_DB = filepath.Join(t.TempDir(), "settings.json")
	model := configurableAppModel(t, DefaultConfiguration())
	model.openConfigure(configureHelpTab, sessionNow())
	model.configureSelected = 0
	model.updateConfigure("enter", sessionNow())
	model.configureSelected = configureSpecificRows(configureHelpTab) + 2
	command := model.updateConfigure("enter", sessionNow())
	if command == nil {
		t.Fatal("Start after dismissing hints did not schedule persistence")
	}
	updated, _ := model.Update(command())
	model = updated.(appModel)
	persisted, err := LoadConfiguration(RUNTIME_SETTINGS_DB)
	if err != nil {
		t.Fatal(err)
	}
	for _, hint := range []string{hintConfigure, hintSettings, hintHelp} {
		if !hasHint(persisted, hint) {
			t.Fatalf("hint %q was not persisted: %#v", hint, persisted.DiscoveredHints)
		}
	}
	if strings.Contains(model.View().Content, "Hint: Ctrl-K") {
		t.Fatalf("dismissed first-run hint remained visible: %q", model.View().Content)
	}
}

func TestCharmConfigureResetScopesDoNotCrossSections(t *testing.T) {
	configuration := DefaultConfiguration()
	configuration.Test.Mode = "count"
	configuration.Test.Count = 25
	configuration.Appearance.Focus = true
	configuration.Appearance.KeySound = "click"
	model := configurableAppModel(t, configuration)

	model.openConfigure(configureTestTab, sessionNow())
	model.configureSelected = configureSpecificRows(configureTestTab)
	model.updateConfigure("enter", sessionNow())
	if !reflect.DeepEqual(model.draftConfiguration.Test, DefaultConfiguration().Test) ||
		model.draftConfiguration.Appearance.KeySound != "click" {
		t.Fatalf("test-section reset crossed sections: %#v", model.draftConfiguration)
	}

	model.draftConfiguration = configuration
	model.configureTab = configureDisplayTab
	model.resetConfigureSection()
	if model.draftConfiguration.Appearance.Focus || model.draftConfiguration.Appearance.KeySound != "click" {
		t.Fatalf("display reset changed sound state: %#v", model.draftConfiguration.Appearance)
	}

	model.draftConfiguration = configuration
	model.configureTab = configureDataTab
	model.configureSelected = 1
	model.activateConfigureRow("enter", sessionNow())
	if !reflect.DeepEqual(model.draftConfiguration.Appearance, DefaultConfiguration().Appearance) {
		t.Fatalf("appearance reset = %#v", model.draftConfiguration.Appearance)
	}
}

func TestCharmMachineResultKeepsVisitedMistakesWithoutInventingTimedTail(t *testing.T) {
	session := testSession("alpha beta")
	for i, character := range "alpya" {
		if err := session.Apply(SessionInput{Kind: InputText, Text: string(character), AtNS: int64(i + 1)}); err != nil {
			t.Fatal(err)
		}
	}
	session.State = SessionExpired
	model := appModel{session: session, settings: defaultRuntimeSettings()}
	model.markResultReady(10)
	if len(model.outputResults) != 1 || !reflect.DeepEqual(model.outputResults[0].Mistakes, []mistake{{Word: "alpha", Typed: "alpya"}}) {
		t.Fatalf("machine result mistakes = %#v", model.outputResults)
	}
}
