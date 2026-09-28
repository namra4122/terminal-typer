package main

import (
	"fmt"
	"os"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/rivo/uniseg"
)

var (
	errorTextStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff8080")).Underline(true)
	boldTypedStyle   = lipgloss.NewStyle().Bold(true)
	currentWordStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#f35815"))
	nextWordStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#a3a3a3"))
)

type testEntry struct {
	test     *Test
	promptID string
}

type testReadyMsg struct {
	test                *Test
	attemptID, promptID string
	err                 error
}
type settingsSavedMsg struct {
	settings runtimeSettings
	err      error
}
type appModel struct {
	session             *Session
	width, height       int
	settings            runtimeSettings
	saved               runtimeSettings
	draftSettings       runtimeSettings
	overrides           settingsOverrides
	flags               flagValues
	settingsOpen        bool
	selected            int
	meaningful          bool
	processedEventCount int
	textInputCount      int
	restartPending      bool
	tooSmall            bool
	timeLimit           time.Duration
	quitting            bool
	dirty               map[int]bool
	message             string
	tests               []*testEntry
	testIndex           int
	generateTest        func() *Test
	attempts            map[string]string
	generating          bool
	testError           string
	savingSettings      bool
	resultReadyAtNS     int64
	result              *SessionResult
}
type appTick time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(25*time.Millisecond, func(t time.Time) tea.Msg { return appTick(t) })
}
func (m appModel) Init() tea.Cmd { return tickCmd() }
func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case settingsSavedMsg:
		m.savingSettings = false
		if v.err != nil {
			m.message = v.err.Error()
			return m, nil
		}
		m.saved = v.settings
		m.draftSettings = v.settings
		m.settings = effectiveRuntimeSettings(v.settings, m.overrides, m.flags)
		m.session.AllowBackspace = m.settings.AllowBackspace
		m.session.SkipWord = m.settings.SkipWord
		m.dirty = nil
		m.message = ""
		m.settingsOpen = false
		_ = m.session.Apply(SessionInput{Kind: InputResume, Reason: "settings", AtNS: sessionNow()})
		return m, nil
	case testReadyMsg:
		m.generating = false
		if v.err != nil {
			m.testError = v.err.Error()
			return m, nil
		}
		m.tests = append(m.tests, &testEntry{test: v.test, promptID: v.promptID})
		m.testIndex = len(m.tests) - 1
		m.activateTest(v.attemptID)
		return m, nil
	case tea.WindowSizeMsg:
		m.width, m.height = v.Width, v.Height
		m.tooSmall = v.Width < 52 || v.Height < 14
		if m.tooSmall {
			_ = m.session.Apply(SessionInput{Kind: InputPause, Reason: "too-small", AtNS: sessionNow()})
		} else {
			_ = m.session.Apply(SessionInput{Kind: InputResume, Reason: "too-small", AtNS: sessionNow()})
		}
	case appTick:
		now := sessionNow()
		_ = m.session.Apply(SessionInput{Kind: InputTick, AtNS: now})
		if m.session.ActiveNS >= int64(2*time.Second) {
			m.meaningful = true
		}
		if m.restartPending && now >= m.session.RestartUntilNS {
			_ = m.session.Apply(SessionInput{Kind: InputResume, Reason: "restart", AtNS: now})
			m.restartPending = false
			m.session.RestartUntilNS = 0
		}
		m.expireIfNeeded(now)
		return m, tickCmd()
	case tea.KeyPressMsg:
		key := v.String()
		now := sessionNow()
		_ = m.session.Apply(SessionInput{Kind: InputTick, AtNS: now})
		m.expireIfNeeded(now)
		if m.generating || m.tooSmall || m.savingSettings {
			if key == "ctrl+c" {
				m.quitting = true
				return m, tea.Quit
			}
			if key == "ctrl+l" {
				return m, nil
			}
			return m, nil
		}
		if (m.session.State == SessionCompleted || m.session.State == SessionExpired) && key != "ctrl+c" && key != "ctrl+l" && now-m.resultReadyAtNS < int64(200*time.Millisecond) {
			return m, nil
		}
		switch key {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "ctrl+l":
			return m, nil
		case "ctrl+p":
			if !m.settingsOpen {
				m.settingsOpen = true
				m.draftSettings = m.saved
				m.message = ""
				_ = m.session.Apply(SessionInput{Kind: InputPause, Reason: "settings", AtNS: now})
				return m, nil
			}
			return m, m.closeSettings(now)
		case "d":
			if m.settingsOpen && m.message != "" {
				m.draftSettings = m.saved
				m.dirty = nil
				m.message = ""
				m.settingsOpen = false
				_ = m.session.Apply(SessionInput{Kind: InputResume, Reason: "settings", AtNS: now})
				return m, nil
			}
		case "esc", "escape":
			if m.settingsOpen {
				return m, m.closeSettings(now)
			}
			if !m.meaningful {
				if err := m.restartCurrentAttempt(); err != nil {
					m.testError = err.Error()
				}
				return m, nil
			}
			if m.restartPending {
				if now <= m.session.RestartUntilNS {
					if err := m.restartCurrentAttempt(); err != nil {
						m.testError = err.Error()
					}
				} else {
					_ = m.session.Apply(SessionInput{Kind: InputResume, Reason: "restart", AtNS: now})
				}
				m.restartPending = false
				m.session.RestartUntilNS = 0
				return m, nil
			}
			m.restartPending = true
			m.session.RestartUntilNS = now + int64(time.Second)
			_ = m.session.Apply(SessionInput{Kind: InputPause, Reason: "restart", AtNS: now})
			return m, nil
		case "left", "ctrl+h":
			if m.settingsOpen {
				return m, nil
			}
			_ = m.session.Apply(SessionInput{Kind: InputPrevious, AtNS: now})
			if m.testIndex > 0 {
				m.testIndex--
				if err := m.activateTest(""); err != nil {
					m.testError = err.Error()
				}
			}
			return m, nil
		case "right":
			if m.settingsOpen {
				return m, nil
			}
			_ = m.session.Apply(SessionInput{Kind: InputNext, AtNS: now})
			return m, m.requestNextTest()
		case "up":
			if m.settingsOpen {
				m.selected = (m.selected + settingsRowCount - 1) % settingsRowCount
			}
			return m, nil
		case "down":
			if m.settingsOpen {
				m.selected = (m.selected + 1) % settingsRowCount
			}
			return m, nil
		case "enter":
			if m.settingsOpen {
				advanceSetting(&m.draftSettings, m.selected)
				m.dirty[m.selected] = true
			} else if m.session.State == SessionCompleted || m.session.State == SessionExpired {
				return m, m.requestNextTest()
			}
			return m, nil
		case "r":
			if !m.settingsOpen && (m.session.State == SessionCompleted || m.session.State == SessionExpired) {
				if err := m.restartCurrentAttempt(); err != nil {
					m.testError = err.Error()
				}
				return m, nil
			}
		case "space":
			if m.settingsOpen {
				advanceSetting(&m.draftSettings, m.selected)
				m.dirty[m.selected] = true
			} else if m.settings.SkipWord {
				p := m.session.prompt()
				if !(m.session.Cursor > 0 && m.session.Cursor < len(p) && p[m.session.Cursor-1] == ' ' && p[m.session.Cursor] != ' ') {
					_ = m.session.Apply(SessionInput{Kind: InputSkip, AtNS: now})
				}
			} else {
				_ = m.session.Apply(SessionInput{Kind: InputText, Text: " ", AtNS: now})
				m.refreshMeaningful()
			}
			m.markResultReady(now)
			return m, nil
		case "backspace", "ctrl+backspace", "ctrl+w":
			if m.settingsOpen || !m.settings.AllowBackspace {
				return m, nil
			}
			kind := InputBackspace
			if key == "ctrl+backspace" || key == "ctrl+w" {
				kind = InputDeleteWord
			}
			_ = m.session.Apply(SessionInput{Kind: kind, AtNS: now})
			return m, nil
		}
		if m.settingsOpen || m.tooSmall || m.session.State == SessionCompleted || m.session.State == SessionExpired {
			return m, nil
		}
		text := v.Key().Text
		if text == "" {
			text = v.Key().String()
		}
		for _, r := range text {
			if unicode.IsPrint(r) {
				_ = m.session.Apply(SessionInput{Kind: InputText, Text: string(r), AtNS: now})
				m.refreshMeaningful()
			}
		}
		m.markResultReady(now)
		return m, nil
	}
	return m, nil
}

