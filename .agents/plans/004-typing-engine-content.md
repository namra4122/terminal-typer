# Phase 1 - Typing engine and content

- Lifecycle: planning outline. Convert each candidate below into a scoped implementation spec before changing code.
- Goal: make every typing test use a consistent configuration, content source, and session lifecycle while preserving existing commands and input behavior.
- User outcome: choose English, Hinglish, or code-oriented material and run timed, word-count, quote, custom, or practice tests in the terminal.

## Current starting point

- `src/tt.go:434-464` defines the current flags; `src/tt.go:527-548` selects one of the word, quote, stdin, or file generators. Preserve their precedence for existing invocations.
- `src/wordtest.go:5-25`, `src/quotetest.go:8-27`, `src/datatest.go:3-26`, and `src/filetest.go:8-43` return `[]segment` through separate closures. `src/typer.go:26-35` defines the current segment and mistake types.
- `src/typer.go:157-190` advances segments; `src/typer.go:247-510` owns event handling, timing, cursor input, and test exits. Settings already pause the timer; resize currently restarts the test (`src/tt.go:654-658`).
- Embedded source assets live in `words/`, `quotes/`, `themes/`, and `sounds/`; `Makefile:39-43` regenerates the packed source and manual. Existing resource lookup must continue to work.

## Spec candidates

1. Define a `TestConfig` model for mode, content pack, duration or word count, difficulty, and independent modifiers. Specify defaults and CLI precedence so bare `tt`, `-n`, `-g`, `-t`, `-words`, `-quotes`, positional file input, stdin, `-raw`, and `-multi` retain their current meaning.
1. Define one `Test` representation with source identity, display text or segments, attribution, generation parameters, and whether results count toward history and personal bests. Route existing generators through it before adding new modes.
1. Separate content selection from the typing session. Define explicit `time`, `words`, `quote`, `custom`, and `practice` modes; add 15/30/60/120-second and 10/25/50/100-word choices plus validated custom lengths. Preserve `-t` and `-n` as compatible entry points.
1. Build natural-language packs: existing English 1k as the baseline, then curated English 5k/10k and conversational Roman-script Hinglish. State provenance and licensing for every new pack; keep packs local and embedded or user supplied.
1. Build realistic code-snippet packs for Go, Python, JavaScript, TypeScript, Rust, Bash, SQL, JSON, and YAML. Preserve significant whitespace and punctuation in code tests; do not turn code into random keyword lists.
1. Add independent punctuation, numbers, and capitalization or realistic-text controls for suitable natural-language generators. Define whether combinations apply to quotes, stdin, files, and code; avoid silently transforming user-supplied text.
1. Extract a reusable typing session state model for start, input, pause, resume, timeout, complete, restart, navigation, and resize. Record timestamped input and correction events without persisting full user text by default. Preserve current backspace, word skipping, and control keys.
1. Define terminal resize behavior and test replay from a stable generated prompt. Ensure paused time never counts toward live or final speed and a restart does not accidentally record a completed result.

## Completion signals

- A user can complete one test from each supported source through the same session lifecycle, including file and piped input.
- New modes and packs can be selected without changing the meaning of existing documented flags.
- A timed test stops on elapsed active time; a word test stops on its configured count; a practice test is marked ineligible for ordinary personal bests.
- Punctuation, numbers, and capitalization can be toggled independently where applicable, and code snippets retain meaningful syntax and line breaks.
- Session tests cover correction, pause, timeout, restart, navigation, and resize. `make verify` and `make smoke` pass.

## Boundaries and dependencies

- Phase 2 defines derived metrics and persistent result history from these events. Keep event semantics stable enough to support it, but do not add graphs or PB logic here.
- Phase 4 defines adaptive practice and difficulty. Phase 1 only supplies an explicit practice mode and eligibility marker.
- Plan `003-tui-redesign.md` contains visual direction; this phase reuses existing `tcell` rendering and does not require a framework change.
