package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/rivo/uniseg"
)

var (
	charmForegroundColor = lipgloss.Color("#fafafa")
	charmBackgroundColor = lipgloss.Color("#1a1a1a")
	charmViewStyle       = lipgloss.NewStyle().Foreground(charmForegroundColor).Background(charmBackgroundColor)
	errorTextStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff8080")).Underline(true)
	boldTypedStyle       = lipgloss.NewStyle().Bold(true)
	currentWordStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#f35815"))
	nextWordStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#a3a3a3"))
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
	configuration Configuration
	settings      runtimeSettings
	err           error
}
type hintsSavedMsg struct {
	hints []string
	err   error
}
type configurationStartedMsg struct {
	configuration       Configuration
	test                *Test
	attemptID, promptID string
	err                 error
}
type historySavedMsg struct {
	id  string
	err error
}
type historyLoadedMsg struct {
	requestID uint64
	practice  bool
	offset    int
	page      HistoryPage
	err       error
}
type historyExitTimeoutMsg struct{}
type practiceHistoryLoadedMsg struct {
	requestID uint64
	page      HistoryPage
	err       error
}
type practiceReadyMsg struct {
	test                *Test
	attemptID, promptID string
	err                 error
}
type practicePreflightMsg struct {
	requestID uint64
	err       error
}

// CharmLaunchOptions are invocation-only presentation and output choices.
// They are deliberately excluded from Configuration so CLI flags never become
// remembered test or appearance settings.
type CharmLaunchOptions struct {
	Theme             string
	OneShot, NoReport bool
	CSV, JSON         bool
	ReadOnly          bool
}

const (
	configureTestTab = iota
	configureContentTab
	configureTypingTab
	configureDisplayTab
	configureSoundTab
	configureDataTab
	configureHelpTab
	configureTabCount
)

var configureTabLabels = [configureTabCount]string{
	"Test", "Content", "Typing", "Display", "Sound", "Data", "Help",
}

const (
	hintConfigure = "configure"
	hintSettings  = "settings"
	hintHelp      = "help"
)

