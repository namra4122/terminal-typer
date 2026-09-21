package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gdamore/tcell"
)

type highlightMode string

const (
	highlightCurrentAndNext highlightMode = "current-and-next"
	highlightCurrentOnly    highlightMode = "current-only"
	highlightNextOnly       highlightMode = "next-only"
	highlightOff            highlightMode = "off"
)

type runtimeSettings struct {
	ShowWPM        bool          `json:"showWPM"`
	SkipWord       bool          `json:"skipWord"`
	AllowBackspace bool          `json:"allowBackspace"`
	BlockCursor    bool          `json:"blockCursor"`
	BoldTypedText  bool          `json:"boldTypedText"`
	Highlight      highlightMode `json:"highlight"`
}

type persistedSettings struct {
	Version  int             `json:"version"`
	Settings runtimeSettings `json:"settings"`
}

type settingsOverrides struct {
	ShowWPM        bool
	SkipWord       bool
	AllowBackspace bool
	BlockCursor    bool
	BoldTypedText  bool
	Highlight      bool
}

type flagValues struct {
	ShowWPM        bool
	SkipWord       bool
	AllowBackspace bool
	BlockCursor    bool
	BoldTypedText  bool
	Highlight      highlightMode
}

type runtimeSettingsWire struct {
	ShowWPM        *bool          `json:"showWPM"`
	SkipWord       *bool          `json:"skipWord"`
	AllowBackspace *bool          `json:"allowBackspace"`
	BlockCursor    *bool          `json:"blockCursor"`
	BoldTypedText  *bool          `json:"boldTypedText"`
	Highlight      *highlightMode `json:"highlight"`
}

type persistedSettingsWire struct {
	Version  *int                 `json:"version"`
	Settings *runtimeSettingsWire `json:"settings"`
}

func defaultRuntimeSettings() runtimeSettings {
	return runtimeSettings{
		ShowWPM:        false,
		SkipWord:       true,
		AllowBackspace: true,
		BlockCursor:    false,
		BoldTypedText:  false,
		Highlight:      highlightCurrentAndNext,
	}
}

func loadRuntimeSettings(path string, warnings io.Writer) runtimeSettings {
	settings, err := loadPersistedSettings(path)
	if err == nil {
		return settings
	}
	if !os.IsNotExist(err) {
		fmt.Fprintf(warnings, "tt: ignoring runtime settings: %s\n", err)
	}
	return defaultRuntimeSettings()
}

func validHighlightMode(mode highlightMode) bool {
	switch mode {
	case highlightCurrentAndNext, highlightCurrentOnly, highlightNextOnly, highlightOff:
		return true
	default:
		return false
	}
}

func loadPersistedSettings(path string) (runtimeSettings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return runtimeSettings{}, err
	}

	var wire persistedSettingsWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return runtimeSettings{}, err
	}
	if wire.Version == nil || *wire.Version != 1 {
		return runtimeSettings{}, fmt.Errorf("unsupported settings version")
	}
	if wire.Settings == nil || wire.Settings.ShowWPM == nil || wire.Settings.SkipWord == nil ||
		wire.Settings.AllowBackspace == nil || wire.Settings.BlockCursor == nil ||
		wire.Settings.BoldTypedText == nil || wire.Settings.Highlight == nil {
		return runtimeSettings{}, errors.New("settings document is missing required fields")
	}
	if !validHighlightMode(*wire.Settings.Highlight) {
		return runtimeSettings{}, fmt.Errorf("invalid highlight mode %q", *wire.Settings.Highlight)
	}

	return runtimeSettings{
		ShowWPM:        *wire.Settings.ShowWPM,
		SkipWord:       *wire.Settings.SkipWord,
		AllowBackspace: *wire.Settings.AllowBackspace,
		BlockCursor:    *wire.Settings.BlockCursor,
		BoldTypedText:  *wire.Settings.BoldTypedText,
		Highlight:      *wire.Settings.Highlight,
	}, nil
}

