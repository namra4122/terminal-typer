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

// SettingKind describes the value shape a settings row presents to a renderer.
// The kind is intentionally independent from the formatter so layout code can
// choose generic affordances without knowing the runtime settings fields.
type SettingKind int

const (
	SettingBoolean SettingKind = iota
	SettingEnum
	SettingText
)

// SettingRow contains the declarative metadata needed to render one setting.
// FormatValue is evaluated against either the effective or saved settings,
// depending on which value a renderer wants to present.
type SettingRow struct {
	Label        string
	Section      string
	Kind         SettingKind
	FormatValue  func(runtimeSettings) string
	IsOverridden func(settingsOverrides) bool
}

const (
	settingsSectionGeneral    = "General"
	settingsSectionAppearance = "Appearance"
)

func formatToggle(value bool) string {
	if value {
		return "On"
	}
	return "Off"
}

func formatCursorStyle(settings runtimeSettings) string {
	if settings.BlockCursor {
		return "Block"
	}
	return "Bar"
}

func formatTypedTextWeight(settings runtimeSettings) string {
	if settings.BoldTypedText {
		return "Bold"
	}
	return "Normal"
}

func formatWordHighlighting(settings runtimeSettings) string {
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
}

var settingsRows = [settingsRowCount]SettingRow{
	{
		Label:        "Show WPM",
		Section:      settingsSectionGeneral,
		Kind:         SettingBoolean,
		FormatValue:  func(settings runtimeSettings) string { return formatToggle(settings.ShowWPM) },
		IsOverridden: func(overrides settingsOverrides) bool { return overrides.ShowWPM },
	},
	{
		Label:        "Skip word on Space",
		Section:      settingsSectionGeneral,
		Kind:         SettingBoolean,
		FormatValue:  func(settings runtimeSettings) string { return formatToggle(settings.SkipWord) },
		IsOverridden: func(overrides settingsOverrides) bool { return overrides.SkipWord },
	},
	{
		Label:        "Allow Backspace",
		Section:      settingsSectionGeneral,
		Kind:         SettingBoolean,
		FormatValue:  func(settings runtimeSettings) string { return formatToggle(settings.AllowBackspace) },
		IsOverridden: func(overrides settingsOverrides) bool { return overrides.AllowBackspace },
	},
	{
		Label:        "Cursor style",
		Section:      settingsSectionAppearance,
		Kind:         SettingEnum,
		FormatValue:  formatCursorStyle,
		IsOverridden: func(overrides settingsOverrides) bool { return overrides.BlockCursor },
	},
	{
		Label:        "Typed text weight",
		Section:      settingsSectionAppearance,
		Kind:         SettingEnum,
		FormatValue:  formatTypedTextWeight,
		IsOverridden: func(overrides settingsOverrides) bool { return overrides.BoldTypedText },
	},
	{
		Label:        "Word highlighting",
		Section:      settingsSectionAppearance,
		Kind:         SettingEnum,
		FormatValue:  formatWordHighlighting,
		IsOverridden: func(overrides settingsOverrides) bool { return overrides.Highlight },
	},
}

// settingsLabels remains as a compatibility view for callers that only need
// labels. The row descriptors above are the single source of label metadata.
var settingsLabels = func() [settingsRowCount]string {
	var labels [settingsRowCount]string
	for row, setting := range settingsRows {
		labels[row] = setting.Label
	}
	return labels
}()

func settingIsOverridden(row int, overrides settingsOverrides) bool {
	if row < 0 || row >= len(settingsRows) {
		return false
	}
	return settingsRows[row].IsOverridden(overrides)
}