func (m *appModel) refreshMeaningful() {
	events := m.session.Events
	for i := m.processedEventCount; i < len(events); i++ {
		if events[i].Kind == InputText {
			m.textInputCount++
		}
	}
	m.processedEventCount = len(events)
	if m.textInputCount >= 5 || m.session.ActiveNS >= int64(2*time.Second) {
		m.meaningful = true
	}
}
func (m *appModel) expireIfNeeded(now int64) {
	if m.timeLimit >= 0 && m.session.State == SessionRunning && m.session.ActiveNS >= int64(m.timeLimit) {
		m.session.ActiveNS = int64(m.timeLimit)
		m.session.State = SessionExpired
		m.markResultReady(now)
	}
}

func (m *appModel) markResultReady(now int64) {
	if (m.session.State == SessionCompleted || m.session.State == SessionExpired) && m.result == nil && m.resultReadyAtNS == 0 {
		m.resultReadyAtNS = now
		result, err := FinishResult(m.session.Snapshot(), time.Now().UTC().UnixMilli())
		if err == nil {
			m.result = &result
		} else {
			m.testError = err.Error()
		}
	}
}

func appCursor(x, y int, block bool) *tea.Cursor {
	cursor := tea.NewCursor(x, y)
	if block {
		cursor.Shape = tea.CursorBlock
	} else {
		cursor.Shape = tea.CursorBar
	}
	cursor.Blink = true
	return cursor
}