type appModel struct {
	session                     *Session
	width, height               int
	settings                    runtimeSettings
	saved                       runtimeSettings
	draftSettings               runtimeSettings
	configuration               Configuration
	draftConfiguration          Configuration
	configurationDirty          map[string]bool
	showFirstRunHints           bool
	hintSaveError               string
	overrides                   settingsOverrides
	flags                       flagValues
	launchOptions               CharmLaunchOptions
	settingsOpen                bool
	configureOpen               bool
	configureTab                int
	configureSelected           int
	configureEditing            string
	configureInput              string
	configurePreview            bool
	configureConfirm            string
	configureMessage            string
	savingConfiguration         bool
	selected                    int
	meaningful                  bool
	processedEventCount         int
	textInputCount              int
	restartPending              bool
	tooSmall                    bool
	timeLimit                   time.Duration
	quitting                    bool
	successfulExit              bool
	timedRefillCursor           int
	dirty                       map[int]bool
	message                     string
	tests                       []*testEntry
	testIndex                   int
	generateTest                func() *Test
	attempts                    map[string]string
	generating                  bool
	testError                   string
	savingSettings              bool
	resultReadyAtNS             int64
	result                      *SessionResult
	outputResults               []result
	historyRoot                 string
	historyRecord               HistoryRecord
	saveState                   SaveState
	saveError                   string
	historyOpen                 bool
	historyLoading              bool
	historyPractice             bool
	historyOffset               int
	historySelected             int
	historyPage                 HistoryPage
	historyError                string
	exitRequested               bool
	historyRequestID            uint64
	pendingHistory              map[string]HistoryRecord
	backgroundSaveError         string
	practiceReview              bool
	practiceLoading             bool
	practiceError               string
	practicePreflightLoading    bool
	practicePreflightError      string
	practiceHistorySampleCount  int
	practiceHistoryError        string
	practiceRequestID           uint64
	practiceSelected            int
	practiceDismissed           map[string]bool
	practiceAllItems            []PracticeItem
	practicePlan                *PracticePlan
	practiceActive              bool
	practiceOriginResult        *SessionResult
	practiceOriginSession       *Session
	practiceOriginTest          *Test
	practiceOriginHistoryRecord HistoryRecord
	practiceOriginSaveState     SaveState
	practiceOriginSaveError     string
	practiceOriginTestIndex     int
	practiceComparisons         []PracticeComparison
	practiceReturning           bool
}
type appTick time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(25*time.Millisecond, func(t time.Time) tea.Msg { return appTick(t) })
}
func (m appModel) Init() tea.Cmd {
	if !m.showFirstRunHints || m.launchOptions.ReadOnly {
		return tickCmd()
	}
	draft := m.configuration
	for _, id := range []string{hintConfigure, hintSettings, hintHelp} {
		discoverHint(&draft, id)
	}
	return tea.Batch(tickCmd(), func() tea.Msg {
		committed, err := commitConfiguration(RUNTIME_SETTINGS_DB, draft, []string{"discoveredHints"})
		return hintsSavedMsg{hints: committed.DiscoveredHints, err: err}
	})
}
func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case hintsSavedMsg:
		if v.err != nil {
			m.hintSaveError = fmt.Sprintf("Cannot remember hints: %v", v.err)
		} else {
			m.configuration.DiscoveredHints = v.hints
		}
		return m, nil
	case historySavedMsg:
		delete(m.pendingHistory, v.id)
		latest := v.id == m.historyRecord.ID
		if v.err != nil {
			if latest {
				m.saveState = SaveFailed
				m.saveError = v.err.Error()
			} else {
				m.backgroundSaveError = v.err.Error()
			}
		} else if latest {
			m.saveState = SaveSaved
			m.saveError = ""
		}
		if v.err == nil && len(m.pendingHistory) == 0 && m.backgroundSaveError == "save still pending" {
			m.backgroundSaveError = ""
		}
		if m.exitRequested && len(m.pendingHistory) == 0 {
			if m.backgroundSaveError == "" && m.saveState != SaveFailed {
				m.quitting = true
				return m, tea.Quit
			}
			m.exitRequested = false
		}
		return m, nil
	case historyLoadedMsg:
		if !m.historyOpen || v.requestID != m.historyRequestID {
			return m, nil
		}
		m.historyLoading = false
		m.historyPage = v.page
		m.historySelected = 0
		if v.err != nil {
			m.historyError = v.err.Error()
		} else {
			m.historyError = ""
		}
		return m, nil
	case practiceHistoryLoadedMsg:
		if !m.practiceReview || v.requestID != m.practiceRequestID {
			return m, nil
		}
		m.practiceLoading = false
		m.practiceHistoryError = ""
		m.practiceHistorySampleCount = len(v.page.Records)
		if v.err != nil {
			m.practiceHistoryError = v.err.Error()
		}
		if m.practiceOriginResult == nil {
			m.practiceError = "The originating result is unavailable."
			return m, nil
		}
		recent := v.page.Records
		if m.practiceOriginTest != nil && m.practiceOriginTest.Origin.Revision != "" {
			filtered := make([]HistoryRecord, 0, len(recent))
			for _, record := range recent {
				if record.PackRevision == m.practiceOriginTest.Origin.Revision {
					filtered = append(filtered, record)
				}
			}
			recent = filtered
		}
		plan, err := SelectPractice(*m.practiceOriginResult, recent)
		if err != nil {
			m.practicePlan = nil
			m.practiceAllItems = nil
			m.practiceError = err.Error()
			return m, nil
		}
		if m.practiceOriginTest != nil {
			plan.Origin = m.practiceOriginTest.Origin
		}
		m.practicePlan = &plan
		m.practiceAllItems = append([]PracticeItem(nil), plan.Items...)
		m.practiceDismissed = make(map[string]bool)
		m.practiceSelected = 0
		m.practiceError = ""
		m.practicePreflightError = ""
		m.practicePreflightLoading = true
		return m, m.preflightPractice()
	case practicePreflightMsg:
		if !m.practiceReview || v.requestID != m.practiceRequestID {
			return m, nil
		}
		m.practicePreflightLoading = false
		if v.err != nil {
			m.practicePreflightError = v.err.Error()
		} else {
			m.practicePreflightError = ""
		}
		return m, nil
	case practiceReadyMsg:
		m.generating = false
		if v.err != nil {
			m.practiceError = v.err.Error()
			m.practiceReview = true
			return m, nil
		}
		if v.test == nil {
			m.practiceError = "Practice did not produce a test."
			m.practiceReview = true
			return m, nil
		}
		attemptID, promptID := v.attemptID, v.promptID
		m.tests = append(m.tests, &testEntry{test: v.test, promptID: promptID})
		m.testIndex = len(m.tests) - 1
		m.practiceReview = false
		m.practiceLoading = false
		m.practiceActive = true
		m.practiceError = ""
		if err := m.activateTest(attemptID); err != nil {
			m.testError = err.Error()
		}
		return m, nil
	case historyExitTimeoutMsg:
		if len(m.pendingHistory) > 0 {
			if _, currentPending := m.pendingHistory[m.historyRecord.ID]; currentPending {
				m.saveState = SaveFailed
				m.saveError = "save still pending"
			}
			m.backgroundSaveError = "save still pending"
			m.exitRequested = false
		}
		return m, nil
	case settingsSavedMsg:
		m.savingSettings = false
		if v.err != nil {
			m.message = v.err.Error()
			return m, nil
		}
		m.configuration = v.configuration
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
	case configurationStartedMsg:
		m.savingConfiguration = false
		if v.err != nil {
			m.configureMessage = v.err.Error()
			return m, nil
		}
		m.configuration = v.configuration
		m.showFirstRunHints = false
		m.draftConfiguration = v.configuration
		m.saved = v.configuration.Settings
		m.draftSettings = v.configuration.Settings
		m.settings = effectiveRuntimeSettings(v.configuration.Settings, m.overrides, m.flags)
		m.configureOpen = false
		m.configurePreview = false
		m.configureConfirm = ""
		m.configureEditing = ""
		m.configureMessage = ""
		m.configurationDirty = nil
		m.tests = []*testEntry{{test: v.test, promptID: v.promptID}}
		m.testIndex = 0
		m.generateTest = newTestGenerator(v.test.Config, nil)
		m.attempts = make(map[string]string)
		if err := m.activateTest(v.attemptID); err != nil {
			m.testError = err.Error()
		}
		return m, nil
	case testReadyMsg:
		m.generating = false
		if v.err != nil {
			m.practiceReturning = false
			m.testError = v.err.Error()
			return m, nil
		}
		if m.practiceReturning {
			m.practiceReturning = false
			m.practiceActive = false
			m.practiceReview = false
			m.practicePlan = nil
			m.practiceOriginResult = nil
			m.practiceOriginSession = nil
			m.practiceComparisons = nil
			m.practiceOriginTest = nil
			if m.practiceOriginTestIndex >= 0 && m.practiceOriginTestIndex < len(m.tests) {
				m.tests = m.tests[:m.practiceOriginTestIndex+1]
			}
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
		saveCmd := m.expireIfNeeded(now)
		return m, tea.Batch(tickCmd(), saveCmd)
	case tea.KeyPressMsg:
		key := v.String()
		now := sessionNow()
		_ = m.session.Apply(SessionInput{Kind: InputTick, AtNS: now})
		saveCmd := m.expireIfNeeded(now)
		if saveCmd != nil {
			return m, saveCmd
		}
		if m.generating || m.tooSmall || m.savingSettings || m.savingConfiguration {
			if key == "ctrl+c" {
				return m, m.requestQuit()
			}
			if key == "ctrl+l" {
				return m, nil
			}
			return m, nil
		}
		if m.configureOpen {
			return m, m.updateConfigure(key, now)
		}
		if m.historyOpen {
			switch key {
			case "ctrl+c":
				return m, m.requestQuit()
			case "esc", "escape":
				m.historyOpen = false
				m.historyError = ""
				return m, nil
			case "up":
				if m.historySelected > 0 {
					m.historySelected--
				}
			case "down":
				if m.historySelected+1 < len(m.historyPage.Records) {
					m.historySelected++
				}
			case "t":
				m.historyPractice = !m.historyPractice
				m.historyOffset = 0
				return m, m.loadHistory()
			case "left":
				if m.historyOffset >= 25 {
					m.historyOffset -= 25
					return m, m.loadHistory()
				}
			case "right":
				if m.historyOffset+len(m.historyPage.Records) < m.historyPage.Total {
					m.historyOffset += 25
					return m, m.loadHistory()
				}
			}
			return m, nil
		}
		if m.practiceReview {
			return m, m.updatePracticeReview(key, now)
		}
		if m.practiceActive && (m.session.State == SessionCompleted || m.session.State == SessionExpired) {
			switch key {
			case "esc", "escape":
				m.restorePracticeOrigin()
				return m, nil
			case "p", "a":
				m.practiceReview = false
				return m, m.startPracticeAttempt()
			case "n", "q":
				return m, m.returnToRegular()
			}
			return m, nil
		}
		if (m.session.State == SessionCompleted || m.session.State == SessionExpired) && key != "ctrl+c" && key != "ctrl+l" && now-m.resultReadyAtNS < int64(200*time.Millisecond) {
			return m, nil
		}
		switch key {
		case "ctrl+c":
			return m, m.requestQuit()
		case "ctrl+l":
			return m, nil
		case "ctrl+k":
			m.openConfigure(configureTestTab, now)
			return m, nil
		case "?":
			m.openConfigure(configureHelpTab, now)
			return m, nil
		case "ctrl+p":
			if !m.settingsOpen {
				m.settingsOpen = true
				m.draftSettings = m.saved
				m.dirty = make(map[int]bool)
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
				if m.launchOptions.OneShot {
					m.successfulExit = true
					return m, m.requestQuit()
				}
				return m, m.requestNextTest()
			}
			return m, nil
		case "h":
			if !m.settingsOpen && (m.session.State == SessionCompleted || m.session.State == SessionExpired) {
				m.historyOpen = true
				m.historyPractice = false
				m.historyOffset = 0
				m.historySelected = 0
				return m, m.loadHistory()
			}
		case "p":
			if !m.settingsOpen && (m.session.State == SessionCompleted || m.session.State == SessionExpired) && m.result != nil && !m.practiceActive {
				return m, m.beginPracticeReview()
			}
		case "s":
			_, stillPending := m.pendingHistory[m.historyRecord.ID]
			if !m.settingsOpen && !stillPending && (m.session.State == SessionCompleted || m.session.State == SessionExpired) && m.saveState == SaveFailed {
				m.saveState = SavePending
				m.saveError = ""
				return m, m.saveHistory()
			}
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
					m.ensureTimedBuffer()
				}
			} else {
				_ = m.session.Apply(SessionInput{Kind: InputText, Text: " ", AtNS: now})
				m.ensureTimedBuffer()
				m.refreshMeaningful()
			}
			return m, m.markResultReady(now)
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
				m.ensureTimedBuffer()
				m.refreshMeaningful()
			}
		}
		return m, m.markResultReady(now)
	}
	return m, nil
}

func configurationOrDefault(configuration Configuration, saved runtimeSettings) Configuration {
	if configuration.Version == 0 {
		configuration = DefaultConfiguration()
		if saved != (runtimeSettings{}) {
			configuration.Settings = saved
		}
	}
	if configuration.DiscoveredHints == nil {
		configuration.DiscoveredHints = []string{}
	}
	return configuration
}

func hasHint(configuration Configuration, id string) bool {
	for _, discovered := range configuration.DiscoveredHints {
		if discovered == id {
			return true
		}
	}
	return false
}

func discoverHint(configuration *Configuration, id string) bool {
	if configuration == nil || hasHint(*configuration, id) {
		return false
	}
	configuration.DiscoveredHints = append(configuration.DiscoveredHints, id)
	sort.Strings(configuration.DiscoveredHints)
	return true
}

