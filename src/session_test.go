package main

import (
	"errors"
	"reflect"
	"testing"
)

func testSession(prompt string) *Session {
	return NewSession(&Test{Segments: []segment{{Text: prompt}}}, "attempt", "prompt")
}

func TestSessionControlsDoNotStartClockAndSettingsPauseActiveTime(t *testing.T) {
	s := testSession("alpha beta")
	for _, in := range []SessionInput{
		{Kind: InputNext, AtNS: 1_000_000_000},
		{Kind: InputPrevious, AtNS: 2_000_000_000},
		{Kind: InputPause, Reason: "settings", AtNS: 3_000_000_000},
		{Kind: InputResume, Reason: "settings", AtNS: 6_000_000_000},
	} {
		if err := s.Apply(in); err != nil {
			t.Fatal(err)
		}
	}
	if s.ActiveNS != 0 || s.PauseNS != 0 || s.State != SessionReady {
		t.Fatalf("ready controls changed clock/state: active=%d pause=%d state=%s", s.ActiveNS, s.PauseNS, s.State)
	}
	if err := s.Apply(SessionInput{Kind: InputText, Text: "a", AtNS: 7_000_000_000}); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply(SessionInput{Kind: InputPause, Reason: "settings", AtNS: 8_000_000_000}); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply(SessionInput{Kind: InputResume, Reason: "settings", AtNS: 11_000_000_000}); err != nil {
		t.Fatal(err)
	}
	if s.ActiveNS != 1_000_000_000 || s.PauseNS != 3_000_000_000 {
		t.Fatalf("active=%d pause=%d; want 1s active and 3s pause", s.ActiveNS, s.PauseNS)
	}
}

func TestSessionSkipAndBackspaceRetainCanonicalPrompt(t *testing.T) {
	s := testSession("one two")
	if err := s.Apply(SessionInput{Kind: InputSkip, AtNS: 1}); err != nil {
		t.Fatal(err)
	}
	if s.Cursor != 4 || !reflect.DeepEqual(s.Typed[:4], []rune{0, 0, 0, ' '}) {
		t.Fatalf("skip cursor=%d typed=%q", s.Cursor, string(s.Typed[:4]))
	}
	if err := s.Apply(SessionInput{Kind: InputBackspace, AtNS: 2}); err != nil {
		t.Fatal(err)
	}
	if s.Cursor != 3 || s.Typed[3] != 0 {
		t.Fatalf("backspace cursor=%d typed=%q", s.Cursor, string(s.Typed))
	}
}

func TestSessionRejectsInvalidInputAndDecreasingClock(t *testing.T) {
	s := testSession("a")
	if err := s.Apply(SessionInput{Kind: InputText, Text: "ab", AtNS: 10}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("multi-scalar input error=%v", err)
	}
	if err := s.Apply(SessionInput{Kind: InputText, Text: "a", AtNS: 10}); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply(SessionInput{Kind: InputBackspace, AtNS: 9}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("decreasing clock error=%v", err)
	}
}

func TestSessionSnapshotDoesNotShareMutableState(t *testing.T) {
	s := testSession("abc")
	if err := s.Apply(SessionInput{Kind: InputText, Text: "a", AtNS: 1}); err != nil {
		t.Fatal(err)
	}
	copy := s.Snapshot()
	copy.Typed[0] = 'z'
	copy.Events[0].Text = "z"
	copy.PauseReasons["external"] = true
	copy.Test.Segments[0].Text = "changed"
	if s.Typed[0] != 'a' || s.Events[0].Text != "a" || s.PauseReasons["external"] || s.Test.Segments[0].Text != "abc" {
		t.Fatal("snapshot shares mutable state with session")
	}
}

func TestSessionPreservesGroupBoundariesInPrompt(t *testing.T) {
	s := NewSession(&Test{Segments: []segment{{Text: "one two"}, {Text: "three four"}}}, "a", "p")
	if got := string(s.prompt()); got != "one two\nthree four" {
		t.Fatalf("canonical prompt = %q", got)
	}
	if len(s.Typed) != len([]rune("one two\nthree four")) {
		t.Fatalf("typed slots = %d, want one per prompt scalar", len(s.Typed))
	}
}

func TestSessionPauseBeforeInputBlocksTypingWithoutStartingClock(t *testing.T) {
	s := testSession("alpha")
	if err := s.Apply(SessionInput{Kind: InputPause, Reason: "too-small", AtNS: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply(SessionInput{Kind: InputText, Text: "a", AtNS: 2}); err != nil {
		t.Fatal(err)
	}
	if s.Cursor != 0 || s.State != SessionReady || s.ActiveNS != 0 {
		t.Fatalf("paused ready session accepted typing: cursor=%d state=%s active=%d", s.Cursor, s.State, s.ActiveNS)
	}
	if err := s.Apply(SessionInput{Kind: InputResume, Reason: "too-small", AtNS: 3}); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply(SessionInput{Kind: InputText, Text: "a", AtNS: 4}); err != nil {
		t.Fatal(err)
	}
	if s.Cursor != 1 || s.State != SessionRunning || s.ActiveNS != 0 {
		t.Fatalf("resumed typing state: cursor=%d state=%s active=%d", s.Cursor, s.State, s.ActiveNS)
	}
}
