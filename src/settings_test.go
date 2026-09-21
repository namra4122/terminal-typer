package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell"
)

func TestPersistedSettingsRoundTripAndValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.Chmod(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("relax settings directory mode: %v", err)
	}
	want := runtimeSettings{
		ShowWPM:        true,
		SkipWord:       false,
		AllowBackspace: false,
		BlockCursor:    true,
		BoldTypedText:  true,
		Highlight:      highlightNextOnly,
	}

	if err := savePersistedSettings(path, want); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	got, err := loadPersistedSettings(path)
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loaded settings = %#v, want %#v", got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat settings: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("settings mode = %04o, want 0600", info.Mode().Perm())
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat settings directory: %v", err)
	}
	if dirInfo.Mode().Perm() != 0700 {
		t.Fatalf("settings directory mode = %04o, want 0700", dirInfo.Mode().Perm())
	}

	invalid := []byte(`{"version":1,"settings":{"showWPM":false}}`)
	if err := os.WriteFile(path, invalid, 0600); err != nil {
		t.Fatalf("write invalid settings: %v", err)
	}
	if _, err := loadPersistedSettings(path); err == nil {
		t.Fatal("loadPersistedSettings accepted a document with missing fields")
	}
	unchanged, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read invalid settings: %v", err)
	}
	if string(unchanged) != string(invalid) {
		t.Fatalf("invalid settings file was modified: %q", unchanged)
	}
}

func TestEffectiveRuntimeSettingsUsesOnlyVisitedFlags(t *testing.T) {
	saved := runtimeSettings{
		ShowWPM:        true,
		SkipWord:       false,
		AllowBackspace: false,
		BlockCursor:    false,
		BoldTypedText:  true,
		Highlight:      highlightCurrentOnly,
	}
	flags := flagValues{
		ShowWPM:        false,
		SkipWord:       true,
		AllowBackspace: true,
		BlockCursor:    true,
		BoldTypedText:  false,
		Highlight:      highlightNextOnly,
	}
	overrides := collectSettingsOverrides(map[string]bool{
		"showwpm":    true,
		"highlight2": true,
	})

	got := effectiveRuntimeSettings(saved, overrides, flags)
	want := saved
	want.ShowWPM = false
	want.Highlight = highlightNextOnly
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("effective settings = %#v, want %#v", got, want)
	}
}

func TestSettingsModalMergesOnlyChangedRow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	originalPath := RUNTIME_SETTINGS_DB
	RUNTIME_SETTINGS_DB = path
	defer func() { RUNTIME_SETTINGS_DB = originalPath }()

	saved := defaultRuntimeSettings()
	onDisk := saved
	onDisk.ShowWPM = true
	if err := savePersistedSettings(path, onDisk); err != nil {
		t.Fatalf("seed settings: %v", err)
	}

	screen := newSettingsTestScreen(t, 80, 24)
	defer screen.Fini()
	result := make(chan [2]bool, 1)
	go func() {
		committed, interrupted := showSettings(screen, &saved, settingsOverrides{}, flagValues{})
		result <- [2]bool{committed, interrupted}
	}()
	screen.PostEventWait(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	screen.PostEventWait(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
	screen.PostEventWait(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))

	gotResult := <-result
	if gotResult != [2]bool{true, false} {
		t.Fatalf("modal result = %v, want committed without interrupt", gotResult)
	}
	if !saved.ShowWPM || saved.SkipWord {
		t.Fatalf("merged settings = %#v, want disk ShowWPM and changed SkipWord", saved)
	}
	reloaded, err := loadPersistedSettings(path)
	if err != nil {
		t.Fatalf("reload merged settings: %v", err)
	}
	if !reflect.DeepEqual(reloaded, saved) {
		t.Fatalf("persisted settings = %#v, want %#v", reloaded, saved)
	}
}

func TestSettingsModalSmallScreenIgnoresChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	originalPath := RUNTIME_SETTINGS_DB
	RUNTIME_SETTINGS_DB = path
	defer func() { RUNTIME_SETTINGS_DB = originalPath }()

	saved := defaultRuntimeSettings()
	screen := newSettingsTestScreen(t, 51, 13)
	defer screen.Fini()
	result := make(chan [2]bool, 1)
	go func() {
		committed, interrupted := showSettings(screen, &saved, settingsOverrides{}, flagValues{})
		result <- [2]bool{committed, interrupted}
	}()
	screen.PostEventWait(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
	screen.PostEventWait(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))

	if got := <-result; got != [2]bool{false, false} {
		t.Fatalf("modal result = %v, want unchanged close", got)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("settings file exists after undersized input: %v", err)
	}
	if got := simulationText(screen); !strings.Contains(got, "Terminal too small for settings (need 52x14)") {
		t.Fatalf("small-screen message not rendered:\n%s", got)
	}
}

