# Phase 4 - Training, difficulty, and keyboards

- Lifecycle: planning outline. Split difficulty, practice, and keyboard work into bounded implementation specs.
- Goal: use local result data to target weak typing patterns and support multiple keyboard layouts.
- User outcome: run focused practice and see progress for the layout and code symbols they use.

## Current starting point

- `src/typer.go:31-35` records mistyped words; `src/tt.go:336-345` and `src/db.go:9-55` keep mistake data local. Phase 2 adds timed errors, key metrics, tags, and history.
- `src/typer.go:460-500` already handles word deletion, backspace, and rune input. Existing `-noskip` and `-nobackspace` flags and saved settings must remain meaningful.
- `src/layout.go:159-268` contains width-aware drawing helpers suitable for an optional keymap view.

## Spec candidates

1. Define normal, expert, and master difficulty with precise failure and correction rules. Specify stop-on-character, stop-on-word, no-backspace, no-correction, and blind controls; resolve their interactions with existing skipping and deletion keys.
1. Add minimum WPM, accuracy, and burst thresholds with explicit evaluation windows, fail states, and result eligibility. Keep aborted and failed tests distinguishable from completed tests in Phase 2 history.
1. Generate practice tests from missed words, slow words, frequently mistyped words, difficult character pairs, and difficult word transitions. Show why each item was selected and let users repeat or dismiss it.
1. Create developer-specific practice for braces, brackets, parentheses, semicolon, colon, underscore, hyphen, slash, and backslash, plus weak tokens from the selected programming-language pack. Preserve syntax and meaningful surrounding context.
1. Define QWERTY, Colemak, Colemak-DH, Dvorak, and Workman logical layouts. Add a versioned local format for custom layouts. Keep physical key positions and logical character mapping separate, and do not assume terminal input reveals hardware scan codes.
1. Add an optional on-screen keyboard view that reacts to input the terminal can report, highlights incorrect characters, and degrades clearly when physical key identity is unavailable.
1. Group key analytics and results by selected layout and allow layout-specific history, tags, and PB comparisons. Define how changing layouts mid-session affects result attribution.
1. Make practice sessions ineligible for ordinary PBs and trends by default, while retaining an optional separate practice log for progress.

## Completion signals

- Each difficulty rule has a reproducible pass or fail outcome under correction, skip, and backspace paths.
- A completed test with known weak words can launch a local practice test that contains those words or transitions and explains the selection.
- Selecting a logical layout changes analytics labels and keymap visualization without rewriting the terminal's actual keyboard input.
- Practice does not change ordinary PBs. `make verify` and `make smoke` pass.

## Boundaries and dependencies

- Requires Phase 1's explicit practice mode and Phase 2's eligible history, per-key timing, and error positions.
- Use Phase 3 navigation and compact layout patterns for practice selection and the optional keymap.
- Terminal APIs may not reveal physical keys. State inferred or unavailable physical-key data honestly in the eventual implementation spec.
