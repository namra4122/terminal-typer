package main

import (
	"fmt"
	"time"
)

// legacyLaunch selects the unchanged explicit-input route.
func legacyLaunch(flags testOptions, visited map[string]bool) bool {
	legacy := !flags.StdinIsTerminal && flags.Words != "-" && flags.Quotes != "-"
	legacy = legacy || flags.File != ""
	for _, name := range []string{"n", "g", "t", "words", "quotes", "start", "raw", "multi"} {
		legacy = legacy || visited[name]
	}
	return legacy
}

// ResolveLaunch keeps test-defining legacy inputs independent of saved test
// choices. Presentation flags never select a different test.
func ResolveLaunch(saved Configuration, flags testOptions, visited map[string]bool) (TestConfig, error) {
	if legacyLaunch(flags, visited) {
		cfg := resolveTestConfig(flags)
		if cfg.Source == wordSource && flags.Words == "" && flags.Quotes == "" && flags.File == "" && flags.StdinIsTerminal {
			cfg.Pack = "1000en"
		}
		return cfg, nil
	}
	if err := ValidateSavedTest(saved.Test); err != nil {
		return TestConfig{}, fmt.Errorf("saved test: %w", err)
	}
	test := saved.Test
	cfg := TestConfig{
		Source: wordSource, Pack: test.Pack,
		Difficulty: test.Difficulty, Modifiers: test.Modifiers,
		TimeLimit: -1,
		Groups:    1,
	}
	switch test.Mode {
	case "timed":
		cfg.Mode = testMode("timed")
		cfg.TimeLimit = time.Duration(test.DurationSeconds) * time.Second
		cfg.WordCount, cfg.WordsPerGroup = 200, 200
	case "count":
		cfg.Mode = wordMode
		cfg.WordCount, cfg.WordsPerGroup = test.Count, test.Count
	case "quote":
		cfg.Mode, cfg.Source, cfg.Pack = quoteMode, quoteSource, "en"
		cfg.WordCount, cfg.WordsPerGroup = 0, 0
	default:
		return TestConfig{}, fmt.Errorf("unsupported saved mode %q", test.Mode)
	}
	return cfg, nil
}