func promptWordRanges(prompt []rune, cursor int) (currentStart, currentEnd, nextStart, nextEnd int) {
	currentStart, currentEnd, nextStart, nextEnd = -1, -1, -1, -1
	if len(prompt) == 0 {
		return
	}
	position := cursor
	if position >= len(prompt) {
		position = len(prompt) - 1
	}
	for position < len(prompt) && (prompt[position] == ' ' || prompt[position] == '\n') {
		position++
	}
	if position == len(prompt) {
		position--
		for position > 0 && (prompt[position] == ' ' || prompt[position] == '\n') {
			position--
		}
	}
	currentStart, currentEnd = position, position+1
	for currentStart > 0 && prompt[currentStart-1] != ' ' && prompt[currentStart-1] != '\n' {
		currentStart--
	}
	for currentEnd < len(prompt) && prompt[currentEnd] != ' ' && prompt[currentEnd] != '\n' {
		currentEnd++
	}
	nextStart = currentEnd
	for nextStart < len(prompt) && (prompt[nextStart] == ' ' || prompt[nextStart] == '\n') {
		nextStart++
	}
	nextEnd = nextStart
	for nextEnd < len(prompt) && prompt[nextEnd] != ' ' && prompt[nextEnd] != '\n' {
		nextEnd++
	}
	return
}

func promptViewport(prompt string, cursor, width, rows int) (start, end, startLine int, scrolled bool) {
	if width < 1 {
		width = 1
	}
	if rows < 1 {
		rows = 1
	}
	promptRuneCount := utf8.RuneCountInString(prompt)
	clusters := uniseg.NewGraphemes(prompt)
	line, column, scalar, cursorLine := 0, 0, 0, 0
	for clusters.Next() {
		cluster := clusters.Str()
		clusterEnd := scalar + utf8.RuneCountInString(cluster)
		cursorHere := scalar <= cursor && cursor < clusterEnd
		clusterWidth := uniseg.StringWidth(cluster)
		if cluster == "\n" {
			if cursorHere {
				cursorLine = line
			}
			line++
			column = 0
			scalar = clusterEnd
			continue
		}
		if column > 0 && column+clusterWidth > width {
			line++
			column = 0
		}
		if cursorHere {
			cursorLine = line
		}
		column += clusterWidth
		scalar = clusterEnd
	}
	if cursor >= promptRuneCount {
		cursorLine = line
	}
	totalLines := line + 1
	if totalLines <= rows {
		return 0, promptRuneCount, 0, false
	}
	visibleLines := rows - 2
	if visibleLines < 1 {
		visibleLines = 1
	}
	startLine = cursorLine - visibleLines/2
	if startLine < 0 {
		startLine = 0
	}
	if startLine+visibleLines > totalLines {
		startLine = totalLines - visibleLines
	}
	endLine := startLine + visibleLines
	start, end = -1, promptRuneCount
	clusters.Reset()
	line, column, scalar = 0, 0, 0
	for clusters.Next() {
		cluster := clusters.Str()
		clusterEnd := scalar + utf8.RuneCountInString(cluster)
		if cluster == "\n" {
			if line == startLine && start < 0 {
				start = scalar
			}
			line++
			column = 0
			if line >= endLine {
				end = clusterEnd
				break
			}
			scalar = clusterEnd
			continue
		}
		clusterWidth := uniseg.StringWidth(cluster)
		if column > 0 && column+clusterWidth > width {
			line++
			column = 0
		}
		if line == startLine && start < 0 {
			start = scalar
		}
		if line >= endLine {
			end = scalar
			break
		}
		column += clusterWidth
		scalar = clusterEnd
	}
	if start < 0 {
		start = 0
	}
	return start, end, startLine, true
}