func (m *appModel) openConfigure(tab int, now int64) {
	if m.settingsOpen {
		return
	}
	m.configuration = configurationOrDefault(m.configuration, m.saved)
	m.draftConfiguration = m.configuration
	m.configurationDirty = make(map[string]bool)
	m.configureOpen = true
	m.configureTab = tab
	m.configureSelected = 0
	m.configureEditing = ""
	m.configureInput = ""
	m.configurePreview = false
	m.configureConfirm = ""
	m.configureMessage = ""
	hint := hintConfigure
	if tab == configureHelpTab {
		hint = hintHelp
	}
	if m.showFirstRunHints {
		for _, id := range []string{hintConfigure, hintSettings, hintHelp} {
			if discoverHint(&m.draftConfiguration, id) {
				m.configurationDirty["discoveredHints"] = true
			}
		}
	}
	if discoverHint(&m.draftConfiguration, hint) {
		m.configurationDirty["discoveredHints"] = true
	}
	_ = m.session.Apply(SessionInput{Kind: InputPause, Reason: "configure", AtNS: now})
}

func configureSpecificRows(tab int) int {
	switch tab {
	case configureTestTab:
		return 3
	case configureTypingTab:
		return 2
	case configureDisplayTab:
		return 4
	case configureDataTab:
		return 3
	case configureHelpTab:
		return 1
	default:
		return 0
	}
}

func configureRowCount(tab int) int {
	return configureSpecificRows(tab) + 4
}

func (m *appModel) closeConfigure(now int64) {
	m.configureOpen = false
	m.configurePreview = false
	m.configureConfirm = ""
	m.configureEditing = ""
	m.configureInput = ""
	m.configureMessage = ""
	m.configurationDirty = nil
	m.draftConfiguration = Configuration{}
	_ = m.session.Apply(SessionInput{Kind: InputResume, Reason: "configure", AtNS: now})
}

func (m *appModel) updateConfigure(key string, now int64) tea.Cmd {
	if key == "ctrl+c" {
		return m.requestQuit()
	}
	if m.configureConfirm != "" {
		switch key {
		case "enter", "y", "Y":
			confirmation := m.configureConfirm
			m.configureConfirm = ""
			if confirmation == "reset-all" {
				m.draftConfiguration = DefaultConfiguration()
				m.configurationDirty = map[string]bool{
					"settings": true, "test": true, "appearance": true, "discoveredHints": true,
				}
			}
			return m.startConfiguration()
		case "esc", "escape", "n", "N":
			m.configureConfirm = ""
		}
		return nil
	}
	if m.configurePreview {
		if key == "esc" || key == "escape" || key == "enter" || key == "p" {
			m.configurePreview = false
		}
		return nil
	}
	if m.configureEditing != "" {
		switch key {
		case "esc", "escape":
			m.configureEditing = ""
			m.configureInput = ""
			m.configureMessage = ""
		case "backspace":
			if len(m.configureInput) > 0 {
				m.configureInput = m.configureInput[:len(m.configureInput)-1]
			}
		case "enter":
			value, err := strconv.Atoi(m.configureInput)
			if err != nil {
				m.configureMessage = "Enter a whole number."
				return nil
			}
			switch m.configureEditing {
			case "duration":
				if value < 5 || value > 3600 {
					m.configureMessage = "Duration must be between 5 and 3600 seconds."
					return nil
				}
				m.draftConfiguration.Test.DurationSeconds = value
				m.configurationDirty["test.durationSeconds"] = true
			case "count":
				if value < 1 || value > 500 {
					m.configureMessage = "Count must be between 1 and 500 words."
					return nil
				}
				m.draftConfiguration.Test.Count = value
				m.configurationDirty["test.count"] = true
			}
			m.configureEditing = ""
			m.configureInput = ""
			m.configureMessage = ""
		default:
			if len(key) == 1 && key[0] >= '0' && key[0] <= '9' {
				m.configureInput += key
			}
		}
		return nil
	}

	switch key {
	case "esc", "escape":
		m.closeConfigure(now)
	case "tab":
		m.configureTab = (m.configureTab + 1) % configureTabCount
		m.configureSelected = 0
		if m.configureTab == configureHelpTab && discoverHint(&m.draftConfiguration, hintHelp) {
			m.configurationDirty["discoveredHints"] = true
		}
	case "shift+tab":
		m.configureTab = (m.configureTab + configureTabCount - 1) % configureTabCount
		m.configureSelected = 0
	case "up":
		count := configureRowCount(m.configureTab)
		m.configureSelected = (m.configureSelected + count - 1) % count
	case "down":
		count := configureRowCount(m.configureTab)
		m.configureSelected = (m.configureSelected + 1) % count
	case "enter", "space":
		return m.activateConfigureRow(key, now)
	case "p":
		m.configurePreview = true
	case "s":
		return m.requestConfigurationStart()
	}
	return nil
}

func cyclePreset(current int, presets []int) int {
	for i, preset := range presets {
		if current == preset {
			return presets[(i+1)%len(presets)]
		}
	}
	return presets[0]
}

func (m *appModel) markConfigurationDirty(path string) {
	if m.configurationDirty == nil {
		m.configurationDirty = make(map[string]bool)
	}
	m.configurationDirty[path] = true
	m.configureMessage = ""
}

func (m *appModel) activateConfigureRow(key string, now int64) tea.Cmd {
	specific := configureSpecificRows(m.configureTab)
	if m.configureSelected >= specific {
		switch m.configureSelected - specific {
		case 0:
			m.resetConfigureSection()
		case 1:
			m.configurePreview = true
		case 2:
			return m.requestConfigurationStart()
		case 3:
			m.closeConfigure(now)
		}
		return nil
	}

	switch m.configureTab {
	case configureTestTab:
		switch m.configureSelected {
		case 0:
			switch m.draftConfiguration.Test.Mode {
			case "timed":
				m.draftConfiguration.Test.Mode = "count"
				m.draftConfiguration.Test.Pack = "1000en"
			case "count":
				m.draftConfiguration.Test.Mode = "quote"
				m.draftConfiguration.Test.Pack = "en"
			default:
				m.draftConfiguration.Test.Mode = "timed"
				m.draftConfiguration.Test.Pack = "1000en"
			}
			m.markConfigurationDirty("test")
		case 1:
			if key == "enter" {
				m.configureEditing = "duration"
				m.configureInput = ""
				m.configureMessage = "Type 5–3600 seconds, then press Enter."
			} else {
				m.draftConfiguration.Test.DurationSeconds = cyclePreset(
					m.draftConfiguration.Test.DurationSeconds, []int{15, 30, 60, 120},
				)
				m.markConfigurationDirty("test.durationSeconds")
			}
		case 2:
			if key == "enter" {
				m.configureEditing = "count"
				m.configureInput = ""
				m.configureMessage = "Type 1–500 words, then press Enter."
			} else {
				m.draftConfiguration.Test.Count = cyclePreset(
					m.draftConfiguration.Test.Count, []int{10, 25, 50, 100},
				)
				m.markConfigurationDirty("test.count")
			}
		}
	case configureTypingTab:
		if m.configureSelected == 0 {
			m.draftConfiguration.Settings.SkipWord = !m.draftConfiguration.Settings.SkipWord
			m.markConfigurationDirty("settings.skipWord")
		} else {
			m.draftConfiguration.Settings.AllowBackspace = !m.draftConfiguration.Settings.AllowBackspace
			m.markConfigurationDirty("settings.allowBackspace")
		}
	case configureDisplayTab:
		switch m.configureSelected {
		case 0:
			m.draftConfiguration.Settings.ShowWPM = !m.draftConfiguration.Settings.ShowWPM
			m.markConfigurationDirty("settings.showWPM")
		case 1:
			m.draftConfiguration.Settings.BlockCursor = !m.draftConfiguration.Settings.BlockCursor
			m.markConfigurationDirty("settings.blockCursor")
		case 2:
			m.draftConfiguration.Settings.BoldTypedText = !m.draftConfiguration.Settings.BoldTypedText
			m.markConfigurationDirty("settings.boldTypedText")
		case 3:
			advanceSetting(&m.draftConfiguration.Settings, settingWordHighlighting)
			m.markConfigurationDirty("settings.highlight")
		}
	case configureDataTab:
		switch m.configureSelected {
		case 0:
			m.draftConfiguration.Test = DefaultConfiguration().Test
			m.markConfigurationDirty("test")
		case 1:
			m.draftConfiguration.Appearance = DefaultConfiguration().Appearance
			m.markConfigurationDirty("appearance")
		case 2:
			m.configureConfirm = "reset-all"
		}
	case configureHelpTab:
		for _, id := range []string{hintConfigure, hintSettings, hintHelp} {
			discoverHint(&m.draftConfiguration, id)
		}
		m.markConfigurationDirty("discoveredHints")
	}
	return nil
}

