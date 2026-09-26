# Plan 002: Reproducible measurements on Results

Status: proposed implementation specification. No feature in this document is implemented by the act of writing it. Source: [PRD](../../PRD.md). Planning baseline: 2026-09-26 working tree, including existing uncommitted `src/test.go`, `src/test_test.go`, and `src/tt.go` changes. Reinspect those files before implementation and preserve unrelated work.

## Goal and user job

A completed Charm test shows one deterministic metric definition with clear Retry and Next behavior.

As a terminal-heavy developer, I can exercise this capability entirely from the keyboard while keeping my input and progress local. This phase is one reviewable PR, estimated at 1-3 focused implementation sessions. The file table is the implementation boundary, including tests and public documentation. Generated outputs are declared separately.

## Decisions and assumptions

- Metric definition is `tt-metrics/1`. Count Unicode scalar values, including literal spaces and punctuation, excluding structural newlines and auto-advanced layout breaks. Do not normalize user-authored text. Grapheme-aware drawing does not change measurement units.
- Keep the existing per-invocation export schema and legacy metric meanings on compatibility routes until plan 015. New rich metrics use only the definition below; document the eventual approved formula difference.
- No abandoned attempt is stored or exported as a completed result. Completed and timed-expired attempts are valid. Zero-duration values are unavailable, represented by nil pointers/JSON null.
- Go, Bubble Tea v2, Lip Gloss v2, the existing Makefile, and local-only application data are inherited from plan 001. No server, account, telemetry, executable plugins, or release action is part of this phase.

## Scope

- Derive effective/raw WPM, CPM, input accuracy, final character outcomes, error history, consistency, and 1-second series from the session reducer.
- Show primary effective WPM, accuracy, consistency, raw WPM, remaining/total errors, time, configuration, source attribution, retry status, and an eligibility reason. Start with no PB marker until plan 012.
- Offer Next (default), Retry (same canonical prompt), and return-to-typing. Reserve no visible button for unfinished features. Freeze values at the terminal outcome and block buffered completion actions for 200 ms.

## Explicitly deferred scope

- Persistent rich history: plan 003.
- Weakness review/practice: plan 004.
- Graph and word/pair analysis: plan 010.
- PB comparison: plan 012.

## Requirement coverage

METRIC-01 through METRIC-05; SESSION-04 through SESSION-08, SESSION-10, SESSION-11; UI-02; PB-02, PB-03; DATA-01 (outcome policy).

Every listed requirement is checked by the acceptance criteria and the named fixtures below. Shared IDs can have incremental coverage in several plans. The index maps the final release gates.

## Acceptance criteria

1. A 60-second fixture with 250 retained correct scalars and 300 accepted typing scalars yields 50 effective WPM, 60 raw WPM, and 250 CPM.
2. For 275 correct attempts out of 300 attempts, accuracy is 91.67% on screen. Correcting 25 wrong attempts does not erase them from input accuracy or total error count.
3. At 0 ns duration every speed is unavailable, with no NaN/infinity in the view or serialization. At 1 ms, speeds are finite and tagged `short sample`; live speed stays unavailable before 1 second.
4. Same prompt plus event sequence replays to byte-equal metric JSON, including paused intervals, skip, corrections, Unicode, and mid-word timed expiry.
5. Retry repeats text exactly and is PB-ineligible. Next creates a new prompt ID. Completion processed twice yields one result object.

## Keyboard journey

1. Finish a scripted test containing one corrected mistake and one skipped word.
2. Read effective WPM first, inspect input accuracy and total/remaining errors.
3. Select Retry, verify identical text, and complete with the retry exclusion reason.
4. Select Next and verify fresh generated text with the same configuration.

## Verified reusable implementation

- `src/tt.go:27` defines legacy output fields and `src/tt.go:63` serializes them. Keep the adapter separate from rich results.
- `src/typer.go:262` currently counts final retained scalars; `src/tt.go:633` calculates correct CPM/WPM. Reuse the five-character convention, not the legacy independent calculations.
- Plan 001 supplies `Session.Apply`, ordered events, terminal outcomes, IDs, and pause accounting.

Current-state citations refer to the inspected baseline, not hypothetical future line numbers. Files introduced by predecessor plans are cited by interface name below.

## Components and file boundary

| File | Change |
| --- | --- |
| `src/metrics.go` (new) | Pure metric reducer and `SessionResult`. |
| `src/metrics_test.go` (new) | Golden event/metric fixtures and serialization boundaries. |
| `src/session.go` (plan 001) | Add outcome counters and per-word observation timing. |
| `src/app.go` (plan 001) | Replace basic report with rich Results and retry/next actions. |
| `src/app_test.go` (plan 001) | Result-action latch and journey tests. |
| `README.md:81`, `man.md` | Explain rich metrics and unchanged legacy export route. |

## Data model and constraints