func TestSettingsModalCtrlCInterruptsWithoutSaving(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	originalPath := RUNTIME_SETTINGS_DB
	RUNTIME_SETTINGS_DB = path
	defer func() { RUNTIME_SETTINGS_DB = originalPath }()

	saved := defaultRuntimeSettings()
	screen := newSettingsTestScreen(t, 80, 24)
	defer screen.Fini()
	result := make(chan [2]bool, 1)
	go func() {
		committed, interrupted := showSettings(screen, &saved, settingsOverrides{}, flagValues{})
		result <- [2]bool{committed, interrupted}
	}()
	screen.PostEventWait(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
	screen.PostEventWait(tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModNone))

	if got := <-result; got != [2]bool{false, true} {
		t.Fatalf("modal result = %v, want interrupt without commit", got)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("settings file exists after interrupt: %v", err)
	}
}

func TestApplyRuntimeSettingsMapsEffectiveControls(t *testing.T) {
	screen := newSettingsTestScreen(t, 80, 24)
	defer screen.Fini()

	typer := NewTyper(
		screen,
		false,
		tcell.ColorWhite,
		tcell.ColorBlack,
		tcell.ColorGreen,
		tcell.ColorYellow,
		tcell.ColorBlue,
		tcell.ColorRed,
	)
	var cursor bytes.Buffer
	typer.tty = &cursor
	typer.savedSettings = runtimeSettings{
		ShowWPM:        true,
		SkipWord:       false,
		AllowBackspace: false,
		BlockCursor:    true,
		BoldTypedText:  true,
		Highlight:      highlightOff,
	}
	typer.overrides = settingsOverrides{ShowWPM: true}
	typer.flagValues = flagValues{ShowWPM: false}
	typer.applyRuntimeSettings()

	if typer.ShowWpm || typer.SkipWord || !typer.DisableBackspace || !typer.BlockCursor {
		t.Fatalf("applied controls = ShowWpm:%t SkipWord:%t DisableBackspace:%t BlockCursor:%t",
			typer.ShowWpm, typer.SkipWord, typer.DisableBackspace, typer.BlockCursor)
	}
	if typer.currentWordStyle != typer.defaultStyle || typer.nextWordStyle != typer.defaultStyle {
		t.Fatal("highlight-off setting did not remove both word highlights")
	}
	_, _, attributes := typer.correctStyle.Decompose()
	if attributes&tcell.AttrBold == 0 {
		t.Fatal("bold typed-text setting did not embolden the correct style")
	}
	if cursor.String() != "\033[2 q" {
		t.Fatalf("cursor sequence = %q, want block cursor sequence", cursor.String())
	}
}

func TestTypingStateAndTimeSurviveSettingsModal(t *testing.T) {
	screen := newSettingsTestScreen(t, 80, 24)
	defer screen.Fini()

	typer := NewTyper(
		screen,
		false,
		tcell.ColorWhite,
		tcell.ColorBlack,
		tcell.ColorGreen,
		tcell.ColorYellow,
		tcell.ColorBlue,
		tcell.ColorRed,
	)
	typer.savedSettings = defaultRuntimeSettings()
	typer.applyRuntimeSettings()

	base := time.Unix(100, 0)
	times := []time.Time{
		base,
		base.Add(time.Second),
		base.Add(11 * time.Second),
		base.Add(12 * time.Second),
	}
	nextTime := 0
	typer.now = func() time.Time {
		value := times[nextTime]
		nextTime++
		return value
	}

	type typingResult struct {
		errors   int
		correct  int
		rc       int
		duration time.Duration
	}
	result := make(chan typingResult, 1)
	go func() {
		errors, correct, rc, duration, _ := typer.start("ab", -1, false, "")
		result <- typingResult{errors: errors, correct: correct, rc: rc, duration: duration}
	}()
	screen.PostEventWait(tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone))
	screen.PostEventWait(tcell.NewEventKey(tcell.KeyCtrlP, 0, tcell.ModNone))
	screen.PostEventWait(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	screen.PostEventWait(tcell.NewEventKey(tcell.KeyRune, 'b', tcell.ModNone))

	got := <-result
	if got.errors != 0 || got.correct != 2 || got.rc != TyperComplete {
		t.Fatalf("typing result = %#v, want two preserved correct characters", got)
	}
	if got.duration != 2*time.Second {
		t.Fatalf("typing duration = %s, want 2s excluding 10s modal interval", got.duration)
	}
	_, _, resumedStyle, _ := screen.GetContent(0, 0)
	if resumedStyle != typer.defaultStyle {
		t.Fatal("typing screen did not restore its themed default style after Settings")
	}
}