func (m *appModel) resetConfigureSection() {
	defaults := DefaultConfiguration()
	switch m.configureTab {
	case configureTestTab, configureContentTab:
		m.draftConfiguration.Test = defaults.Test
		m.markConfigurationDirty("test")
	case configureTypingTab:
		m.draftConfiguration.Settings.SkipWord = defaults.Settings.SkipWord
		m.draftConfiguration.Settings.AllowBackspace = defaults.Settings.AllowBackspace
		m.markConfigurationDirty("settings.skipWord")
		m.markConfigurationDirty("settings.allowBackspace")
	case configureDisplayTab:
		m.draftConfiguration.Settings.ShowWPM = defaults.Settings.ShowWPM
		m.draftConfiguration.Settings.BlockCursor = defaults.Settings.BlockCursor
		m.draftConfiguration.Settings.BoldTypedText = defaults.Settings.BoldTypedText
		m.draftConfiguration.Settings.Highlight = defaults.Settings.Highlight
		m.draftConfiguration.Appearance.Theme = defaults.Appearance.Theme
		m.draftConfiguration.Appearance.Focus = defaults.Appearance.Focus
		m.draftConfiguration.Appearance.ShowErrors = defaults.Appearance.ShowErrors
		m.draftConfiguration.Appearance.ReducedMotion = defaults.Appearance.ReducedMotion
		for _, path := range []string{
			"settings.showWPM", "settings.blockCursor", "settings.boldTypedText",
			"settings.highlight", "appearance.theme", "appearance.focus",
			"appearance.showErrors", "appearance.reducedMotion",
		} {
			m.markConfigurationDirty(path)
		}
	case configureSoundTab:
		m.draftConfiguration.Appearance.KeySound = defaults.Appearance.KeySound
		m.draftConfiguration.Appearance.ErrorSound = defaults.Appearance.ErrorSound
		m.draftConfiguration.Appearance.CompletionSound = defaults.Appearance.CompletionSound
		m.draftConfiguration.Appearance.PBSound = defaults.Appearance.PBSound
		for _, path := range []string{
			"appearance.keySound", "appearance.errorSound",
			"appearance.completionSound", "appearance.pbSound",
		} {
			m.markConfigurationDirty(path)
		}
	case configureDataTab, configureHelpTab:
		m.draftConfiguration.DiscoveredHints = append([]string(nil), defaults.DiscoveredHints...)
		m.markConfigurationDirty("discoveredHints")
	}
	m.configureMessage = "Section reset in the draft. Start to save it."
}

func (m *appModel) requestConfigurationStart() tea.Cmd {
	if err := ValidateSavedTest(m.draftConfiguration.Test); err != nil {
		m.configureMessage = err.Error()
		return nil
	}
	if m.meaningful {
		m.configureConfirm = "start"
		return nil
	}
	return m.startConfiguration()
}

func (m *appModel) startConfiguration() tea.Cmd {
	draft := m.draftConfiguration
	if err := ValidateSavedTest(draft.Test); err != nil {
		m.configureMessage = err.Error()
		return nil
	}
	m.markConfigurationDirty("test")
	dirty := make([]string, 0, len(m.configurationDirty))
	for path, changed := range m.configurationDirty {
		if changed {
			dirty = append(dirty, path)
		}
	}
	sort.Strings(dirty)
	m.savingConfiguration = true
	m.configureMessage = ""
	return func() tea.Msg {
		cfg, err := ResolveLaunch(draft, testOptions{StdinIsTerminal: true, TimeoutSeconds: -1}, nil)
		if err != nil {
			return configurationStartedMsg{err: fmt.Errorf("validate test: %w", err)}
		}
		generate, err := prepareTestGenerator(cfg, nil)
		if err != nil {
			return configurationStartedMsg{err: fmt.Errorf("load test content: %w", err)}
		}
		test := generate()
		if test == nil {
			return configurationStartedMsg{err: fmt.Errorf("generate test content: no content available")}
		}
		attemptID, err := newSessionID()
		if err != nil {
			return configurationStartedMsg{err: err}
		}
		promptID, err := newSessionID()
		if err != nil {
			return configurationStartedMsg{err: err}
		}
		committed, err := commitConfiguration(RUNTIME_SETTINGS_DB, draft, dirty)
		if err != nil {
			return configurationStartedMsg{err: fmt.Errorf("save configuration: %w", err)}
		}
		return configurationStartedMsg{
			configuration: committed, test: test, attemptID: attemptID, promptID: promptID,
		}
	}
}

func timedRefillFrontier(prompt []rune, start, wordsToVisit int) int {
	if start < 0 {
		start = 0
	}
	if wordsToVisit <= 0 {
		return start
	}
	inWord := false
	completed := 0
	for index := start; index < len(prompt); index++ {
		if unicode.IsSpace(prompt[index]) {
			if inWord {
				completed++
				if completed == wordsToVisit {
					return index + 1
				}
				inWord = false
			}
		} else {
			inWord = true
		}
	}
	return len(prompt)
}

func (m *appModel) resetTimedRefillFrontier() {
	m.timedRefillCursor = 0
	if m.session == nil || m.session.Test == nil || m.session.Test.Config.Mode != timedMode {
		return
	}
	m.timedRefillCursor = timedRefillFrontier(
		m.session.prompt(), 0, m.session.Test.Config.WordCount-49,
	)
}

func (m *appModel) ensureTimedBuffer() {
	if m.session == nil || m.session.Test == nil || m.session.Test.Config.Mode != timedMode {
		return
	}
	if m.timedRefillCursor == 0 {
		m.resetTimedRefillFrontier()
	}
	if m.session.Cursor < m.timedRefillCursor {
		return
	}
	previousFrontier := m.timedRefillCursor
	addition := m.session.Test.appendTimedWords(100)
	if addition == "" {
		return
	}
	addedRunes := []rune(" " + addition)
	m.session.promptText = m.session.Test.Segments[0].Text
	m.session.promptRunes = append(m.session.promptRunes, addedRunes...)
	m.session.Typed = append(m.session.Typed, make([]rune, len(addedRunes))...)
	m.timedRefillCursor = timedRefillFrontier(m.session.prompt(), previousFrontier, 100)
	if m.session.State == SessionCompleted {
		m.session.State = SessionRunning
	}
}

func (m *appModel) beginPracticeReview() tea.Cmd {
	m.practiceOriginResult = m.result
	m.practiceOriginSession = m.session
	if m.session != nil {
		m.practiceOriginTest = m.session.Test
	}
	m.practiceOriginHistoryRecord = m.historyRecord
	m.practiceOriginSaveState = m.saveState
	m.practiceOriginSaveError = m.saveError
	m.practiceOriginTestIndex = m.testIndex
	m.practiceReview = true
	m.practiceLoading = false
	m.practicePreflightLoading = false
	m.practicePreflightError = ""
	m.practiceHistorySampleCount = 0
	m.practiceError = ""
	m.practiceHistoryError = ""
	m.practicePlan = nil
	m.practiceAllItems = nil
	m.practiceSelected = 0
	m.practiceDismissed = make(map[string]bool)
	if m.session == nil || m.session.Test == nil ||
		!m.session.Test.Origin.Embedded ||
		(m.session.Test.Origin.Kind != "embedded-word" && m.session.Test.Origin.Kind != "embedded-quote") {
		m.practiceError = ErrInsufficientEvidence.Error()
		return nil
	}
	m.practiceLoading = true
	m.practiceRequestID++
	requestID := m.practiceRequestID
	root := m.historyRoot
	origin := m.session.Test.Origin
	mode := string(m.result.Config.Mode)
	return func() tea.Msg {
		page, err := ReadHistory(root, HistoryQuery{
			Practice: false, Limit: 20, PackID: origin.PackID,
			PackRevision: origin.Revision, Mode: mode,
		})
		return practiceHistoryLoadedMsg{requestID: requestID, page: page, err: err}
	}
}