func savePersistedSettings(path string, settings runtimeSettings) error {
	if !validHighlightMode(settings.Highlight) {
		return fmt.Errorf("invalid highlight mode %q", settings.Highlight)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	if err := os.Chmod(filepath.Dir(path), 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(persistedSettings{Version: 1, Settings: settings}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(path), ".settings-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	removeTemp := true
	defer func() {
		if removeTemp {
			os.Remove(tmpPath)
		}
	}()

	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	removeTemp = false
	return nil
}

func collectSettingsOverrides(visited map[string]bool) settingsOverrides {
	return settingsOverrides{
		ShowWPM:        visited["showwpm"],
		SkipWord:       visited["noskip"],
		AllowBackspace: visited["nobackspace"],
		BlockCursor:    visited["blockcursor"],
		BoldTypedText:  visited["bold"],
		Highlight:      visited["nohighlight"] || visited["highlight1"] || visited["highlight2"],
	}
}

func effectiveRuntimeSettings(saved runtimeSettings, overrides settingsOverrides, flags flagValues) runtimeSettings {
	effective := saved
	if overrides.ShowWPM {
		effective.ShowWPM = flags.ShowWPM
	}
	if overrides.SkipWord {
		effective.SkipWord = flags.SkipWord
	}
	if overrides.AllowBackspace {
		effective.AllowBackspace = flags.AllowBackspace
	}
	if overrides.BlockCursor {
		effective.BlockCursor = flags.BlockCursor
	}
	if overrides.BoldTypedText {
		effective.BoldTypedText = flags.BoldTypedText
	}
	if overrides.Highlight {
		effective.Highlight = flags.Highlight
	}
	return effective
}

const (
	settingShowWPM = iota
	settingSkipWord
	settingAllowBackspace
	settingCursorStyle
	settingTypedTextWeight
	settingWordHighlighting
	settingsRowCount
)

var settingsLabels = [settingsRowCount]string{
	"Show WPM",
	"Skip word on Space",
	"Allow Backspace",
	"Cursor style",
	"Typed text weight",
	"Word highlighting",
}

func settingIsOverridden(row int, overrides settingsOverrides) bool {
	switch row {
	case settingShowWPM:
		return overrides.ShowWPM
	case settingSkipWord:
		return overrides.SkipWord
	case settingAllowBackspace:
		return overrides.AllowBackspace
	case settingCursorStyle:
		return overrides.BlockCursor
	case settingTypedTextWeight:
		return overrides.BoldTypedText
	case settingWordHighlighting:
		return overrides.Highlight
	default:
		return false
	}
}

func settingValue(settings runtimeSettings, row int) string {
	switch row {
	case settingShowWPM:
		if settings.ShowWPM {
			return "On"
		}
		return "Off"
	case settingSkipWord:
		if settings.SkipWord {
			return "On"
		}
		return "Off"
	case settingAllowBackspace:
		if settings.AllowBackspace {
			return "On"
		}
		return "Off"
	case settingCursorStyle:
		if settings.BlockCursor {
			return "Block"
		}
		return "Bar"
	case settingTypedTextWeight:
		if settings.BoldTypedText {
			return "Bold"
		}
		return "Normal"
	case settingWordHighlighting:
		switch settings.Highlight {
		case highlightCurrentOnly:
			return "Current only"
		case highlightNextOnly:
			return "Next only"
		case highlightOff:
			return "Off"
		default:
			return "Current + next"
		}
	default:
		return ""
	}
}

func advanceSetting(settings *runtimeSettings, row int) {
	switch row {
	case settingShowWPM:
		settings.ShowWPM = !settings.ShowWPM
	case settingSkipWord:
		settings.SkipWord = !settings.SkipWord
	case settingAllowBackspace:
		settings.AllowBackspace = !settings.AllowBackspace
	case settingCursorStyle:
		settings.BlockCursor = !settings.BlockCursor
	case settingTypedTextWeight:
		settings.BoldTypedText = !settings.BoldTypedText
	case settingWordHighlighting:
		switch settings.Highlight {
		case highlightCurrentAndNext:
			settings.Highlight = highlightCurrentOnly
		case highlightCurrentOnly:
			settings.Highlight = highlightNextOnly
		case highlightNextOnly:
			settings.Highlight = highlightOff
		default:
			settings.Highlight = highlightCurrentAndNext
		}
	}
}

func mergeDirtySettings(base, draft runtimeSettings, dirty map[int]bool) runtimeSettings {
	if dirty[settingShowWPM] {
		base.ShowWPM = draft.ShowWPM
	}
	if dirty[settingSkipWord] {
		base.SkipWord = draft.SkipWord
	}
	if dirty[settingAllowBackspace] {
		base.AllowBackspace = draft.AllowBackspace
	}
	if dirty[settingCursorStyle] {
		base.BlockCursor = draft.BlockCursor
	}
	if dirty[settingTypedTextWeight] {
		base.BoldTypedText = draft.BoldTypedText
	}
	if dirty[settingWordHighlighting] {
		base.Highlight = draft.Highlight
	}
	return base
}

func drawSettings(screen tcell.Screen, selected int, draft runtimeSettings, overrides settingsOverrides, flags flagValues, message string) {
	const width, height = 52, 14
	screen.Clear()
	screen.HideCursor()
	screen.SetStyle(tcell.StyleDefault)

	screenWidth, screenHeight := screen.Size()
	if screenWidth < width || screenHeight < height {
		drawStringAtCenter(screen, "Terminal too small for settings (need 52x14)", tcell.StyleDefault)
		screen.Show()
		return
	}

	x := (screenWidth - width) / 2
	y := (screenHeight - height) / 2
	for column := 0; column < width; column++ {
		screen.SetContent(x+column, y, '-', nil, tcell.StyleDefault)
		screen.SetContent(x+column, y+height-1, '-', nil, tcell.StyleDefault)
	}
	for line := 1; line < height-1; line++ {
		screen.SetContent(x, y+line, '|', nil, tcell.StyleDefault)
		screen.SetContent(x+width-1, y+line, '|', nil, tcell.StyleDefault)
	}

	drawString(screen, x+(width-len("Settings"))/2, y+1, "Settings", -1, tcell.StyleDefault.Bold(true))
	active := effectiveRuntimeSettings(draft, overrides, flags)
	for row, label := range settingsLabels {
		marker := " "
		if row == selected {
			marker = ">"
		}
		line := fmt.Sprintf("%s %-20s %s", marker, label, settingValue(active, row))
		if settingIsOverridden(row, overrides) {
			line += fmt.Sprintf("  CLI override; Saved: %s", settingValue(draft, row))
		}
		drawString(screen, x+2, y+3+row, line, -1, tcell.StyleDefault)
	}

	drawString(screen, x+5, y+10, "Up/Down select · Space/Enter change", -1, tcell.StyleDefault)
	drawString(screen, x+13, y+11, "Esc/Ctrl-P save & close", -1, tcell.StyleDefault)
	if message != "" {
		runes := []rune("Error: " + message)
		if len(runes) > width-4 {
			runes = runes[:width-4]
		}
		drawString(screen, x+2, y+12, string(runes), -1, tcell.StyleDefault)
	}
	screen.Show()
}

func showSettings(screen tcell.Screen, saved *runtimeSettings, overrides settingsOverrides, flags flagValues) (committed bool, interrupted bool) {
	draft := *saved
	dirty := make(map[int]bool)
	selected := 0
	message := ""

	for {
		drawSettings(screen, selected, draft, overrides, flags, message)
		event := screen.PollEvent()
		switch event := event.(type) {
		case *tcell.EventResize:
			continue
		case *tcell.EventKey:
			switch event.Key() {
			case tcell.KeyCtrlC:
				return false, true
			case tcell.KeyCtrlP, tcell.KeyEscape:
				if len(dirty) == 0 {
					return false, false
				}

				latest, err := loadPersistedSettings(RUNTIME_SETTINGS_DB)
				if os.IsNotExist(err) {
					latest = defaultRuntimeSettings()
				} else if err != nil {
					message = err.Error()
					continue
				}
				merged := mergeDirtySettings(latest, draft, dirty)
				if err := savePersistedSettings(RUNTIME_SETTINGS_DB, merged); err != nil {
					message = err.Error()
					continue
				}
				*saved = merged
				return true, false
			}

			screenWidth, screenHeight := screen.Size()
			if screenWidth < 52 || screenHeight < 14 {
				continue
			}
			switch event.Key() {
			case tcell.KeyUp:
				selected = (selected + settingsRowCount - 1) % settingsRowCount
			case tcell.KeyDown:
				selected = (selected + 1) % settingsRowCount
			case tcell.KeyRune:
				if event.Rune() != ' ' {
					continue
				}
				advanceSetting(&draft, selected)
				dirty[selected] = true
				message = ""
			case tcell.KeyEnter:
				advanceSetting(&draft, selected)
				dirty[selected] = true
				message = ""
			}
		}
	}
}
