package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type SessionState string
type InputKind string

const (
	SessionReady     SessionState = "ready"
	SessionRunning   SessionState = "running"
	SessionPaused    SessionState = "paused"
	SessionCompleted SessionState = "completed"
	SessionExpired   SessionState = "expired"
	InputText        InputKind    = "text"
	InputBackspace   InputKind    = "backspace"
	InputDeleteWord  InputKind    = "delete-word"
	InputSkip        InputKind    = "skip"
	InputNext        InputKind    = "next"
	InputPrevious    InputKind    = "previous"
	InputPause       InputKind    = "pause"
	InputResume      InputKind    = "resume"
	InputTick        InputKind    = "tick"
)

var ErrInvalidInput = errors.New("invalid session input")

type SessionInput struct {
	Kind   InputKind
	Text   string
	AtNS   int64
	Reason string
}
type InputEvent struct {
	Seq                       uint64
	ActiveNS                  int64
	Kind                      InputKind
	Text                      string
	CursorBefore, CursorAfter int
}
type Session struct {
	AttemptID, PromptID, RetryOf                   string
	Test                                           *Test
	AllowBackspace, SkipWord                       bool
	State                                          SessionState
	Cursor                                         int
	Typed                                          []rune
	promptText                                     string
	promptRunes                                    []rune
	Events                                         []InputEvent
	ExtraByWord                                    map[int][]rune
	StartedAtNS, LastAtNS, ActiveNS, PauseNS       int64
	PauseReasons                                   map[string]bool
	RestartUntilNS                                 int64
	liveCorrect, liveAttempts, liveCorrectAttempts int
	liveErrors, liveCorrectedErrors                int
	liveErrorSlots                                 map[int]bool
}

func newSessionID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
func NewSession(test *Test, attemptID, promptID string) *Session {
	var prompt strings.Builder
	if test != nil {
		for i, part := range test.Segments {
			if i > 0 {
				prompt.WriteByte('\n')
			}
			prompt.WriteString(part.Text)
		}
	}
	promptText := prompt.String()
	promptRunes := []rune(promptText)
	typed := make([]rune, len(promptRunes))
	return &Session{AttemptID: attemptID, PromptID: promptID, Test: test, State: SessionReady, Typed: typed, promptText: promptText, promptRunes: promptRunes, PauseReasons: map[string]bool{}, liveErrorSlots: map[int]bool{}, ExtraByWord: map[int][]rune{}}
}

func (s *Session) setTyped(slot int, value rune) {
	if slot < 0 || slot >= len(s.Typed) {
		return
	}
	if s.promptRunes[slot] == '\n' {
		s.Typed[slot] = value
		return
	}
	expected := s.promptRunes[slot]
	if s.Typed[slot] != 0 && s.Typed[slot] == expected {
		s.liveCorrect--
	}
	if value != 0 && value == expected {
		s.liveCorrect++
	}
	s.Typed[slot] = value
}