func (m *appModel) updatePracticeReview(key string, now int64) tea.Cmd {
	switch key {
	case "ctrl+c":
		return m.requestQuit()
	case "esc", "escape":
		m.practiceReview = false
		m.practiceLoading = false
		m.practicePreflightLoading = false
		m.practicePreflightError = ""
		m.practiceHistorySampleCount = 0
		m.practicePlan = nil
		m.practiceError = ""
		m.practiceHistoryError = ""
		return nil
	case "up":
		if m.practiceSelected > 0 {
			m.practiceSelected--
		}
	case "down":
		if m.practicePlan != nil && m.practiceSelected+1 < len(m.practicePlan.Items) {
			m.practiceSelected++
		}
	case "delete", "backspace":
		if m.practicePlan != nil && len(m.practicePlan.Items) > 0 && m.practiceSelected < len(m.practicePlan.Items) {
			item := m.practicePlan.Items[m.practiceSelected]
			m.practiceDismissed[item.Item] = true
			m.practicePlan.Items = append(m.practicePlan.Items[:m.practiceSelected], m.practicePlan.Items[m.practiceSelected+1:]...)
			if m.practiceSelected >= len(m.practicePlan.Items) {
				m.practiceSelected = len(m.practicePlan.Items) - 1
			}
		}
	case "r":
		if m.practicePlan != nil {
			for _, item := range m.practiceAllItems {
				if !m.practiceDismissed[item.Item] {
					continue
				}
				delete(m.practiceDismissed, item.Item)
				m.practicePlan.Items = m.practicePlan.Items[:0]
				for _, ranked := range m.practiceAllItems {
					if !m.practiceDismissed[ranked.Item] {
						m.practicePlan.Items = append(m.practicePlan.Items, ranked)
					}
				}
				for i, active := range m.practicePlan.Items {
					if active.Item == item.Item {
						m.practiceSelected = i
						break
					}
				}
				break
			}
		}
	case "enter":
		if m.practicePreflightLoading {
			return nil
		}
		if m.practicePreflightError != "" {
			m.practiceError = m.practicePreflightError
			return nil
		}
		if m.practicePlan != nil && len(m.practicePlan.Items) > 0 {
			return m.startPracticeAttempt()
		}
	}
	_ = now
	return nil
}

func (m *appModel) preflightPractice() tea.Cmd {
	if m.practicePlan == nil {
		return nil
	}
	plan := *m.practicePlan
	plan.Items = append([]PracticeItem(nil), m.practicePlan.Items...)
	origin := m.practiceOriginTest
	neutralPack := "1000en"
	expectedRevision := ""
	if origin != nil {
		if origin.Config.Source == quoteSource {
			if origin.Config.Pack != "en" {
				return func() tea.Msg {
					return practicePreflightMsg{requestID: m.practiceRequestID, err: ErrNoNeutralPack}
				}
			}
		} else if origin.Config.Source == wordSource {
			neutralPack = origin.Config.Pack
			expectedRevision = origin.Origin.Revision
		}
		plan.Origin = origin.Origin
	}
	requestID := m.practiceRequestID
	return func() tea.Msg {
		data, resolved, err := ResolveResource("words", neutralPack)
		if err != nil || !resolved.Embedded || resolved.PackID != neutralPack ||
			(expectedRevision != "" && resolved.Revision != expectedRevision) {
			if err == nil {
				err = ErrNoNeutralPack
			}
			return practicePreflightMsg{requestID: requestID, err: fmt.Errorf("%w: %v", ErrNoNeutralPack, err)}
		}
		_, err = BuildPractice(plan, strings.Fields(string(data)), 0)
		return practicePreflightMsg{requestID: requestID, err: err}
	}
}

func (m *appModel) startPracticeAttempt() tea.Cmd {
	if m.practicePlan == nil || len(m.practicePlan.Items) == 0 {
		m.practiceError = ErrInsufficientEvidence.Error()
		m.practiceReview = true
		return nil
	}
	plan := *m.practicePlan
	plan.Items = append([]PracticeItem(nil), m.practicePlan.Items...)
	neutralPack := "1000en"
	if m.practiceOriginTest != nil && m.practiceOriginTest.Config.Source == wordSource {
		neutralPack = m.practiceOriginTest.Config.Pack
	}
	if m.practiceOriginTest == nil && m.practiceOriginSession != nil {
		m.practiceOriginTest = m.practiceOriginSession.Test
	}
	origin := m.practiceOriginTest
	if origin == nil && m.practiceOriginSession != nil {
		origin = m.practiceOriginSession.Test
	}
	if origin != nil && origin.Config.Source == wordSource {
		neutralPack = origin.Config.Pack
	}
	expectedRevision := ""
	if origin != nil && origin.Config.Source == wordSource {
		expectedRevision = origin.Origin.Revision
	}
	if origin != nil && origin.Config.Source == quoteSource && origin.Config.Pack != "en" {
		m.practiceError = ErrNoNeutralPack.Error()
		m.practiceReview = true
		return nil
	}
	m.generating = true
	m.practiceReview = false
	return func() tea.Msg {
		data, resolved, err := ResolveResource("words", neutralPack)
		if err != nil || !resolved.Embedded || resolved.PackID != neutralPack || (expectedRevision != "" && resolved.Revision != expectedRevision) {
			if err == nil {
				err = ErrNoNeutralPack
			}
			return practiceReadyMsg{err: fmt.Errorf("%w: %v", ErrNoNeutralPack, err)}
		}
		if len(data) == 0 {
			return practiceReadyMsg{err: ErrNoNeutralPack}
		}
		test, err := BuildPractice(plan, strings.Fields(string(data)), time.Now().UnixNano())
		if err != nil {
			return practiceReadyMsg{err: err}
		}
		attemptID, err := newSessionID()
		if err != nil {
			return practiceReadyMsg{err: err}
		}
		promptID, err := newSessionID()
		if err != nil {
			return practiceReadyMsg{err: err}
		}
		return practiceReadyMsg{test: test, attemptID: attemptID, promptID: promptID}
	}
}

func (m *appModel) restorePracticeOrigin() {
	if m.practiceOriginSession != nil {
		m.session = m.practiceOriginSession
	}
	if m.practiceOriginResult != nil {
		m.result = m.practiceOriginResult
	}
	if m.practiceOriginTestIndex >= 0 && m.practiceOriginTestIndex < len(m.tests) {
		m.tests = m.tests[:m.practiceOriginTestIndex+1]
		m.testIndex = m.practiceOriginTestIndex
	}
	if m.practiceOriginResult != nil {
		m.historyRecord = m.practiceOriginHistoryRecord
		m.saveState = m.practiceOriginSaveState
		m.saveError = m.practiceOriginSaveError
	}
	m.practiceActive = false
	m.practiceReview = false
	m.practiceLoading = false
	m.practicePlan = nil
	m.practiceOriginResult = nil
	m.practiceOriginSession = nil
	m.practiceOriginTest = nil
	m.practiceComparisons = nil
}