func (m *appModel) activateTest(attemptID string) error {
	if m.testIndex < 0 || m.testIndex >= len(m.tests) {
		return fmt.Errorf("test index %d is unavailable", m.testIndex)
	}
	entry := m.tests[m.testIndex]
	if attemptID == "" {
		var err error
		attemptID, err = newSessionID()
		if err != nil {
			return err
		}
	}
	session := NewSession(entry.test, attemptID, entry.promptID)
	session.AllowBackspace = m.settings.AllowBackspace
	session.SkipWord = m.settings.SkipWord
	if previous := m.attempts[entry.promptID]; previous != "" {
		session.RetryOf = previous
	}
	if m.attempts == nil {
		m.attempts = make(map[string]string)
	}
	m.attempts[entry.promptID] = attemptID
	m.session = session
	m.timeLimit = entry.test.Config.TimeLimit
	m.meaningful = false
	m.processedEventCount = 0
	m.textInputCount = 0
	m.resultReadyAtNS = 0
	m.result = nil
	m.testError = ""
	return nil
}

func (m *appModel) restartCurrentAttempt() error {
	attemptID, err := newSessionID()
	if err != nil {
		return err
	}
	previousAttempt := m.session.AttemptID
	m.session = NewSession(m.session.Test, attemptID, m.session.PromptID)
	m.session.AllowBackspace = m.settings.AllowBackspace
	m.session.SkipWord = m.settings.SkipWord
	m.session.RetryOf = previousAttempt
	if m.attempts == nil {
		m.attempts = make(map[string]string)
	}
	m.attempts[m.session.PromptID] = attemptID
	m.meaningful = false
	m.restartPending = false
	m.resultReadyAtNS = 0
	m.result = nil
	m.processedEventCount = 0
	m.textInputCount = 0
	m.testError = ""
	return nil
}
func (m *appModel) requestNextTest() tea.Cmd {
	if m.testIndex+1 < len(m.tests) {
		m.testIndex++
		if err := m.activateTest(""); err != nil {
			m.testError = err.Error()
		}
		return nil
	}
	if m.generateTest == nil {
		m.testError = "No further test generator is available."
		return nil
	}
	m.generating = true
	generate := m.generateTest
	return func() tea.Msg {
		test := generate()
		if test == nil {
			return testReadyMsg{err: fmt.Errorf("no further tests are available")}
		}
		attemptID, err := newSessionID()
		if err != nil {
			return testReadyMsg{err: err}
		}
		promptID, err := newSessionID()
		if err != nil {
			return testReadyMsg{err: err}
		}
		return testReadyMsg{test: test, attemptID: attemptID, promptID: promptID}
	}
}
func (m appModel) View() tea.View {
	var b strings.Builder
	var nativeCursor *tea.Cursor
	if m.width == 0 {
		m.width = 80
	}
	if m.height == 0 {
		m.height = 24
	}
	if m.tooSmall {
		b.WriteString("Terminal too small — requires 52×14. Resize or Ctrl-C to quit.")
	} else if m.restartPending {
		b.WriteString("Restart this test? Press Escape again within 1 second to retry; wait to resume.")
	} else if m.testError != "" {
		fmt.Fprintf(&b, "Unable to start test: %s\n\nPress Right or Enter to retry · Ctrl-C to quit", m.testError)
	} else if m.generating {
		b.WriteString("Generating next test…")
	} else if m.settingsOpen {
		b.WriteString("Settings\n\n")
		for i, row := range settingsRows {
			marker := "  "
			if i == m.selected {
				marker = "> "
			}
			label := row.Label
			value := settingValue(m.draftSettings, i)
			if settingIsOverridden(i, m.overrides) {
				label += " (CLI override)"
				value = settingValue(effectiveRuntimeSettings(m.saved, m.overrides, m.flags), i)
			}
			fmt.Fprintf(&b, "%s%-24s %s\n", marker, label, value)
		}
		b.WriteString("\nUp/Down select · Enter change · Ctrl-P/Escape save and resume")
		if m.message != "" {
			b.WriteString("\n")
			b.WriteString(m.message)
			b.WriteString("\nd discard changes · Ctrl-P/Escape retry save")
		}
	} else if m.session.State == SessionCompleted || m.session.State == SessionExpired {
		result := m.result
		if result == nil {
			b.WriteString("Unable to calculate results · Ctrl-C: quit")
		} else {
			metrics := result.Measurements
			wpm := metricDisplay(metrics.WPM, "")
			rawWPM := metricDisplay(metrics.RawWPM, "")
			cpm := metricDisplay(metrics.CPM, "")
			accuracy := metricDisplay(metrics.Accuracy, "%")
			consistency := metricDisplay(metrics.Consistency, "%")
			label := ""
			if metrics.ActiveMS > 0 && metrics.ActiveMS < 1000 {
				label = " · short sample"
			}
			fmt.Fprintf(&b, "Results · %s%s\n\nEffective WPM  %s\nAccuracy       %s\nConsistency    %s\nRaw WPM        %s\nCPM            %s\nErrors         %d total · %d uncorrected\nTime           %.2fs active · %.2fs paused\nFinal speeds can differ from interval speeds after corrections.",
				result.Outcome, label, wpm, accuracy, consistency, rawWPM, cpm, metrics.Errors.Total, metrics.Errors.Uncorrected,
				float64(metrics.ActiveMS)/1000, float64(metrics.PauseMS)/1000)
			fmt.Fprintf(&b, "\nConfig        %s · %d words · %d groups", result.Config.Mode, result.Config.WordCount, result.Config.Groups)
			backspace, skip := "disabled", "disabled"
			if result.AllowBackspace {
				backspace = "enabled"
			}
			if result.SkipWord {
				skip = "enabled"
			}
			fmt.Fprintf(&b, "\nInput         backspace %s · skip %s", backspace, skip)
			if len(result.EligibilityReasons) > 0 {
				fmt.Fprintf(&b, "\nEligibility   %s", strings.Join(result.EligibilityReasons, "; "))
			}
			if result.Attribution != "" {
				fmt.Fprintf(&b, "\nSource         %s", result.Attribution)
			}
			if result.RetryOf != "" {
				b.WriteString("\nRetry          practice · PB-ineligible")
			}
			fmt.Fprintf(&b, "\n\nEnter: Next · r: Retry · Ctrl-C: quit")
		}
	} else {
		cfg := m.session.Test.Config
		live, _ := Measure(*m.session)
		fmt.Fprintf(&b, "Words · %d words · %d groups · %d/%d chars · total errors %d\n\n", cfg.WordCount, cfg.Groups, m.session.Cursor, len(m.session.Typed), live.Errors.Total)
		if m.settings.ShowWPM && m.session.ActiveNS >= int64(time.Second) && live.WPM != nil {
			fmt.Fprintf(&b, "WPM %s\n\n", metricDisplay(live.WPM, ""))
		}
		p := m.session.prompt()
		currentStart, currentEnd, nextStart, nextEnd := promptWordRanges(p, m.session.Cursor)
		headerRows := 2
		if m.settings.ShowWPM && m.session.ActiveNS >= int64(time.Second) && live.WPM != nil {
			headerRows += 2
		}
		promptRows := m.height - headerRows - 3
		visibleStart, visibleEnd, _, scrolled := promptViewport(m.session.promptText, m.session.Cursor, m.width, promptRows)
		promptTop := headerRows
		if scrolled && visibleStart > 0 {
			b.WriteString("…\n")
			promptTop++
		}
		clusters := uniseg.NewGraphemes(m.session.promptText)
		start, line, column := 0, 0, 0
		lastWasNewline := false
		for clusters.Next() {
			cluster := clusters.Str()
			end := start + utf8.RuneCountInString(cluster)
			if end <= visibleStart {
				start = end
				continue
			}
			if start >= visibleEnd {
				break
			}
			cursorHere := start <= m.session.Cursor && m.session.Cursor < end
			if cluster == "\n" {
				if cursorHere {
					nativeCursor = appCursor(column, promptTop+line, m.settings.BlockCursor)
				}
				b.WriteByte('\n')
				line++
				column = 0
				lastWasNewline = true
				start = end
				continue
			}
			clusterWidth := uniseg.StringWidth(cluster)
			if column > 0 && column+clusterWidth > m.width {
				b.WriteByte('\n')
				line++
				column = 0
				lastWasNewline = true
			} else {
				lastWasNewline = false
			}
			if cursorHere {
				nativeCursor = appCursor(column, promptTop+line, m.settings.BlockCursor)
			}
			var style lipgloss.Style
			hasStyle := false
			typed := false
			incorrect := false
			for i := start; i < end && i < len(m.session.Typed); i++ {
				if m.session.Typed[i] != 0 {
					typed = true
					if m.session.Typed[i] != p[i] {
						incorrect = true
					}
				}
			}
			if incorrect {
				style, hasStyle = errorTextStyle, true
			} else if typed && m.settings.BoldTypedText {
				style, hasStyle = boldTypedStyle, true
			} else {
				switch m.settings.Highlight {
				case highlightCurrentAndNext:
					if start < currentEnd && end > currentStart {
						style, hasStyle = currentWordStyle, true
					} else if start < nextEnd && end > nextStart {
						style, hasStyle = nextWordStyle, true
					}
				case highlightCurrentOnly:
					if start < currentEnd && end > currentStart {
						style, hasStyle = currentWordStyle, true
					}
				case highlightNextOnly:
					if start < nextEnd && end > nextStart {
						style, hasStyle = nextWordStyle, true
					}
				}
			}
			if hasStyle {
				b.WriteString(style.Render(cluster))
			} else {
				b.WriteString(cluster)
			}
			column += clusterWidth
			start = end
		}
		if m.session.Cursor == len(p) && m.session.Cursor >= visibleStart && m.session.Cursor <= visibleEnd {
			nativeCursor = appCursor(column, promptTop+line, m.settings.BlockCursor)
		}
		if scrolled && visibleEnd < len(p) {
			if !lastWasNewline {
				b.WriteByte('\n')
			}
			b.WriteRune('…')
		}
		fmt.Fprintf(&b, "\n\nActive %.1fs · Space skip · Ctrl-P settings · Esc restart", float64(m.session.ActiveNS)/1e9)
	}
	v := tea.NewView(lipgloss.NewStyle().Foreground(lipgloss.Color("#fafafa")).Background(lipgloss.Color("#1a1a1a")).Render(b.String()))
	v.AltScreen = true
	v.Cursor = nativeCursor
	return v
}