func (s *Session) correctLiveError(slot int) {
	if s.liveErrorSlots[slot] {
		delete(s.liveErrorSlots, slot)
		s.liveCorrectedErrors++
	}
}
func (s *Session) prompt() []rune {
	return s.promptRunes
}
func (s *Session) settle(at int64) error {
	if at < s.LastAtNS {
		return ErrInvalidInput
	}
	if s.State == SessionRunning {
		d := at - s.LastAtNS
		s.ActiveNS += d
	}
	if s.State == SessionPaused {
		s.PauseNS += at - s.LastAtNS
	}
	s.LastAtNS = at
	return nil
}
func (s *Session) Apply(in SessionInput) error {
	if s == nil || in.AtNS < 0 {
		return ErrInvalidInput
	}
	var textRune rune
	switch in.Kind {
	case InputText:
		if !utf8.ValidString(in.Text) || utf8.RuneCountInString(in.Text) != 1 {
			return ErrInvalidInput
		}
		textRune, _ = utf8.DecodeRuneInString(in.Text)
		if !unicode.IsPrint(textRune) {
			return ErrInvalidInput
		}
	case InputBackspace, InputDeleteWord, InputSkip, InputNext, InputPrevious, InputTick:
	case InputPause, InputResume:
		if in.Reason == "" {
			return ErrInvalidInput
		}
	default:
		return ErrInvalidInput
	}
	if err := s.settle(in.AtNS); err != nil {
		return err
	}
	before := s.Cursor
	p := s.prompt()
	if s.State == SessionCompleted || s.State == SessionExpired {
		return nil
	}
	if in.Kind == InputPause || in.Kind == InputResume {
		if in.Kind == InputPause {
			if !s.PauseReasons[in.Reason] {
				if len(s.PauseReasons) == 0 && s.State == SessionRunning {
					s.State = SessionPaused
				}
				s.PauseReasons[in.Reason] = true
			}
		} else if s.PauseReasons[in.Reason] {
			delete(s.PauseReasons, in.Reason)
			if len(s.PauseReasons) == 0 && s.State == SessionPaused {
				s.State = SessionRunning
			}
		}
		s.Events = append(s.Events, InputEvent{Seq: uint64(len(s.Events) + 1), ActiveNS: s.ActiveNS, Kind: in.Kind, Text: in.Text, CursorBefore: before, CursorAfter: s.Cursor})
		return nil
	}
	if s.State == SessionPaused || len(s.PauseReasons) > 0 {
		return nil
	}
	switch in.Kind {
	case InputText:
		if s.Cursor >= len(p) {
			return nil
		}
		if s.State == SessionReady {
			s.State = SessionRunning
			s.StartedAtNS = in.AtNS
		}
		slot := s.Cursor
		if p[slot] == ' ' && textRune != ' ' && slot > 0 && !unicode.IsSpace(p[slot-1]) {
			s.ExtraByWord[slot] = append(s.ExtraByWord[slot], textRune)
			s.liveAttempts++
			s.liveErrors++
			break
		}
		s.liveAttempts++
		s.correctLiveError(slot)
		if textRune == p[slot] {
			s.liveCorrectAttempts++
		} else {
			s.liveErrors++
			s.liveErrorSlots[slot] = true
		}
		s.setTyped(slot, textRune)
		s.Cursor++
		for s.Cursor < len(p) && p[s.Cursor] == '\n' {
			s.setTyped(s.Cursor, p[s.Cursor])
			s.Cursor++
		}
	case InputBackspace:
		if extras := s.ExtraByWord[s.Cursor]; len(extras) > 0 {
			s.ExtraByWord[s.Cursor] = extras[:len(extras)-1]
			if len(s.ExtraByWord[s.Cursor]) == 0 {
				delete(s.ExtraByWord, s.Cursor)
			}
			s.liveCorrectedErrors++
		} else if s.Cursor > 0 {
			s.Cursor--
			for s.Cursor > 0 && p[s.Cursor] == '\n' {
				s.Cursor--
			}
			s.correctLiveError(s.Cursor)
			s.setTyped(s.Cursor, 0)
		}
	case InputDeleteWord:
		if extras := s.ExtraByWord[s.Cursor]; len(extras) > 0 {
			s.liveCorrectedErrors += len(extras)
			delete(s.ExtraByWord, s.Cursor)
		}
		if s.Cursor > 0 {
			s.Cursor--
			for s.Cursor > 0 && (p[s.Cursor] == ' ' || p[s.Cursor] == '\n') {
				s.Cursor--
			}
			for s.Cursor > 0 && p[s.Cursor] != ' ' && p[s.Cursor] != '\n' {
				s.Cursor--
			}
			if p[s.Cursor] == ' ' || p[s.Cursor] == '\n' {
				s.setTyped(s.Cursor, p[s.Cursor])
			}
			for i := s.Cursor; i < before; i++ {
				s.correctLiveError(i)
				s.setTyped(i, 0)
			}
		}
	case InputSkip:
		if s.Cursor < len(p) {
			for s.Cursor < len(p) && p[s.Cursor] != ' ' && p[s.Cursor] != '\n' {
				s.correctLiveError(s.Cursor)
				s.liveErrors++
				s.liveErrorSlots[s.Cursor] = true
				s.setTyped(s.Cursor, 0)
				s.Cursor++
			}
			if s.Cursor < len(p) {
				if p[s.Cursor] == ' ' {
					s.liveAttempts++
					s.liveCorrectAttempts++
				}
				s.setTyped(s.Cursor, p[s.Cursor])
				s.Cursor++
			}
		}
	case InputNext, InputPrevious:
	case InputTick:
	case InputPause, InputResume:
	default:
		return ErrInvalidInput
	}
	if in.Kind != InputTick {
		s.Events = append(s.Events, InputEvent{Seq: uint64(len(s.Events) + 1), ActiveNS: s.ActiveNS, Kind: in.Kind, Text: in.Text, CursorBefore: before, CursorAfter: s.Cursor})
	}
	if s.Cursor >= len(p) {
		s.State = SessionCompleted
	}
	return nil
}
func (s *Session) Snapshot() Session {
	cp := *s
	cp.promptRunes = append([]rune(nil), s.promptRunes...)
	cp.Typed = append([]rune(nil), s.Typed...)
	cp.Events = append([]InputEvent(nil), s.Events...)
	cp.ExtraByWord = make(map[int][]rune, len(s.ExtraByWord))
	for word, extra := range s.ExtraByWord {
		cp.ExtraByWord[word] = append([]rune(nil), extra...)
	}
	cp.PauseReasons = make(map[string]bool, len(s.PauseReasons))
	for k, v := range s.PauseReasons {
		cp.PauseReasons[k] = v
	}
	cp.liveErrorSlots = make(map[int]bool, len(s.liveErrorSlots))
	for slot, present := range s.liveErrorSlots {
		cp.liveErrorSlots[slot] = present
	}
	if s.Test != nil {
		testCopy := *s.Test
		testCopy.Segments = append([]segment(nil), s.Test.Segments...)
		cp.Test = &testCopy
	}
	return cp
}

var monotonicStart = time.Now()

func sessionNow() int64 { return time.Since(monotonicStart).Nanoseconds() }