func (m *appModel) returnToRegular() tea.Cmd {
	origin := m.practiceOriginTest
	if origin == nil && m.practiceOriginSession != nil {
		origin = m.practiceOriginSession.Test
	}
	if origin == nil {
		m.practiceError = "The originating regular test is unavailable."
		return nil
	}
	cfg := origin.Config
	m.practiceReturning = true
	m.generating = true
	return func() tea.Msg {
		typ, name := "words", cfg.Pack
		if cfg.Source == quoteSource {
			typ = "quotes"
		}
		if _, _, err := ResolveResource(typ, name); err != nil {
			return testReadyMsg{err: fmt.Errorf("unable to restore regular test: %w", err)}
		}
		generate := newTestGenerator(cfg, nil)
		test := generate()
		if test == nil {
			return testReadyMsg{err: fmt.Errorf("unable to generate a fresh regular test")}
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

func (m *appModel) expireIfNeeded(now int64) tea.Cmd {
	if m.timeLimit >= 0 && m.session.State == SessionRunning && m.session.ActiveNS >= int64(m.timeLimit) {
		m.session.ActiveNS = int64(m.timeLimit)
		m.session.State = SessionExpired
		return m.markResultReady(now)
	}
	return nil
}

func legacyResultFromSession(sessionResult SessionResult, session Session) result {
	visited := session.Cursor
	if visited > len(session.promptRunes) {
		visited = len(session.promptRunes)
	}
	legacy := result{
		Timestamp: sessionResult.FinishedUnixMS / 1000,
		Mistakes:  extractMistypedWords(session.promptRunes[:visited], session.Typed[:visited]),
	}
	if legacy.Mistakes == nil {
		legacy.Mistakes = []mistake{}
	}
	if sessionResult.Measurements.WPM != nil {
		legacy.Wpm = int(math.Round(*sessionResult.Measurements.WPM))
	}
	if sessionResult.Measurements.CPM != nil {
		legacy.Cpm = int(math.Round(*sessionResult.Measurements.CPM))
	}
	if sessionResult.Measurements.Accuracy != nil {
		legacy.Accuracy = *sessionResult.Measurements.Accuracy
	}
	return legacy
}

func (m *appModel) markResultReady(now int64) tea.Cmd {
	if (m.session.State != SessionCompleted && m.session.State != SessionExpired) || m.result != nil || m.resultReadyAtNS != 0 {
		return nil
	}
	m.resultReadyAtNS = now
	snapshot := m.session.Snapshot()
	sessionResult, err := FinishResult(snapshot, time.Now().UTC().UnixMilli())
	if err != nil {
		m.testError = err.Error()
		return nil
	}
	m.result = &sessionResult
	m.outputResults = append(m.outputResults, legacyResultFromSession(sessionResult, snapshot))
	if m.practiceActive && m.practicePlan != nil {
		m.practiceComparisons = ComparePractice(*m.practicePlan, sessionResult)
	}

	var saveCmd tea.Cmd
	if m.historyRoot != "" && m.session.Test != nil && m.session.Test.EligibleForHistory {
		m.historyRecord = ProjectHistory(sessionResult, m.session.Test.Origin, PrivacyPolicy{})
		if m.practiceActive && m.practicePlan != nil {
			m.historyRecord.PracticeDetail = &PracticeDetail{
				ParentID:          m.practicePlan.ParentID,
				WindowStartUnixMS: m.practicePlan.WindowStartUnixMS,
				SampleSessions:    m.practicePlan.SampleSessions,
				Items:             append([]PracticeItem(nil), m.practicePlan.Items...),
				Comparisons:       append([]PracticeComparison(nil), m.practiceComparisons...),
			}
		}
		m.saveState = SavePending
		m.saveError = ""
		saveCmd = m.saveHistory()
	}
	if !m.launchOptions.NoReport {
		return saveCmd
	}
	if m.launchOptions.OneShot {
		m.successfulExit = true
		if saveCmd != nil {
			m.exitRequested = true
			return tea.Batch(saveCmd, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return historyExitTimeoutMsg{} }))
		}
		m.quitting = true
		return tea.Quit
	}
	return tea.Batch(saveCmd, m.requestNextTest())
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
	m.resetTimedRefillFrontier()
	m.meaningful = false
	m.processedEventCount = 0
	m.textInputCount = 0
	m.resultReadyAtNS = 0
	m.result = nil
	m.testError = ""
	m.historyOpen = false
	m.historyLoading = false
	m.historyPage = HistoryPage{}
	m.historyError = ""
	m.historyRecord = HistoryRecord{}
	m.saveState = ""
	m.saveError = ""
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
	m.resetTimedRefillFrontier()
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
	m.historyOpen = false
	m.historyLoading = false
	m.historyPage = HistoryPage{}
	m.historyError = ""
	m.historyRecord = HistoryRecord{}
	m.saveState = ""
	m.saveError = ""
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

func configureMarker(selected, row int) string {
	if selected == row {
		return "> "
	}
	return "  "
}

func configureSettingValue(settings runtimeSettings, row int, overrides settingsOverrides, flags flagValues) string {
	saved := settingValue(settings, row)
	if !settingIsOverridden(row, overrides) {
		return saved
	}
	active := settingValue(effectiveRuntimeSettings(settings, overrides, flags), row)
	return fmt.Sprintf("%s saved · %s active CLI", saved, active)
}

func (m appModel) renderConfigure(b *strings.Builder) {
	if m.configureConfirm == "start" {
		b.WriteString("Replace the current test?\n\n")
		b.WriteString("Your typed progress will be replaced by a fresh test. The current session is unchanged until saving succeeds.\n\n")
		b.WriteString("Enter/y confirm · Escape/n return")
		return
	}
	if m.configureConfirm == "reset-all" {
		b.WriteString("Reset all settings?\n\n")
		b.WriteString("Test, appearance, typing, display, and hint preferences will return to defaults. The current typed progress will be replaced by a fresh default test. History is not changed.\n\n")
		b.WriteString("Enter/y confirm · Escape/n return")
		return
	}
	if m.configurePreview {
		test := m.draftConfiguration.Test
		fmt.Fprintf(b, "Preview\n\nMode          %s\nPack          %s\nDuration      %ds\nWord count    %d\nDifficulty    %s\n\n",
			test.Mode, test.Pack, test.DurationSeconds, test.Count, test.Difficulty)
		b.WriteString("Preview does not save or replace the current test.\n\nEscape/Enter return")
		return
	}

	b.WriteString("Configure\n")
	for i, label := range configureTabLabels {
		if i == m.configureTab {
			fmt.Fprintf(b, " [%s]", label)
		} else {
			fmt.Fprintf(b, "  %s", label)
		}
	}
	b.WriteString("\n\n")

	row := 0
	switch m.configureTab {
	case configureTestTab:
		fmt.Fprintf(b, "%s%-22s %s\n", configureMarker(m.configureSelected, row), "Mode", m.draftConfiguration.Test.Mode)
		row++
		duration := fmt.Sprintf("%ds", m.draftConfiguration.Test.DurationSeconds)
		if m.configureEditing == "duration" {
			duration = m.configureInput + "▏"
		} else if m.draftConfiguration.Test.Mode != "timed" {
			duration += " (saved for timed)"
		}
		fmt.Fprintf(b, "%s%-22s %s\n", configureMarker(m.configureSelected, row), "Duration", duration)
		row++
		count := fmt.Sprintf("%d words", m.draftConfiguration.Test.Count)
		if m.configureEditing == "count" {
			count = m.configureInput + "▏"
		} else if m.draftConfiguration.Test.Mode != "count" {
			count += " (saved for count)"
		}
		fmt.Fprintf(b, "%s%-22s %s\n", configureMarker(m.configureSelected, row), "Word count", count)
		row++
	case configureContentTab:
		pack, description := m.draftConfiguration.Test.Pack, "English 1k"
		if m.draftConfiguration.Test.Mode == "quote" {
			pack, description = "en", "English quotes"
		}
		fmt.Fprintf(b, "  Pack                   %s · %s\n", pack, description)
		b.WriteString("  The saved embedded pack is active; private files and stdin are never remembered.\n")
	case configureTypingTab:
		fmt.Fprintf(b, "%s%-22s %s\n", configureMarker(m.configureSelected, row), "Skip word on Space", configureSettingValue(m.draftConfiguration.Settings, settingSkipWord, m.overrides, m.flags))
		row++
		fmt.Fprintf(b, "%s%-22s %s\n", configureMarker(m.configureSelected, row), "Allow Backspace", configureSettingValue(m.draftConfiguration.Settings, settingAllowBackspace, m.overrides, m.flags))
		row++
	case configureDisplayTab:
		fmt.Fprintf(b, "%s%-22s %s\n", configureMarker(m.configureSelected, row), "Show WPM", configureSettingValue(m.draftConfiguration.Settings, settingShowWPM, m.overrides, m.flags))
		row++
		fmt.Fprintf(b, "%s%-22s %s\n", configureMarker(m.configureSelected, row), "Cursor style", configureSettingValue(m.draftConfiguration.Settings, settingCursorStyle, m.overrides, m.flags))
		row++
		fmt.Fprintf(b, "%s%-22s %s\n", configureMarker(m.configureSelected, row), "Typed text weight", configureSettingValue(m.draftConfiguration.Settings, settingTypedTextWeight, m.overrides, m.flags))
		row++
		fmt.Fprintf(b, "%s%-22s %s\n", configureMarker(m.configureSelected, row), "Word highlighting", configureSettingValue(m.draftConfiguration.Settings, settingWordHighlighting, m.overrides, m.flags))
		row++
		fmt.Fprintf(b, "  Theme                  %s\n", m.draftConfiguration.Appearance.Theme)
	case configureSoundTab:
		b.WriteString("  Sound is off. No sound controls are available in this configuration screen.\n")
	case configureDataTab:
		fmt.Fprintf(b, "%sReset test configuration\n", configureMarker(m.configureSelected, row))
		row++
		fmt.Fprintf(b, "%sReset appearance\n", configureMarker(m.configureSelected, row))
		row++
		fmt.Fprintf(b, "%sReset all settings…\n", configureMarker(m.configureSelected, row))
		row++
		b.WriteString("  History is stored separately and is never removed by these resets.\n")
	case configureHelpTab:
		fmt.Fprintf(b, "%sDismiss first-run hints\n", configureMarker(m.configureSelected, row))
		row++
		b.WriteString("\n  Ctrl-K Configure · Ctrl-P live Settings · ? Help\n")
		b.WriteString("  Tab/Shift-Tab groups · Up/Down rows · Enter/Space change · Escape cancel\n")
	}

	fmt.Fprintf(b, "\n%sReset current section\n", configureMarker(m.configureSelected, row))
	row++
	fmt.Fprintf(b, "%sPreview\n", configureMarker(m.configureSelected, row))
	row++
	fmt.Fprintf(b, "%sStart\n", configureMarker(m.configureSelected, row))
	row++
	fmt.Fprintf(b, "%sCancel\n", configureMarker(m.configureSelected, row))
	b.WriteString("\nTab/Shift-Tab groups · Up/Down select · Enter/Space change")
	if m.savingConfiguration {
		b.WriteString("\nSaving configuration and preparing fresh content…")
	}
	if m.configureMessage != "" {
		fmt.Fprintf(b, "\n%s", m.configureMessage)
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
	} else if m.configureOpen {
		m.renderConfigure(&b)
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
	} else if m.practiceReview {
		b.WriteString("Practice weaknesses\n\n")
		if m.practiceLoading {
			b.WriteString("Loading recent comparable history…")
		} else if m.practicePreflightLoading {
			b.WriteString("Checking suitable neutral words and context limits…")
		} else if m.practiceError != "" || m.practicePreflightError != "" {
			errText := m.practiceError
			if errText == "" {
				errText = m.practicePreflightError
			}
			fmt.Fprintf(&b, "Practice unavailable: %s\n0 qualifying items · 1 current / %d history samples",
				errText, m.practiceHistorySampleCount)
			if m.practiceHistoryError != "" {
				fmt.Fprintf(&b, "\nHistory unavailable; current result only: %s", m.practiceHistoryError)
			}
		} else if m.practicePlan == nil || len(m.practicePlan.Items) == 0 {
			fmt.Fprintf(&b, "No qualifying weaknesses were found.\n0 qualifying items · 1 current / %d history samples", m.practiceHistorySampleCount)
		} else {
			if m.practiceHistoryError != "" {
				fmt.Fprintf(&b, "History unavailable; current result only: %s\n\n", m.practiceHistoryError)
			}
			if m.practiceOriginResult != nil {
				fmt.Fprintf(&b, "Evidence window: %s – %s · %d sessions\n\n",
					time.UnixMilli(m.practicePlan.WindowStartUnixMS).UTC().Format("2006-01-02"),
					time.UnixMilli(m.practiceOriginResult.FinishedUnixMS).UTC().Format("2006-01-02"),
					m.practicePlan.SampleSessions)
			}
			for i, item := range m.practicePlan.Items {
				marker := "  "
				if i == m.practiceSelected {
					marker = "> "
				}
				median := "unavailable"
				if item.MedianMSPerScalar != nil {
					median = fmt.Sprintf("%.1fms/scalar", *item.MedianMSPerScalar)
				}
				fmt.Fprintf(&b, "%s%s · %s · %d occurrences · %d current/%d history · %s\n",
					marker, item.Item, item.Reason, item.Occurrences, item.CurrentOccurrences, item.HistoryOccurrences, median)
				if item.Context != "" {
					fmt.Fprintf(&b, "   sample: %s\n", item.Context)
				}
			}
			fmt.Fprintf(&b, "\n%d selected · %d sample sessions · Enter start", len(m.practicePlan.Items), m.practicePlan.SampleSessions)
		}
		b.WriteString("\n\nUp/Down select · Delete dismiss · r restore · Enter start · Escape cancel")
	} else if m.practiceActive && (m.session.State == SessionCompleted || m.session.State == SessionExpired) {
		b.WriteString("Practice results\n\n")
		if len(m.practiceComparisons) == 0 {
			b.WriteString("No item comparisons are available.")
		} else {
			speedUnavailable := false
			for _, comparison := range m.practiceComparisons {
				if comparison.SpeedChangePercent == nil {
					speedUnavailable = true
				}
				fmt.Fprintf(&b, "%s · accuracy Δ %s · errors %d (%s) → %d (%s) · speed %s · samples %d/%d\n",
					comparison.Item, practiceDeltaDisplay(comparison.AccuracyDelta),
					comparison.Baseline.Errors, practiceRate(comparison.Baseline),
					comparison.Drill.Errors, practiceRate(comparison.Drill),
					practiceDeltaDisplay(comparison.SpeedChangePercent),
					comparison.Baseline.Occurrences, comparison.Drill.Occurrences)
			}
			if speedUnavailable {
				b.WriteString("Speed comparison unavailable when either side has fewer than 3 complete samples.")
			}
		}
		b.WriteString("\nPractice again: p or a · Return to regular test: n · Escape originating result")
	} else if m.historyOpen {
		mode := "Regular"
		if m.historyPractice {
			mode = "Practice"
		}
		fmt.Fprintf(&b, "History · %s · %d results\n\n", mode, m.historyPage.Total)
		if m.historyPage.Rebuilt && !m.historyLoading && m.historyError == "" {
			b.WriteString("History index refreshed.\n\n")
		}
		switch {
		case m.historyLoading:
			b.WriteString("Loading history…")
		case m.historyError != "":
			fmt.Fprintf(&b, "History unavailable: %s\n\nThe store was not changed. Verify no tt writer is active before removing a reported lock.", m.historyError)
		case len(m.historyPage.Records) == 0:
			b.WriteString("No completed tests in this history.")
		default:
			for i, record := range m.historyPage.Records {
				marker := "  "
				if i == m.historySelected {
					marker = "> "
				}
				fmt.Fprintf(&b, "%s%s  %-8s  WPM %s  Accuracy %s\n", marker,
					time.UnixMilli(record.FinishedUnixMS).UTC().Format("2006-01-02 15:04"),
					record.Mode, metricDisplay(record.Measurements.WPM, ""), metricDisplay(record.Measurements.Accuracy, "%"))
			}
			if m.historySelected >= 0 && m.historySelected < len(m.historyPage.Records) {
				selected := m.historyPage.Records[m.historySelected]
				fmt.Fprintf(&b, "\nSelected · %s · %s · %d errors · %.2fs active",
					selected.Outcome, selected.SourceLabel, selected.Measurements.Errors.Total,
					float64(selected.Measurements.ActiveMS)/1000)
			}
		}
		b.WriteString("\n\nUp/Down select · Left/Right page · t regular/practice · Escape results")
	} else if (m.session.State == SessionCompleted || m.session.State == SessionExpired) && m.launchOptions.NoReport {
		if m.launchOptions.OneShot {
			b.WriteString("Finishing test…")
		} else {
			b.WriteString("Preparing next test…")
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
			switch m.saveState {
			case SavePending:
				b.WriteString("\nStorage       Saving")
			case SaveSaved:
				b.WriteString("\nStorage       Saved")
			case SaveFailed:
				fmt.Fprintf(&b, "\nStorage       not stored: %s · s: Retry save", m.saveError)
			}
			fmt.Fprintf(&b, "\n\nEnter: Next · r: Retry · p: Review practice evidence · h: History · Ctrl-C: quit")
		}
	} else {
		cfg := m.session.Test.Config
		live, _ := Measure(*m.session)
		switch cfg.Mode {
		case timedMode:
			fmt.Fprintf(&b, "Timed · %.0fs · %d words buffered · %d/%d chars · total errors %d\n\n",
				cfg.TimeLimit.Seconds(), cfg.WordCount, m.session.Cursor, len(m.session.Typed), live.Errors.Total)
		case quoteMode:
			fmt.Fprintf(&b, "Quote · %s · %d/%d chars · total errors %d\n\n",
				cfg.Pack, m.session.Cursor, len(m.session.Typed), live.Errors.Total)
		default:
			fmt.Fprintf(&b, "Count · %d words · %d/%d chars · total errors %d\n\n",
				cfg.WordCount, m.session.Cursor, len(m.session.Typed), live.Errors.Total)
		}
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
		fmt.Fprintf(&b, "\n\nActive %.1fs · Space skip · Ctrl-K configure · Ctrl-P settings · Esc restart", float64(m.session.ActiveNS)/1e9)
		if m.showFirstRunHints || !hasHint(m.configuration, hintConfigure) || !hasHint(m.configuration, hintSettings) || !hasHint(m.configuration, hintHelp) {
			b.WriteString("\nHint: Ctrl-K Configure · Ctrl-P Settings · ? Help")
		}
		if m.hintSaveError != "" {
			fmt.Fprintf(&b, "\n%s", m.hintSaveError)
		}
	}
	v := tea.NewView(charmViewStyle.Render(b.String()))
	v.ForegroundColor = charmForegroundColor
	v.BackgroundColor = charmBackgroundColor
	v.AltScreen = true
	v.Cursor = nativeCursor
	return v
}
func practiceDeltaDisplay(value *float64) string {
	if value == nil {
		return "unavailable"
	}
	return fmt.Sprintf("%+.2f%%", *value)
}
func practiceRate(item PracticeItem) string {
	if item.Attempts <= 0 {
		return "unavailable"
	}
	return fmt.Sprintf("%.1f%%", float64(item.Errors)*100/float64(item.Attempts))
}

func (m *appModel) saveHistory() tea.Cmd {
	record, root := m.historyRecord, m.historyRoot
	if m.pendingHistory == nil {
		m.pendingHistory = make(map[string]HistoryRecord)
	}
	m.pendingHistory[record.ID] = record
	return func() tea.Msg {
		return historySavedMsg{id: record.ID, err: SaveHistory(root, record)}
	}
}

func (m *appModel) loadHistory() tea.Cmd {
	root, practice, offset := m.historyRoot, m.historyPractice, m.historyOffset
	m.historyRequestID++
	requestID := m.historyRequestID
	m.historyLoading = true
	m.historyError = ""
	return func() tea.Msg {
		page, err := ReadHistory(root, HistoryQuery{Practice: practice, Offset: offset, Limit: 25})
		return historyLoadedMsg{requestID: requestID, practice: practice, offset: offset, page: page, err: err}
	}
}

func (m *appModel) requestQuit() tea.Cmd {
	if len(m.pendingHistory) > 0 && !m.exitRequested {
		m.exitRequested = true
		return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return historyExitTimeoutMsg{} })
	}
	if len(m.pendingHistory) > 0 {
		m.backgroundSaveError = "save still pending"
	}
	m.quitting = true
	return tea.Quit
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
	draft := m.draftSettings
	dirty := make(map[int]bool, len(m.dirty))
	for row, isDirty := range m.dirty {
		dirty[row] = isDirty
	}
	baseConfiguration := configurationOrDefault(m.configuration, m.saved)
	baseConfiguration.Settings = draft
	m.savingSettings = true
	return func() tea.Msg {
		committed, err := commitConfiguration(path, baseConfiguration, dirtySettingsPaths(dirty))
		if err != nil {
			return settingsSavedMsg{err: fmt.Errorf("cannot save %s: %w", path, err)}
		}
		return settingsSavedMsg{configuration: committed, settings: committed.Settings}
	}
}