func (m *appModel) closeSettings(now int64) tea.Cmd {
	if m.savingSettings {
		return nil
	}
	if len(m.dirty) == 0 {
		m.settings = effectiveRuntimeSettings(m.saved, m.overrides, m.flags)
		m.draftSettings = m.saved
		m.settingsOpen = false
		_ = m.session.Apply(SessionInput{Kind: InputResume, Reason: "settings", AtNS: now})
		return nil
	}
	return m.saveSettings()
}

func (m *appModel) saveSettings() tea.Cmd {
	path := RUNTIME_SETTINGS_DB
	saved, draft := m.saved, m.draftSettings
	dirty := make(map[int]bool, len(m.dirty))
	for row, isDirty := range m.dirty {
		dirty[row] = isDirty
	}
	m.savingSettings = true
	return func() tea.Msg {
		base, err := loadPersistedSettings(path)
		if err != nil {
			if os.IsNotExist(err) {
				base = saved
			} else {
				return settingsSavedMsg{err: fmt.Errorf("cannot load %s: %w", path, err)}
			}
		}
		merged := mergeDirtySettings(base, draft, dirty)
		if err := savePersistedSettings(path, merged); err != nil {
			return settingsSavedMsg{err: fmt.Errorf("cannot save %s: %w", path, err)}
		}
		return settingsSavedMsg{settings: merged}
	}
}

func RunCharm(test *Test, saved runtimeSettings, overrides settingsOverrides, flags flagValues) ([]result, int, error) {
	attempt, err := newSessionID()
	if err != nil {
		return nil, 1, err
	}
	prompt, err := newSessionID()
	if err != nil {
		return nil, 1, err
	}
	m := appModel{
		session:       NewSession(test, attempt, prompt),
		saved:         saved,
		settings:      effectiveRuntimeSettings(saved, overrides, flags),
		draftSettings: saved,
		overrides:     overrides,
		flags:         flags,
		timeLimit:     test.Config.TimeLimit,
		tests:         []*testEntry{{test: test, promptID: prompt}},
		generateTest:  newTestGenerator(test.Config, nil),
		attempts:      map[string]string{prompt: attempt},
	}
	m.session.AllowBackspace = m.settings.AllowBackspace
	m.session.SkipWord = m.settings.SkipWord
	final, err := tea.NewProgram(m).Run()
	if err != nil {
		return nil, 1, err
	}
	if finalModel, ok := final.(appModel); ok && finalModel.quitting {
		return nil, 1, nil
	}
	return nil, 0, nil
}