func TestInvalidRuntimeSettingsWarnOnceAndUseDefaults(t *testing.T) {
	validSettings := `{"showWPM":false,"skipWord":true,"allowBackspace":true,"blockCursor":false,"boldTypedText":false,"highlight":"current-and-next"}`
	cases := map[string]string{
		"invalid JSON":    `{`,
		"unknown version": `{"version":2,"settings":` + validSettings + `}`,
		"invalid mode":    `{"version":1,"settings":{"showWPM":false,"skipWord":true,"allowBackspace":true,"blockCursor":false,"boldTypedText":false,"highlight":"sometimes"}}`,
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				t.Fatalf("write settings: %v", err)
			}
			var warnings bytes.Buffer
			got := loadRuntimeSettings(path, &warnings)
			if !reflect.DeepEqual(got, defaultRuntimeSettings()) {
				t.Fatalf("fallback settings = %#v, want defaults", got)
			}
			if strings.Count(warnings.String(), "\n") != 1 ||
				!strings.HasPrefix(warnings.String(), "tt: ignoring runtime settings: ") {
				t.Fatalf("warning = %q, want exactly one runtime-settings warning", warnings.String())
			}
			unchanged, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read settings: %v", err)
			}
			if string(unchanged) != content {
				t.Fatalf("invalid settings changed from %q to %q", content, unchanged)
			}
		})
	}
}

func TestUnreadableRuntimeSettingsWarnOnceAndUseDefaults(t *testing.T) {
	path := t.TempDir()
	var warnings bytes.Buffer
	got := loadRuntimeSettings(path, &warnings)
	if !reflect.DeepEqual(got, defaultRuntimeSettings()) {
		t.Fatalf("fallback settings = %#v, want defaults", got)
	}
	if strings.Count(warnings.String(), "\n") != 1 ||
		!strings.HasPrefix(warnings.String(), "tt: ignoring runtime settings: ") {
		t.Fatalf("warning = %q, want exactly one runtime-settings warning", warnings.String())
	}
}

func TestAtomicSaveCleansTemporaryFileAfterRenameFailure(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "settings.json")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatalf("create conflicting target: %v", err)
	}
	if err := savePersistedSettings(target, defaultRuntimeSettings()); err == nil {
		t.Fatal("save succeeded with a directory as its target")
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatalf("read settings directory: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "settings.json" {
		t.Fatalf("temporary files left after failed save: %v", entries)
	}
}

func TestSettingsModalSelectionWraps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	originalPath := RUNTIME_SETTINGS_DB
	RUNTIME_SETTINGS_DB = path
	defer func() { RUNTIME_SETTINGS_DB = originalPath }()

	saved := defaultRuntimeSettings()
	screen := newSettingsTestScreen(t, 80, 24)
	defer screen.Fini()
	result := make(chan [2]bool, 1)
	go func() {
		committed, interrupted := showSettings(screen, &saved, settingsOverrides{}, flagValues{})
		result <- [2]bool{committed, interrupted}
	}()
	screen.PostEventWait(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone))
	screen.PostEventWait(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	screen.PostEventWait(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))

	if got := <-result; got != [2]bool{true, false} {
		t.Fatalf("modal result = %v, want committed close", got)
	}
	if saved.Highlight != highlightCurrentOnly {
		t.Fatalf("wrapped selection changed highlight to %q, want %q", saved.Highlight, highlightCurrentOnly)
	}
}