func validCharmColor(value string) bool {
	if len(value) != 7 || value[0] != '#' {
		return false
	}
	_, err := strconv.ParseUint(value[1:], 16, 24)
	return err == nil
}

func applyCharmTheme(name string) error {
	if name == "" || name == "tt-dark" {
		return nil
	}
	data, _, err := ResolveResource("themes", name)
	if err != nil {
		return fmt.Errorf("theme %q: %w", name, err)
	}
	theme := parseConfig(data)
	for _, key := range []string{"fgcol", "bgcol", "hicol", "hicol2", "errcol"} {
		if !validCharmColor(theme[key]) {
			return fmt.Errorf("theme %q has invalid %s", name, key)
		}
	}
	charmForegroundColor = lipgloss.Color(theme["fgcol"])
	charmBackgroundColor = lipgloss.Color(theme["bgcol"])
	charmViewStyle = lipgloss.NewStyle().Foreground(charmForegroundColor).Background(charmBackgroundColor)
	currentWordStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme["hicol2"]))
	nextWordStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme["hicol"]))
	errorTextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme["errcol"])).Underline(true)
	return nil
}

func emitCharmResults(output []result, options CharmLaunchOptions) error {
	if options.JSON {
		data, err := json.Marshal(output)
		if err != nil {
			return fmt.Errorf("encode JSON results: %w", err)
		}
		if _, err := fmt.Fprintln(os.Stdout, string(data)); err != nil {
			return fmt.Errorf("write JSON results: %w", err)
		}
	}
	if options.CSV {
		for _, item := range output {
			if _, err := fmt.Fprintf(os.Stdout, "test,%d,%d,%.2f,%d\n", item.Wpm, item.Cpm, item.Accuracy, item.Timestamp); err != nil {
				return fmt.Errorf("write CSV results: %w", err)
			}
			for _, itemMistake := range item.Mistakes {
				if _, err := fmt.Fprintf(os.Stdout, "mistake,%s,%s\n", itemMistake.Word, itemMistake.Typed); err != nil {
					return fmt.Errorf("write CSV mistakes: %w", err)
				}
			}
		}
	}
	return nil
}