func settingValue(settings runtimeSettings, row int) string {
	if row < 0 || row >= len(settingsRows) {
		return ""
	}
	return settingsRows[row].FormatValue(settings)
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

const (
	// settingsMinimumWidth and settingsMinimumHeight retain the modal's
	// established input gate. Below this size the modal remains read-only so
	// users cannot accidentally edit settings while the panel is unreadable.
	settingsMinimumWidth  = 52
	settingsMinimumHeight = 14
	settingsPanelMaxWidth = 72
	settingsPanelMaxHeight = 16
)

func settingsPanelRect(screenWidth, screenHeight int) (Rect, bool) {
	if screenWidth < settingsMinimumWidth || screenHeight < settingsMinimumHeight {
		return Rect{}, false
	}

	width := screenWidth
	if width > settingsPanelMaxWidth {
		width = settingsPanelMaxWidth
	}
	height := screenHeight
	if height > settingsPanelMaxHeight {
		height = settingsPanelMaxHeight
	}
	return Rect{
		X:      (screenWidth - width) / 2,
		Y:      (screenHeight - height) / 2,
		Width:  width,
		Height: height,
	}, true
}

func drawSettingsFallback(screen tcell.Screen) {
	screenWidth, screenHeight := screen.Size()
	message := TruncateCells("Terminal too small for settings (need 52x14)", screenWidth)
	x := (screenWidth - CellWidth(message)) / 2
	if x < 0 {
		x = 0
	}
	y := screenHeight / 2
	DrawText(screen, x, y, message, DefaultStyles.Warning)
}

func drawSettingsRow(screen tcell.Screen, content Rect, row, selected int, active runtimeSettings, overrides settingsOverrides) {
	if row < 0 || row >= len(settingsRows) || content.Width <= 0 || content.Height <= 0 {
		return
	}

	setting := settingsRows[row]
	marker := "  "
	if row == selected {
		marker = "> "
	}
	label := marker + setting.Label
	labelWidth := CellWidth(label)
	valueWidth := content.Width - labelWidth - 1
	if valueWidth < 1 {
		valueWidth = 1
		labelWidth = content.Width - 2
		if labelWidth < 1 {
			labelWidth = 1
		}
		label = TruncateCells(label, labelWidth)
	}

	value := settingValue(active, row)
	if settingIsOverridden(row, overrides) && valueWidth >= CellWidth(value)+2 {
		value += " *"
	}
	value = AlignRightCells(value, valueWidth)

	DrawTextInRect(screen, Rect{
		X:      content.X,
		Y:      content.Y + row,
		Width:  labelWidth,
		Height: 1,
	}, label, DefaultStyles.Text)
	DrawTextInRect(screen, Rect{
		X:      content.X + labelWidth + 1,
		Y:      content.Y + row,
		Width:  valueWidth,
		Height: 1,
	}, value, DefaultStyles.Value)
}

func drawSettings(screen tcell.Screen, selected int, draft runtimeSettings, overrides settingsOverrides, flags flagValues, message string) {
	screen.Clear()
	screen.HideCursor()
	screen.SetStyle(DefaultStyles.Text)

	screenWidth, screenHeight := screen.Size()
	panel, ok := settingsPanelRect(screenWidth, screenHeight)
	if !ok {
		drawSettingsFallback(screen)
		screen.Show()
		return
	}

	DrawBox(screen, panel, NormalBorder(), DefaultStyles.Border)
	inner := panel.Inset(1)
	if inner.Width <= 0 || inner.Height <= 0 {
		screen.Show()
		return
	}

	// Keep a single cell of horizontal breathing room when possible. Vertical
	// padding is deliberately compact so all settings remain available before
	// optional help is removed on short terminals.
	paddingX := 2
	if inner.Width < 40 {
		paddingX = 1
	}
	content := Rect{
		X:      inner.X + paddingX,
		Y:      inner.Y + 1,
		Width:  inner.Width - paddingX*2,
		Height: inner.Height - 2,
	}
	if content.Width <= 0 || content.Height <= 0 {
		screen.Show()
		return
	}

	DrawTextInRect(screen, Rect{X: content.X, Y: content.Y, Width: content.Width, Height: 1},
		"Settings", DefaultStyles.AppTitle)

	active := effectiveRuntimeSettings(draft, overrides, flags)
	rowsY := content.Y + 1
	rowsHeight := content.Height - 1
	if rowsHeight > settingsRowCount {
		rowsHeight = settingsRowCount
	}
	for row := 0; row < rowsHeight; row++ {
		drawSettingsRow(screen, Rect{
			X:      content.X,
			Y:      rowsY,
			Width:  content.Width,
			Height: rowsHeight,
		}, row, selected, active, overrides)
	}

	nextY := rowsY + rowsHeight
	remaining := content.Y + content.Height - nextY
	if message != "" && remaining > 0 {
		DrawTextInRect(screen, Rect{
			X:      content.X,
			Y:      nextY,
			Width:  content.Width,
			Height: 1,
		}, "Error: "+message, DefaultStyles.Error)
		nextY++
		remaining--
	}

	help := []string{"↑↓ Select · Space/Enter Change", "Esc/Ctrl-P Save & Close"}
	for _, line := range help {
		if remaining <= 0 {
			break
		}
		DrawTextInRect(screen, Rect{
			X:      content.X,
			Y:      nextY,
			Width:  content.Width,
			Height: 1,
		}, line, DefaultStyles.FooterText)
		nextY++
		remaining--
	}
	screen.Show()
}

func showSettings(screen tcell.Screen, saved *runtimeSettings, overrides settingsOverrides, flags flagValues) (committed bool, interrupted bool) {
	draft := *saved
	dirty := make(map[int]bool)
	selected := 0
	original := *saved
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
			if screenWidth < settingsMinimumWidth || screenHeight < settingsMinimumHeight {
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
				if settingValue(draft, selected) == settingValue(original, selected) {
					delete(dirty, selected)
				} else {
					dirty[selected] = true
				}
				message = ""
			case tcell.KeyEnter:
				advanceSetting(&draft, selected)
				if settingValue(draft, selected) == settingValue(original, selected) {
					delete(dirty, selected)
				} else {
					dirty[selected] = true
				}
				message = ""
			}
		}
	}
}