func TestSettingsModalSaveFailureRemainsOpenAndDoesNotApply(t *testing.T) {
	parent := t.TempDir()
	blocker := filepath.Join(parent, "blocker")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0600); err != nil {
		t.Fatalf("create path blocker: %v", err)
	}
	originalPath := RUNTIME_SETTINGS_DB
	RUNTIME_SETTINGS_DB = filepath.Join(blocker, "settings.json")
	defer func() { RUNTIME_SETTINGS_DB = originalPath }()

	saved := defaultRuntimeSettings()
	screen := newSettingsTestScreen(t, 80, 24)
	defer screen.Fini()
	result := make(chan [2]bool, 1)
	go func() {
		committed, interrupted := showSettings(screen, &saved, settingsOverrides{}, flagValues{})
		result <- [2]bool{committed, interrupted}
	}()
	screen.PostEventWait(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
	screen.PostEventWait(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	screen.PostEventWait(tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModNone))

	if got := <-result; got != [2]bool{false, true} {
		t.Fatalf("modal result = %v, want it to remain open until interrupted", got)
	}
	if !reflect.DeepEqual(saved, defaultRuntimeSettings()) {
		t.Fatalf("active saved settings changed after failed write: %#v", saved)
	}
	if rendered := simulationText(screen); !strings.Contains(rendered, "Error: ") {
		t.Fatalf("save failure was not rendered in modal:\n%s", rendered)
	}
}
func TestSettingsModalDoesNotSaveRevertedDraft(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	originalPath := RUNTIME_SETTINGS_DB
	RUNTIME_SETTINGS_DB = path
	defer func() { RUNTIME_SETTINGS_DB = originalPath }()

	saved := defaultRuntimeSettings()
	screen := newSettingsTestScreen(t, 80, 24)
	defer screen.Fini()
	result := make(chan [2]bool, 1)
	go func() {
		committed, interrupted := showSettings(screen, &saved, settingsOverrides{}, flagValues{})
		result <- [2]bool{committed, interrupted}
	}()
	screen.PostEventWait(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
	screen.PostEventWait(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
	screen.PostEventWait(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))

	if got := <-result; got != [2]bool{false, false} {
		t.Fatalf("modal result = %v, want unchanged close", got)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("settings file exists after reverting the draft: %v", err)
	}
}

func TestSettingsModalFitsHighlightOverrideAtMinimumSize(t *testing.T) {
	originalPath := RUNTIME_SETTINGS_DB
	RUNTIME_SETTINGS_DB = filepath.Join(t.TempDir(), "settings.json")
	defer func() { RUNTIME_SETTINGS_DB = originalPath }()

	saved := defaultRuntimeSettings()
	saved.Highlight = highlightOff
	overrides := settingsOverrides{Highlight: true}
	flags := flagValues{Highlight: highlightCurrentAndNext}
	screen := newSettingsTestScreen(t, 52, 14)
	defer screen.Fini()
	result := make(chan [2]bool, 1)
	go func() {
		committed, interrupted := showSettings(screen, &saved, overrides, flags)
		result <- [2]bool{committed, interrupted}
	}()
	screen.PostEventWait(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone))
	screen.PostEventWait(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))

	if got := <-result; got != [2]bool{false, false} {
		t.Fatalf("modal result = %v, want unchanged close", got)
	}
	rendered := simulationText(screen)
	for _, text := range []string{
		"Word highlighting: Current + next  CLI override",
		"Saved: Off",
	} {
		if !strings.Contains(rendered, text) {
			t.Errorf("minimum-size modal does not contain %q:\n%s", text, rendered)
		}
	}
}

func TestSettingsModalRendersRowsAndConsumesTypingKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	originalPath := RUNTIME_SETTINGS_DB
	RUNTIME_SETTINGS_DB = path
	defer func() { RUNTIME_SETTINGS_DB = originalPath }()

	saved := defaultRuntimeSettings()
	overrides := settingsOverrides{ShowWPM: true}
	flags := flagValues{ShowWPM: true}
	screen := newSettingsTestScreen(t, 100, 24)
	defer screen.Fini()
	result := make(chan [2]bool, 1)
	go func() {
		committed, interrupted := showSettings(screen, &saved, overrides, flags)
		result <- [2]bool{committed, interrupted}
	}()
	screen.PostEventWait(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone))
	screen.PostEventWait(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	screen.PostEventWait(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	screen.PostEventWait(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))

	if got := <-result; got != [2]bool{false, false} {
		t.Fatalf("modal result = %v, want unchanged close", got)
	}
	rendered := simulationText(screen)
	for _, text := range append(settingsLabels[:], "Settings", "CLI override", "Saved: Off", "Up/Down select · Space/Enter change", "Esc/Ctrl-P save & close") {
		if !strings.Contains(rendered, text) {
			t.Errorf("modal does not contain %q:\n%s", text, rendered)
		}
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("settings file exists after ignored typing keys: %v", err)
	}
}

func newSettingsTestScreen(t *testing.T, width, height int) tcell.SimulationScreen {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("initialize simulation screen: %v", err)
	}
	screen.SetSize(width, height)
	return screen
}

func simulationText(screen tcell.SimulationScreen) string {
	cells, width, height := screen.GetContents()
	var text strings.Builder
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			text.WriteRune(cells[y*width+x].Runes[0])
		}
		text.WriteByte('\n')
	}
	return text.String()
}
