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
	settings runtimeSettings
	err      error
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
type appModel struct {
	session                     *Session
	width, height               int
	settings                    runtimeSettings
	saved                       runtimeSettings
	draftSettings               runtimeSettings
	overrides                   settingsOverrides
	flags                       flagValues
	settingsOpen                bool
	selected                    int
	meaningful                  bool
	processedEventCount         int
	textInputCount              int
	restartPending              bool
	tooSmall                    bool
	timeLimit                   time.Duration
	quitting                    bool
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
func (m appModel) Init() tea.Cmd { return tickCmd() }
func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
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
		if m.generating || m.tooSmall || m.savingSettings {
			if key == "ctrl+c" {
				return m, m.requestQuit()
			}
			if key == "ctrl+l" {
				return m, nil
			}
			return m, nil
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
				}
			} else {
				_ = m.session.Apply(SessionInput{Kind: InputText, Text: " ", AtNS: now})
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
				m.refreshMeaningful()
			}
		}
		return m, m.markResultReady(now)
	}
	return m, nil
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

func (m *appModel) markResultReady(now int64) tea.Cmd {
	if (m.session.State == SessionCompleted || m.session.State == SessionExpired) && m.result == nil && m.resultReadyAtNS == 0 {
		m.resultReadyAtNS = now
		result, err := FinishResult(m.session.Snapshot(), time.Now().UTC().UnixMilli())
		if err == nil {
			m.result = &result
			if m.practiceActive && m.practicePlan != nil {
				m.practiceComparisons = ComparePractice(*m.practicePlan, result)
			}
			if m.historyRoot != "" && m.session.Test != nil && m.session.Test.EligibleForHistory {
				m.historyRecord = ProjectHistory(result, m.session.Test.Origin, PrivacyPolicy{})
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
				return m.saveHistory()
			}
		} else {
			m.testError = err.Error()
		}
	}
	return nil
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
	m.historyRoot = HistoryRoot()
	m.session.AllowBackspace = m.settings.AllowBackspace
	m.session.SkipWord = m.settings.SkipWord
	final, err := tea.NewProgram(m).Run()
	if err != nil {
		return nil, 1, err
	}
	if finalModel, ok := final.(appModel); ok && finalModel.quitting {
		saveError := finalModel.backgroundSaveError
		if finalModel.saveState == SaveFailed {
			saveError = finalModel.saveError
		}
		if saveError != "" {
			fmt.Fprintf(os.Stderr, "not stored: %s\n", saveError)
		}
		return nil, 1, nil
	}
	return nil, 0, nil
}
