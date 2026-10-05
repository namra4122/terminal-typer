package main

import (
	cryptorand "crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	rand "math/rand/v2"
	"path/filepath"
	"strings"
	"time"
)

type testMode string

const (
	timedMode  testMode = "timed"
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
	Origin             ResourceOrigin
	Segments           []segment
	Attribution        string
	EligibleForHistory bool
	EligibleForPB      bool
	wordStream         *deterministicWordStream
}

type deterministicWordStream struct {
	words           []string
	rng             *rand.Rand
	last            string
	hasAlternatives bool
}

func newDeterministicWordStream(words []string, seed1, seed2 uint64) (*deterministicWordStream, error) {
	if len(words) == 0 {
		return nil, fmt.Errorf("word pack is empty")
	}
	stream := &deterministicWordStream{
		words: append([]string(nil), words...),
		rng:   rand.New(rand.NewPCG(seed1, seed2)),
	}
	for _, word := range words[1:] {
		if word != words[0] {
			stream.hasAlternatives = true
			break
		}
	}
	return stream, nil
}

func (s *deterministicWordStream) next(count int) []string {
	if s == nil || count <= 0 {
		return nil
	}
	generated := make([]string, count)
	for i := range generated {
		word := s.words[s.rng.IntN(len(s.words))]
		if s.hasAlternatives {
			for word == s.last {
				word = s.words[s.rng.IntN(len(s.words))]
			}
		}
		generated[i] = word
		s.last = word
	}
	return generated
}

func (t *Test) appendTimedWords(count int) string {
	if t == nil || t.Config.Mode != timedMode || t.wordStream == nil || count <= 0 {
		return ""
	}
	addition := strings.Join(t.wordStream.next(count), " ")
	if addition == "" {
		return ""
	}
	if len(t.Segments) == 0 {
		t.Segments = []segment{{Text: addition}}
	} else if t.Segments[0].Text == "" {
		t.Segments[0].Text = addition
	} else {
		t.Segments[0].Text += " " + addition
	}
	t.Config.WordCount += count
	t.Config.WordsPerGroup += count
	return addition
}

func testSeed() (uint64, uint64, error) {
	var seed [16]byte
	if _, err := cryptorand.Read(seed[:]); err != nil {
		return 0, 0, fmt.Errorf("creating test seed: %w", err)
	}
	return binary.LittleEndian.Uint64(seed[:8]), binary.LittleEndian.Uint64(seed[8:]), nil
}

func prepareTestGenerator(cfg TestConfig, stdinData []byte) (func() *Test, error) {
	var generate func() []segment
	var timedWords []string
	var origin ResourceOrigin
	sourceID := string(cfg.Source) + ":" + cfg.Pack
	switch cfg.Source {
	case wordSource:
		data, resolved, err := ResolveResource("words", cfg.Pack)
		if err != nil {
			return nil, fmt.Errorf("word pack %q: %w", cfg.Pack, err)
		}
		origin = resolved
		timedWords = strings.Fields(string(data))
		if len(timedWords) == 0 {
			return nil, fmt.Errorf("word pack %q is empty", cfg.Pack)
		}
	case quoteSource:
		data, resolved, err := ResolveResource("quotes", cfg.Pack)
		if err != nil {
			return nil, fmt.Errorf("quote pack %q: %w", cfg.Pack, err)
		}
		var quotes []segment
		if err := json.Unmarshal(data, &quotes); err != nil {
			return nil, fmt.Errorf("quote pack %q: %w", cfg.Pack, err)
		}
		if len(quotes) == 0 {
			return nil, fmt.Errorf("quote pack %q is empty", cfg.Pack)
		}
		origin = resolved
		seed1, seed2, err := testSeed()
		if err != nil {
			return nil, err
		}
		rng := rand.New(rand.NewPCG(seed1, seed2))
		generate = func() []segment {
			quote := quotes[rng.IntN(len(quotes))]
			return []segment{quote}
		}
	case stdinSource:
		origin = ResourceOrigin{Kind: "stdin", Path: "-"}
		generate = generateTestFromData(stdinData, cfg.Raw, cfg.Multi)
		sourceID = "stdin"
	case fileSource:
		origin = ResourceOrigin{Kind: "file", Path: cfg.Pack}
		generate = generateTestFromFile(cfg.Pack, cfg.StartParagraph)
		path, err := filepath.Abs(cfg.Pack)
		if err != nil {
			return nil, fmt.Errorf("resolving input path: %w", err)
		}
		sourceID = "file:" + path
	default:
		return nil, fmt.Errorf("unsupported test source %q", cfg.Source)
	}

	var seedSource *rand.Rand
	if cfg.Source == wordSource {
		seed1, seed2, err := testSeed()
		if err != nil {
			return nil, err
		}
		seedSource = rand.New(rand.NewPCG(seed1, seed2))
	}
	return func() *Test {
		var segments []segment
		var stream *deterministicWordStream
		if cfg.Source == wordSource {
			stream, _ = newDeterministicWordStream(timedWords, seedSource.Uint64(), seedSource.Uint64())
			if cfg.Mode == timedMode || cfg.Groups <= 1 {
				count := cfg.WordCount
				if count <= 0 {
					count = cfg.WordsPerGroup
				}
				segments = []segment{{Text: strings.Join(stream.next(count), " ")}}
			} else {
				segments = make([]segment, cfg.Groups)
				for i := range segments {
					segments[i].Text = strings.Join(stream.next(cfg.WordsPerGroup), " ")
				}
			}
		} else {
			segments = generate()
		}
		if segments == nil {
			return nil
		}
		test := &Test{
			Config:             cfg,
			SourceID:           sourceID,
			Origin:             origin,
			Segments:           segments,
			EligibleForHistory: true,
			EligibleForPB:      true,
		}
		if cfg.Mode == timedMode {
			test.wordStream = stream
		}
		if len(segments) == 1 {
			test.Attribution = segments[0].Attribution
		}
		return test
	}, nil
}

func newTestGenerator(cfg TestConfig, stdinData []byte) func() *Test {
	generate, err := prepareTestGenerator(cfg, stdinData)
	if err != nil {
		die("%v", err)
	}
	return generate
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
