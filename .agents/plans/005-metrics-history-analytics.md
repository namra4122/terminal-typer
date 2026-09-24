# Phase 2 - Metrics, history, and analytics

- Lifecycle: planning outline. Write smaller implementation specs for metric definitions, storage, and views.
- Goal: turn a completed test into an explainable local performance record.
- User outcome: inspect speed, accuracy, timing, errors, trends, and personal bests without an online account.

## Current starting point

- `src/tt.go:27-33` defines a small result with WPM, CPM, accuracy, timestamp, and mistakes. `src/tt.go:63-91` emits JSON and CSV; `src/tt.go:94-229` renders the interactive report.
- `src/tt.go:635-647` calculates final speed and accuracy from aggregate counts and appends a process-local result. `src/typer.go:346-347` calculates optional live WPM.
- `src/db.go:9-55` establishes the local XDG data directory and stores file progress and mistakes as JSON. `src/tt.go:336-345` saves mistakes. Phase 1 supplies timestamped session events and test metadata.

## Spec candidates

1. Specify metric definitions and edge cases before implementation: effective WPM, raw WPM, CPM, accuracy, consistency, elapsed active time, total errors, per-word burst, peak burst, and average burst. Define zero-duration, timeout, skipped-word, corrected-error, and unfinished-test treatment with examples.
1. Classify character outcomes as correct, incorrect, extra, or missed, and record positions and correction paths. Compute per-word timing, key latency, inter-key spacing, and per-second WPM, raw WPM, and error series from Phase 1 events.
1. Expand the in-memory result to include mode, language or content pack, duration or word count, modifiers, difficulty, layout when available, timestamp, mistake summary, character counts, consistency, burst, and metric samples. Keep the existing JSON fields and CSV column order usable; document any additive output fields or opt-in richer format.
1. Add versioned, local test history under the existing XDG data directory. Specify atomic writes, file permissions, corruption handling, retention, and migration from the current settings and mistake stores without deleting them.
1. Add personal bests grouped by mode, test length, language or content pack, and eligible modifiers. Specify tie handling and exclusions for practice, aborted tests, and challenge modifiers before calculating records.
1. Add user tags, tagged personal bests, and filters for history by time, mode, language, tag, layout, and result eligibility. Keep tags local and define rename/delete effects on stored results.
1. Add post-test terminal graphs for WPM, raw WPM, errors, and burst over time, with a text summary when the terminal cannot fit a graph.
1. Add historical trend views for recent WPM, accuracy, and consistency, plus character and key timing breakdowns. Make sample size visible so sparse data is not presented as a reliable trend.

## Completion signals

- The same completed session yields deterministic live, report, JSON, CSV, history, and graph values according to documented formulas.
- History survives process exit, remains local, and handles malformed or interrupted writes without erasing prior valid records.
- PBs and tagged PBs distinguish eligible tests from practice or aborted tests and can be reproduced from stored results.
- A user can inspect a result and at least one trend entirely in the terminal. `make verify` and `make smoke` pass.

## Boundaries and dependencies

- Requires Phase 1's event stream, test configuration, and eligibility marker.
- Phase 3 gives these views their final navigation and layout. This phase may use existing report and layout primitives for a working first presentation.
- Phase 4 uses timing and error history to choose practice material and evaluate training thresholds.
