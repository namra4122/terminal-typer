package main

import (
	"path/filepath"
	"time"
)

type testMode string

const (
	wordMode   testMode = "words"
	quoteMode  testMode = "quote"
	customMode testMode = "custom"
)

type testSource string

const (
	wordSource  testSource = "words"
	quoteSource testSource = "quotes"
	stdinSource testSource = "stdin"
	fileSource  testSource = "file"
)

// TestModifiers are independent natural-language controls for future generators.
// Existing generators leave all of them disabled.
type TestModifiers struct {
	Punctuation    bool
	Numbers        bool
	Capitalization bool
}

// TestConfig is the resolved configuration for one invocation. WordCount is
// the total across groups; WordsPerGroup and Groups preserve the legacy flags.
type TestConfig struct {
	Mode           testMode
	Source         testSource
	Pack           string
	TimeLimit      time.Duration
	WordCount      int
	WordsPerGroup  int
	Groups         int
	Difficulty     string
	Modifiers      TestModifiers
	Raw            bool
	Multi          bool
	StartParagraph int
}

type testOptions struct {
	Words           string
	Quotes          string
	File            string
	StdinIsTerminal bool
	WordsPerGroup   int
	Groups          int
	TimeoutSeconds  int
	Raw             bool
	Multi           bool
	StartParagraph  int
}

func resolveTestConfig(o testOptions) TestConfig {
	cfg := TestConfig{
		WordsPerGroup:  o.WordsPerGroup,
		Groups:         o.Groups,
		WordCount:      o.WordsPerGroup * o.Groups,
		TimeLimit:      -1,
		Raw:            o.Raw,
		Multi:          o.Multi,
		StartParagraph: o.StartParagraph,
	}
	if o.TimeoutSeconds != -1 {
		cfg.TimeLimit = time.Duration(o.TimeoutSeconds) * time.Second
	}

	switch {
	case o.Words != "":
		cfg.Mode, cfg.Source, cfg.Pack = wordMode, wordSource, o.Words
	case o.Quotes != "":
		cfg.Mode, cfg.Source, cfg.Pack = quoteMode, quoteSource, o.Quotes
	case !o.StdinIsTerminal:
		cfg.Mode, cfg.Source = customMode, stdinSource
	case o.File != "":
		cfg.Mode, cfg.Source, cfg.Pack = customMode, fileSource, o.File
	default:
		cfg.Mode, cfg.Source, cfg.Pack = wordMode, wordSource, "1000en"
	}
	return cfg
}

// Test keeps the generated prompt intact. Display reflow operates on a copy.
type Test struct {
	Config             TestConfig
	SourceID           string
	Segments           []segment
	Attribution        string
	EligibleForHistory bool
	EligibleForPB      bool
}

func newTestGenerator(cfg TestConfig, stdinData []byte) func() *Test {
	var generate func() []segment
	sourceID := string(cfg.Source) + ":" + cfg.Pack
	switch cfg.Source {
	case wordSource:
		generate = generateWordTest(cfg.Pack, cfg.WordsPerGroup, cfg.Groups)
	case quoteSource:
		generate = generateQuoteTest(cfg.Pack)
	case stdinSource:
		generate = generateTestFromData(stdinData, cfg.Raw, cfg.Multi)
		sourceID = "stdin"
	case fileSource:
		generate = generateTestFromFile(cfg.Pack, cfg.StartParagraph)
		path, err := filepath.Abs(cfg.Pack)
		if err != nil {
			panic(err)
		}
		sourceID = "file:" + path
	}

	return func() *Test {
		segments := generate()
		if segments == nil {
			return nil
		}
		test := &Test{
			Config:             cfg,
			SourceID:           sourceID,
			Segments:           segments,
			EligibleForHistory: true,
			EligibleForPB:      true,
		}
		if len(segments) == 1 {
			test.Attribution = segments[0].Attribution
		}
		return test
	}
}

func displaySegments(test *Test, reflow func(string) string) []segment {
	segments := append([]segment(nil), test.Segments...)
	if !test.Config.Raw {
		for i := range segments {
			segments[i].Text = reflow(segments[i].Text)
		}
	}
	return segments
}
