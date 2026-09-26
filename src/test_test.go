package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestResolveTestConfigPreservesSourcePrecedence(t *testing.T) {
	cases := []struct {
		name   string
		opts   testOptions
		mode   testMode
		source testSource
		pack   string
	}{
		{"default", testOptions{StdinIsTerminal: true}, wordMode, wordSource, "1000en"},
		{"file", testOptions{StdinIsTerminal: true, File: "book.txt"}, customMode, fileSource, "book.txt"},
		{"stdin before file", testOptions{File: "book.txt"}, customMode, stdinSource, ""},
		{"quote before stdin", testOptions{Quotes: "en", File: "book.txt"}, quoteMode, quoteSource, "en"},
		{"words before quote", testOptions{Words: "custom", Quotes: "en"}, wordMode, wordSource, "custom"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveTestConfig(tc.opts)
			if got.Mode != tc.mode || got.Source != tc.source || got.Pack != tc.pack {
				t.Fatalf("source = %q/%q/%q, want %q/%q/%q", got.Mode, got.Source, got.Pack, tc.mode, tc.source, tc.pack)
			}
		})
	}
}

func TestResolveTestConfigKeepsLegacyParameters(t *testing.T) {
	got := resolveTestConfig(testOptions{
		WordsPerGroup: 10, Groups: 5, TimeoutSeconds: 30,
		Raw: true, Multi: true, StartParagraph: 3, StdinIsTerminal: true,
	})
	if got.WordCount != 50 || got.WordsPerGroup != 10 || got.Groups != 5 || got.TimeLimit != 30*time.Second ||
		!got.Raw || !got.Multi || got.StartParagraph != 3 {
		t.Fatalf("resolved options = %#v", got)
	}
	if got.Modifiers != (TestModifiers{}) || got.Difficulty != "" {
		t.Fatalf("legacy invocation unexpectedly enables future controls: %#v", got)
	}
	got = resolveTestConfig(testOptions{TimeoutSeconds: -1, StdinIsTerminal: true})
	if got.TimeLimit != -1 {
		t.Fatalf("unlimited time = %v, want -1", got.TimeLimit)
	}
}

func TestGeneratedTestKeepsMetadataAndPromptAcrossDisplayRuns(t *testing.T) {
	cfg := resolveTestConfig(testOptions{WordsPerGroup: 10, Groups: 2, TimeoutSeconds: -1, StdinIsTerminal: true})
	generated := newTestGenerator(cfg, nil)()
	if generated == nil || generated.SourceID != "words:1000en" || !reflect.DeepEqual(generated.Config, cfg) ||
		!generated.EligibleForHistory || !generated.EligibleForPB || len(generated.Segments) != 2 {
		t.Fatalf("generated test = %#v", generated)
	}
	original := append([]segment(nil), generated.Segments...)
	first := displaySegments(generated, func(s string) string { return "wrapped:" + s })
	second := displaySegments(generated, func(s string) string { return "again:" + s })
	if !reflect.DeepEqual(generated.Segments, original) || first[0].Text != "wrapped:"+original[0].Text ||
		second[0].Text != "again:"+original[0].Text {
		t.Fatalf("display runs changed source prompt: original=%#v first=%#v second=%#v", generated.Segments, first, second)
	}
}

func TestStdinMultiTestExhaustionAndRawDisplay(t *testing.T) {
	cfg := resolveTestConfig(testOptions{Multi: true, TimeoutSeconds: -1})
	next := newTestGenerator(cfg, []byte("first\nline\n\nsecond"))
	first, second, done := next(), next(), next()
	if first == nil || second == nil || done != nil || first.SourceID != "stdin" || second.SourceID != "stdin" {
		t.Fatalf("stdin sequence = %#v, %#v, %#v", first, second, done)
	}
	if first.Segments[0].Text != "first\nline" || second.Segments[0].Text != "second" {
		t.Fatalf("stdin segments = %#v, %#v", first.Segments, second.Segments)
	}
	rawCfg := resolveTestConfig(testOptions{Multi: true, Raw: true, TimeoutSeconds: -1})
	raw := newTestGenerator(rawCfg, []byte("first\nline\n\nsecond"))()
	displayed := displaySegments(raw, func(string) string { t.Fatal("raw test called reflow"); return "" })
	if displayed[0].Text != "first\nline\n\nsecond" {
		t.Fatalf("raw display = %#v", displayed)
	}
}

func TestQuoteAndFileGeneratorsCarrySourceMetadata(t *testing.T) {
	quoteConfig := resolveTestConfig(testOptions{Quotes: "en", TimeoutSeconds: -1})
	quote := newTestGenerator(quoteConfig, nil)()
	if quote == nil || quote.SourceID != "quotes:en" || len(quote.Segments) != 1 ||
		quote.Attribution != quote.Segments[0].Attribution {
		t.Fatalf("quote metadata = %#v", quote)
	}

	path := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(path, []byte("first\n\nsecond"), 0600); err != nil {
		t.Fatal(err)
	}
	oldDB := FILE_STATE_DB
	FILE_STATE_DB = filepath.Join(t.TempDir(), "file-state.json")
	defer func() { FILE_STATE_DB = oldDB }()
	fileConfig := resolveTestConfig(testOptions{File: path, StdinIsTerminal: true, TimeoutSeconds: -1})
	next := newTestGenerator(fileConfig, nil)
	first, second, done := next(), next(), next()
	if first == nil || second == nil || done != nil || first.SourceID != "file:"+path ||
		first.Segments[0].Text != "first" || second.Segments[0].Text != "second" {
		t.Fatalf("file sequence = %#v, %#v, %#v", first, second, done)
	}
}
