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
	AttemptID, PromptID, RetryOf             string
	Test                                     *Test
	State                                    SessionState
	Cursor                                   int
	Typed                                    []rune
	promptText                               string
	promptRunes                              []rune
	Events                                   []InputEvent
	StartedAtNS, LastAtNS, ActiveNS, PauseNS int64
	PauseReasons                             map[string]bool
	RestartUntilNS                           int64
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
	return &Session{AttemptID: attemptID, PromptID: promptID, Test: test, State: SessionReady, Typed: typed, promptText: promptText, promptRunes: promptRunes, PauseReasons: map[string]bool{}}
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
		s.Typed[s.Cursor] = textRune
		s.Cursor++
		for s.Cursor < len(p) && p[s.Cursor] == '\n' {
			s.Typed[s.Cursor] = p[s.Cursor]
			s.Cursor++
		}
	case InputBackspace:
		if s.Cursor > 0 {
			s.Cursor--
			for s.Cursor > 0 && p[s.Cursor] == '\n' {
				s.Cursor--
			}
			s.Typed[s.Cursor] = 0
		}
	case InputDeleteWord:
		if s.Cursor > 0 {
			s.Cursor--
			for s.Cursor > 0 && (p[s.Cursor] == ' ' || p[s.Cursor] == '\n') {
				s.Cursor--
			}
			for s.Cursor > 0 && p[s.Cursor] != ' ' && p[s.Cursor] != '\n' {
				s.Cursor--
			}
			if p[s.Cursor] == ' ' || p[s.Cursor] == '\n' {
				s.Typed[s.Cursor] = p[s.Cursor]
				s.Cursor++
			}
			for i := s.Cursor; i < before; i++ {
				s.Typed[i] = 0
			}
		}
	case InputSkip:
		if s.Cursor < len(p) {
			for s.Cursor < len(p) && p[s.Cursor] != ' ' && p[s.Cursor] != '\n' {
				s.Typed[s.Cursor] = 0
				s.Cursor++
			}
			if s.Cursor < len(p) {
				s.Typed[s.Cursor] = p[s.Cursor]
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
	cp.PauseReasons = make(map[string]bool, len(s.PauseReasons))
	for k, v := range s.PauseReasons {
		cp.PauseReasons[k] = v
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
