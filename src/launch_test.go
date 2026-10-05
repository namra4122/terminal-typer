package main

import (
	"testing"
	"time"
)

func TestResolveLaunchIgnoresPresentationFlags(t *testing.T) {
	saved := DefaultConfiguration()
	saved.Test.Mode = "count"
	saved.Test.Count = 25
	saved.Test.Modifiers.Punctuation = true
	for _, visited := range []map[string]bool{{}, {"showwpm": true}, {"json": true}, {"theme": true, "oneshot": true, "noreport": true, "csv": true}} {
		cfg, err := ResolveLaunch(saved, testOptions{StdinIsTerminal: true, TimeoutSeconds: -1}, visited)
		if err != nil || cfg.Mode != wordMode || cfg.WordCount != 25 || !cfg.Modifiers.Punctuation || cfg.TimeLimit != -1 {
			t.Fatalf("visited=%v resolved=%+v err=%v", visited, cfg, err)
		}
	}
}

func TestResolveLaunchExplicitInputClearsSavedTimerAndModifiers(t *testing.T) {
	saved := DefaultConfiguration()
	saved.Test.Modifiers.Punctuation = true
	cases := []struct {
		name    string
		opts    testOptions
		visited map[string]bool
		mode    testMode
		source  testSource
		pack    string
		count   int
		limit   time.Duration
	}{
		{"n", testOptions{StdinIsTerminal: true, WordsPerGroup: 10, Groups: 1, TimeoutSeconds: -1}, map[string]bool{"n": true}, wordMode, wordSource, "1000en", 10, -1},
		{"g", testOptions{StdinIsTerminal: true, WordsPerGroup: 50, Groups: 2, TimeoutSeconds: -1}, map[string]bool{"g": true}, wordMode, wordSource, "1000en", 100, -1},
		{"t", testOptions{StdinIsTerminal: true, WordsPerGroup: 50, Groups: 1, TimeoutSeconds: 15}, map[string]bool{"t": true}, wordMode, wordSource, "1000en", 50, 15 * time.Second},
		{"words before quote and file", testOptions{StdinIsTerminal: true, Words: "fr", Quotes: "en", File: "private.txt", WordsPerGroup: 50, Groups: 1, TimeoutSeconds: -1}, map[string]bool{"words": true, "quotes": true}, wordMode, wordSource, "fr", 50, -1},
		{"quote", testOptions{StdinIsTerminal: true, Quotes: "en", WordsPerGroup: 50, Groups: 1, TimeoutSeconds: -1}, map[string]bool{"quotes": true}, quoteMode, quoteSource, "en", 50, -1},
		{"stdin before file", testOptions{File: "private.txt", WordsPerGroup: 50, Groups: 1, TimeoutSeconds: -1}, nil, customMode, stdinSource, "", 50, -1},
		{"start file", testOptions{StdinIsTerminal: true, File: "private.txt", StartParagraph: 0, WordsPerGroup: 50, Groups: 1, TimeoutSeconds: -1}, map[string]bool{"start": true}, customMode, fileSource, "private.txt", 50, -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := ResolveLaunch(saved, tc.opts, tc.visited)
			if err != nil || cfg.Mode != tc.mode || cfg.Source != tc.source || cfg.Pack != tc.pack || cfg.WordCount != tc.count || cfg.TimeLimit != tc.limit || cfg.Modifiers != (TestModifiers{}) {
				t.Fatalf("resolved=%+v err=%v", cfg, err)
			}
		})
	}
}

func TestResolveLaunchNewUserDefaults(t *testing.T) {
	cfg, err := ResolveLaunch(DefaultConfiguration(), testOptions{StdinIsTerminal: true, TimeoutSeconds: -1}, nil)
	if err != nil || cfg.Mode != testMode("timed") || cfg.Pack != "1000en" || cfg.TimeLimit != 30*time.Second || cfg.Modifiers != (TestModifiers{}) || cfg.Difficulty != "normal" {
		t.Fatalf("resolved=%+v err=%v", cfg, err)
	}
}