func RunCharm(test *Test, configuration Configuration, overrides settingsOverrides, flags flagValues, options CharmLaunchOptions) ([]result, int, error) {
	if test == nil {
		return nil, 1, fmt.Errorf("cannot start an empty test")
	}
	configuration = configurationOrDefault(configuration, configuration.Settings)
	if err := applyCharmTheme(options.Theme); err != nil {
		return nil, 1, err
	}
	saved := configuration.Settings
	attempt, err := newSessionID()
	if err != nil {
		return nil, 1, err
	}
	prompt, err := newSessionID()
	if err != nil {
		return nil, 1, err
	}
	m := appModel{
		session:            NewSession(test, attempt, prompt),
		saved:              saved,
		settings:           effectiveRuntimeSettings(saved, overrides, flags),
		draftSettings:      saved,
		configuration:      configuration,
		draftConfiguration: configuration,
		showFirstRunHints:  !hasHint(configuration, hintConfigure) || !hasHint(configuration, hintSettings) || !hasHint(configuration, hintHelp),
		overrides:          overrides,
		flags:              flags,
		launchOptions:      options,
		timeLimit:          test.Config.TimeLimit,
		tests:              []*testEntry{{test: test, promptID: prompt}},
		generateTest:       newTestGenerator(test.Config, nil),
		attempts:           map[string]string{prompt: attempt},
	}
	m.historyRoot = HistoryRoot()
	m.session.AllowBackspace = m.settings.AllowBackspace
	m.session.SkipWord = m.settings.SkipWord
	m.resetTimedRefillFrontier()
	final, err := tea.NewProgram(m).Run()
	if err != nil {
		return nil, 1, err
	}
	finalModel, ok := final.(appModel)
	if !ok {
		return nil, 1, fmt.Errorf("unexpected Charm model %T", final)
	}
	if err := emitCharmResults(finalModel.outputResults, options); err != nil {
		return finalModel.outputResults, 1, err
	}
	rc := 0
	if finalModel.quitting && !finalModel.successfulExit {
		rc = 1
	}
	saveError := finalModel.backgroundSaveError
	if finalModel.saveState == SaveFailed {
		saveError = finalModel.saveError
	}
	if saveError != "" {
		fmt.Fprintf(os.Stderr, "not stored: %s\n", saveError)
	}
	return finalModel.outputResults, rc, nil
}