```go
type CharacterTotals struct { Correct, Incorrect, Extra, Missed int }
type ErrorTotals struct { Total, Corrected, Uncorrected int }
type MetricSample struct { EndMS int64; WindowMS int64; WPM, RawWPM *float64; Errors int }
type WordObservation struct { Start, End int; ActiveMS int64; Attempts, CorrectAttempts, Errors int; Complete bool }
type Measurements struct {
    WPM, RawWPM, CPM, Accuracy, Consistency *float64
    ActiveMS, PauseMS int64; Characters CharacterTotals; Errors ErrorTotals
    Attempts, CorrectAttempts int; Series []MetricSample; Words []WordObservation
}
type SessionResult struct {
    ID, PromptID, RetryOf, MetricVersion string; FinishedUnixMS int64
    Config TestConfig; Outcome string; Practice bool
    Measurements Measurements; EligibilityReasons []string; Attribution string
}
```
Outcome is `completed` or `expired`; `RetryOf` is empty for a fresh attempt. Counters are nonnegative and stored as integer values. Finish time is UTC Unix milliseconds. Internal durations retain ns precision until conversion to ms. Metric calculations use ns, not rounded ms. Capture active correction/skip preferences in the result because they affect eligibility.
A wrong scalar creates an error-event identity. Removing that scalar counts it as corrected, even if the replacement is wrong again. Final mismatches are incorrect, typed overflow at a word boundary is extra, and traversed unfilled expected positions are missed. The unvisited suffix at timed expiry is excluded from missed counts. Total errors count wrong/extra attempts plus explicitly skipped expected scalars. Uncorrected is total minus corrected, including missed scalars. Space skip consumes one space attempt and marks skipped positions missed, not invented typed attempts.

Extend Session with `ExtraByWord map[int][]rune` and error-event IDs per retained slot. At the end of a canonical word, a non-space key before its expected delimiter is retained as an extra for that word. Backspace removes extras first, then canonical input. Space commits the word and its delimiter. With skip off, space is treated as ordinary input at the canonical slot; with skip on it explicitly marks the remaining word frontier missed and commits the delimiter. At the last word of a completion-based prompt, the final expected scalar completes immediately, so trailing extras are not accepted after completion. A skipped final word completes when the skip event visits its end. Structural newlines auto-advance in legacy inputs and are not attempted/correct scalars. Empty prompts are invalid generation, never instant-completed speed records.

Word timing is the active interval from its first attempt to delimiter/final scalar, with corrections returning to a completed word accumulated into that word's observation until final freeze. Neutral words with no errors still carry timing. Pure same-time IME text scalars are accepted in order and may produce zero-duration per-word speed, represented as unavailable. Input events and full prompts remain in memory and never enter automatic history. Paste messages are ignored during timed/count tests with a quiet `Paste ignored` explanation; existing legacy paste behavior remains characterized by plan 015's fixture rather than being silently changed. Ordinary key text including multi-scalar IME input is accepted in arrival order. No normalization changes authored text.


## Go and CLI contracts

```go
func Measure(s Session) (Measurements, error)
func FinishResult(s Session, finishedUnixMS int64) (SessionResult, error)
func RetrySession(previous Session) *Session
```
`Measure` rejects corrupt/non-monotonic replay with `ErrInvalidEvents`. Effective WPM = retained correct scalars * 60 / (5 * active seconds). CPM uses the same numerator without division by 5. Raw WPM = all accepted printable scalar attempts * 60 / (5 * active seconds). Accuracy = correct-at-insertion attempts / all scalar attempts * 100. Corrections do not reduce the attempt denominator.
Take disjoint active-time windows of 1000 ms, including zero-activity windows. Add the final partial window only when >=250 ms. Window effective/raw rates use correct-at-insertion/all-attempt counts during that window, explicitly labelled interval speed, not recomputed final retained accuracy. Error samples count newly created error identities. Consistency = max(0, 100 * (1 - population SD / mean)) over interval raw rates, unavailable with <2 windows or mean 0. Compute with float64, store full precision, round half-up to 2 decimals for display and CSV rich summaries. Final speeds can differ from interval speeds after corrections; label this fact.
Words are maximal non-whitespace spans. Timing starts at first entered scalar and stops at delimiter/final character. Include idle gaps, exclude overlays. Store attempts and error counts per occurrence; speed unavailable for incomplete/zero-time spans. Burst is unavailable in v1, with no fabricated burst score.

There are no HTTP APIs in this phase unless an explicit network contract appears above. Return errors rather than panicking across the application boundary. Bubble Tea commands perform I/O and report immutable messages to the model. `Update` owns mutable UI/session state and does not wait on I/O.

## Dependencies and operation sequence

Required predecessor plans: 1

