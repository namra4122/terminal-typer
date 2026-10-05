package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
		quote.Attribution != quote.Segments[0].Attribution || quote.Origin.Kind != "embedded-quote" || !quote.Origin.Embedded {
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

func TestResolveResourceReturnsWinningOrigin(t *testing.T) {
	explicit := filepath.Join(t.TempDir(), "private-words")
	if err := os.WriteFile(explicit, []byte("secret alpha"), 0600); err != nil {
		t.Fatal(err)
	}
	data, origin, err := ResolveResource("words", explicit)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "secret alpha" || origin.Kind != "private-word" || origin.Path != explicit || origin.Embedded {
		t.Fatalf("explicit resource = %q, %#v", data, origin)
	}

	data, origin, err = ResolveResource("words", "1000en")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 || origin.Kind != "embedded-word" || origin.PackID != "1000en" ||
		origin.Revision == "" || origin.Path != "" || !origin.Embedded {
		t.Fatalf("embedded resource = %d bytes, %#v", len(data), origin)
	}
}

func TestGeneratedWordTestCarriesActualOrigin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "words")
	if err := os.WriteFile(path, []byte("alpha beta gamma"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := resolveTestConfig(testOptions{
		Words: path, StdinIsTerminal: true, WordsPerGroup: 2, Groups: 1, TimeoutSeconds: -1,
	})
	generated := newTestGenerator(cfg, nil)()
	if generated == nil || generated.Origin.Kind != "private-word" || generated.Origin.Path != path ||
		generated.Origin.Embedded {
		t.Fatalf("generated origin = %#v", generated)
	}
}

func TestTimedGeneratorBuildsInitialBufferAndContinuesDeterministicStream(t *testing.T) {
	cfg := TestConfig{
		Mode: timedMode, Source: wordSource, Pack: "1000en",
		TimeLimit: 30 * time.Second, WordCount: 200, WordsPerGroup: 200, Groups: 1,
		Difficulty: "normal",
	}
	generate, err := prepareTestGenerator(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	generated := generate()
	if generated == nil || generated.wordStream == nil || len(generated.Segments) != 1 {
		t.Fatalf("timed test = %#v", generated)
	}
	initial := generated.Segments[0].Text
	if words := strings.Fields(initial); len(words) != 200 {
		t.Fatalf("initial timed buffer has %d words, want 200", len(words))
	}
	addition := generated.appendTimedWords(100)
	if len(strings.Fields(addition)) != 100 || len(strings.Fields(generated.Segments[0].Text)) != 300 {
		t.Fatalf("extended timed buffer = %d + %d words", len(strings.Fields(initial)), len(strings.Fields(addition)))
	}
	if !strings.HasPrefix(generated.Segments[0].Text, initial+" ") {
		t.Fatal("timed extension changed the generated prefix")
	}
	beforeBoundary := strings.Fields(initial)
	afterBoundary := strings.Fields(addition)
	if beforeBoundary[len(beforeBoundary)-1] == afterBoundary[0] {
		t.Fatal("timed extension repeated the boundary word")
	}

	retry := NewSession(generated, "retry", "same-prompt")
	if string(retry.prompt()) != generated.Segments[0].Text {
		t.Fatal("retry did not rewind the complete cached timed material")
	}
}

func TestTimedWordStreamMatchesAcrossChunkBoundaries(t *testing.T) {
	words := []string{"alpha", "beta", "gamma", "delta"}
	one, err := newDeterministicWordStream(words, 7, 11)
	if err != nil {
		t.Fatal(err)
	}
	two, err := newDeterministicWordStream(words, 7, 11)
	if err != nil {
		t.Fatal(err)
	}
	allAtOnce := one.next(300)
	inChunks := append(two.next(200), two.next(100)...)
	if !reflect.DeepEqual(allAtOnce, inChunks) {
		t.Fatal("chunked timed generation did not continue the original random stream")
	}
}

func TestSavedCountAndQuoteGeneratorsProduceConfiguredContent(t *testing.T) {
	count := TestConfig{
		Mode: wordMode, Source: wordSource, Pack: "1000en",
		TimeLimit: -1, WordCount: 25, WordsPerGroup: 25, Groups: 1, Difficulty: "normal",
	}
	countGenerate, err := prepareTestGenerator(count, nil)
	if err != nil {
		t.Fatal(err)
	}
	if generated := countGenerate(); generated == nil || len(strings.Fields(generated.Segments[0].Text)) != 25 {
		t.Fatalf("count generator = %#v", generated)
	}

	quote := TestConfig{
		Mode: quoteMode, Source: quoteSource, Pack: "en",
		TimeLimit: -1, Groups: 1, Difficulty: "normal",
	}
	quoteGenerate, err := prepareTestGenerator(quote, nil)
	if err != nil {
		t.Fatal(err)
	}
	generated := quoteGenerate()
	if generated == nil || len(generated.Segments) != 1 || generated.Segments[0].Text == "" ||
		generated.Attribution != generated.Segments[0].Attribution || generated.Origin.Kind != "embedded-quote" {
		t.Fatalf("quote generator = %#v", generated)
	}
}