1. Settle session clock, freeze terminal state, replay ordered events, calculate final retained and attempted totals, then create one immutable result.
2. At mid-word expiry, include visited positions and accepted attempts only. Stop at the exact selected active duration and reject keys whose settled time is at/after expiry.
3. Render Results from the immutable result. Retry creates a new attempt linked to the prior ID. Next uses the generator and clears retry linkage.
4. Derive live WPM and error count with `Measure`; label live errors `total errors`. Do not run separate formulas in the view.
5. Golden examples also cover `cat` entered as `cxt`, Backspace twice, `at`: 5 attempts, 4 correct-at-entry attempts, 3 retained correct, 1 corrected error. At 2 seconds: effective WPM 18, raw WPM 30, CPM 90, accuracy 80%.
6. Maintain incremental metric counters and active-window aggregates in Session as events arrive. Live Measure reads the cached aggregate in O(1) plus current-word size; it does not replay the entire event log on every keystroke. Final replay is the deterministic validation oracle in tests, while production finalization freezes the same aggregate without a blocking full-log pass. Bound graph/word work in commands and show Results immediately from the frozen totals.

## Non-functional and cross-cutting rules

- Typing input processing performs no filesystem, audio-device, or network waits. No input event is discarded to catch up with rendering. Coalesce redraws, never accepted typing events.
- Use the same active clock for input, metrics, timer, and charts. Application overlays pause once at the outermost entry and resume once after the last blocking view closes.
- Authenticate nothing: this is a local single-user executable. Validate all new input at its boundary and show path/resource/action-specific errors. Local diagnostics contain no typing text unless the user explicitly requested it.
- Network rate limiting belongs exclusively to plan 016. History indexing and concurrency belong to plan 003. This phase consumes those policies only when they are prerequisites.
- Measure ready-screen <=250 ms, event-to-visible-input <=50 ms, and state-preserving resize <=100 ms on the documented reference setup when this phase changes those paths. Use 100 samples and report p50, p95, p99, and maximum. Slow terminal behavior is recorded rather than described as passing.

## Failure modes and required handling

| Path and failure | Handling and caller visibility | Required coverage |
| --- | --- | --- |
| Zero or tiny duration | null unavailable values or finite short sample; visible label | +3 duration fixtures |
| Correction counts erased errors | Preserve event identities, calculate retained and attempt counts separately | +5 correction/skip fixtures |
| Pause creates series spikes | Slice by active time only; deterministic replay error on invalid order | +3 pause/window cases |
| Timed suffix treated as all missed | Count visited frontier only; visible final outcome | +2 timeout cases |
| Buffered completion key selects Next | Guard 200 ms and require a fresh activation; Ctrl-C allowed | +2 journey cases |

The coverage above is required new coverage, not a claim that these tests exist today. No silent, unhandled failure in a changed path satisfies this phase.

## Test plan

| Layer | Named coverage | Minimum new cases |
| --- | --- | --- |
| Unit | Metric formula, Unicode, error, consistency, rounding and time fixtures | 18 |
| Integration | Session replay, live/final projection, retry linkage | 5 |
| E2E | Known-error result and buffered-key guard | 2 |

Use deterministic seeds, clocks, and temporary data directories. E2E cases mean scripted terminal/model journeys with real generation and storage where used, plus native-terminal inspection where capabilities cannot be simulated. Run `GOCACHE=/private/tmp/terminal-typer-go-cache make verify`, `GOCACHE=/private/tmp/terminal-typer-go-cache make smoke`, and `git diff --check`. Update `README.md` and `man.md` together when behavior becomes public. Regenerate with `make assets` when the manual or source assets change and verify `gzip -t tt.1.gz`. Do not edit `src/packed.go` by hand.

## Rollout and rollback

Only the opt-in Charm route changes measurements in this phase. No data migration. Roll back by reverting the metric/view PR or using the compatibility renderer; rich result objects have not yet been persisted. Keep legacy output contracts intact. Document formula version now, then the full export cutover in plan 015.

Release, deployment, publishing binaries, and uploading user data require separate explicit human authorization. Update `CHANGELOG.md` when a release occurs, not while this proposal is written.

## Definition of done

- [ ] AC-1: A 60-second fixture with 250 retained correct scalars and 300 accepted typing scalars yields 50 effective WPM, 60 raw WPM, and 250 CPM.
- [ ] AC-2: For 275 correct attempts out of 300 attempts, accuracy is 91.67% on screen. Correcting 25 wrong attempts does not erase them from input accuracy or total error count.
- [ ] AC-3: At 0 ns duration every speed is unavailable, with no NaN/infinity in the view or serialization. At 1 ms, speeds are finite and tagged `short sample`; live speed stays unavailable before 1 second.
- [ ] AC-4: Same prompt plus event sequence replays to byte-equal metric JSON, including paused intervals, skip, corrections, Unicode, and mid-word timed expiry.
- [ ] AC-5: Retry repeats text exactly and is PB-ineligible. Next creates a new prompt ID. Completion processed twice yields one result object.
