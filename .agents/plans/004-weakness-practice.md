# Plan 004: Review and practice measured weaknesses

Status: proposed implementation specification. No feature in this document is implemented by the act of writing it. Source: [PRD](../../PRD.md). Planning baseline: 2026-09-26 working tree, including existing uncommitted `src/test.go`, `src/test_test.go`, and `src/tt.go` changes. Reinspect those files before implementation and preserve unrelated work.

## Goal and user job

After a real test, a user can review measured weaknesses, complete a short targeted drill, and compare practiced items.

As a terminal-heavy developer, I can exercise this capability entirely from the keyboard while keeping my input and progress local. This phase is one reviewable PR, estimated at 1-3 focused implementation sessions. The file table is the implementation boundary, including tests and public documentation. Generated outputs are declared separately.

## Decisions and assumptions

- Recent evidence is the just-completed regular test plus at most 20 preceding completed/expired regular results from the last 30 days, matching pack revision and case-sensitive item text. Deduplicate the current result ID if it already appears in history. Private fragments are unavailable without plan 011 consent.
- A candidate needs >=2 occurrences OR >=1 wrong attempt in the current test. Slow-only candidates need >=3 complete occurrences and a per-scalar median duration at least 1.5 times the same sample's median. Rank descending error rate, then median ms/scalar, then Unicode lexical order. Select up to 5 items.
- Use exactly 25 whitespace-delimited words: reserve 15 slots for weak items/context and 10 neutral slots from the current embedded word language. Preserve punctuation and a <=5-word original context for quote weaknesses. With no matching neutral pack or fitting context, show insufficient suitable data. Use `1000en` neutrals for English quote packs only.
- Go, Bubble Tea v2, Lip Gloss v2, the existing Makefile, and local-only application data are inherited from plan 001. No server, account, telemetry, executable plugins, or release action is part of this phase.

## Scope

- Expose Practice weaknesses only when qualifying evidence exists; otherwise show a reason and sample size.
- Review each selected item, error/slow reason, sample window, current/history contribution, and count. Start is explicit. Cancel preserves Results.
- Generate a deterministic interleave of 15 weak-context slots and 10 neutral slots, no adjacent duplicate tokens when a distinct token exists. Cycle ranked candidates to fit exact 25 slots; keep each context intact. Practice result compares item accuracy, error rate, and median ms/scalar, with sample limits.
- Practice again repeats the selected curriculum in a fresh shuffle. Return to regular test restores prior regular configuration and generates fresh text. Dismiss an item only within this review session; rebuild selection next time.

## Explicitly deferred scope

- Long-term adaptive training, physical keyboards and difficulty: post-v1, listed in the index.
- Character-pair practice and symbol-specific drills: post-v1. Pair diagnostics become available in plan 010.

## Requirement coverage

PRACTICE-01 through PRACTICE-07; MODE-04; UI-07; UI-02 (weakness action); DATA-03; PB-03; METRIC-06.

Every listed requirement is checked by the acceptance criteria and the named fixtures below. Shared IDs can have incremental coverage in several plans. The index maps the final release gates.

## Acceptance criteria

1. Known error/slow fixtures select the documented ranked items, explain their evidence, and generate exactly 25 words with a 15:10 weak-to-neutral slot ratio.
2. Cancelling review records no test and leaves the prior result/configuration unchanged. A private test without retention has no extracted word candidates.
3. A completed practice record is `practice=true`, is PB-ineligible, and appears only in the practice History view by default.
4. Comparison shows null/unavailable speed when either baseline or drill has fewer than 3 complete item occurrences; it never claims improvement from one sparse sample.
5. Practice again generates a new attempt; dismiss/repeat/return routes work entirely by keyboard.

## Keyboard journey

1. Finish a regular test and activate Practice weaknesses.
2. Review selected items and reasons; dismiss an item with Delete or restore it with r.
3. Enter starts the drill. Complete it and inspect per-item change/sample counts.
4. Choose Practice again or Return to regular test; Escape returns to the originating result.

## Verified reusable implementation

- `src/typer.go:192` already identifies mistyped words but lacks timing. Reuse the word/mistake concept, not aggregate `.errors` for fabricated historical observations.
- Plans 002-003 provide `WordObservation`, `Measurements`, permitted fragments and `ReadHistory`. Plan 001 handles timed-paused input and IDs.

Current-state citations refer to the inspected baseline, not hypothetical future line numbers. Files introduced by predecessor plans are cited by interface name below.

## Components and file boundary

| File | Change |
| --- | --- |
| `src/practice.go` (new) | Evidence selection, deterministic 25-word drill and comparisons. |
| `src/practice_test.go` (new) | Ranking, privacy, context fitting and sample fixtures. |
| `src/app.go` (plan 001) | Review and practice result states. |
| `src/history.go` (plan 003) | Add optional practice evidence fields to the v1 envelope. |
| `src/app_test.go` (plan 001) | Review/cancel/start/repeat/return journeys. |
| `README.md:81`, `man.md` | Describe practice and comparability limits. |

## Data model and constraints

```go
type PracticeItem struct {
    Item, Context, Reason string; Occurrences, Attempts, CorrectAttempts, Errors int
    MedianMSPerScalar *float64; CurrentOccurrences, HistoryOccurrences int
}
type PracticePlan struct { ParentID string; Items []PracticeItem; Tokens []string; WindowStartUnixMS int64; SampleSessions int }
type PracticeComparison struct { Item string; Baseline, Drill PracticeItem; AccuracyDelta, SpeedChangePercent *float64 }
```
Add `practiceDetail` (optional) to HistoryRecord: `{parentId:string,windowStartUnixMS:int64,sampleSessions:int,items:[]PracticeItem,comparisons:[]PracticeComparison}` with lowerCamelCase keys. Retain permitted baseline/drill summaries only, never the full 25-word prompt. `SpeedChangePercent` is `(baseline ms/scalar - drill ms/scalar)/baseline *100`, unavailable when the baseline is zero or sparse. Accuracy uses attempts. Store per-occurrence duration/count summaries within permitted fragment observations as `{activeMS:int64,scalars:int,attempts:int,correctAttempts:int,errors:int}`; <=50 most recent complete occurrences per item, ordered by result time then occurrence.

## Go and CLI contracts

```go
func SelectPractice(current SessionResult, recent []HistoryRecord) (PracticePlan, error)
func BuildPractice(p PracticePlan, neutral []string, seed int64) (*Test, error)
func ComparePractice(p PracticePlan, r SessionResult) []PracticeComparison
```
`ErrInsufficientEvidence`, `ErrNoNeutralPack`, and `ErrContextTooLong` produce visible review explanations. Start consumes plan 001 `NewSession`; completion consumes plan 002 `FinishResult`, then plan 003 `ProjectHistory`/`SaveHistory`. Retry does not automatically start practice. Round-robin complete contexts until 15 weak slots are filled; fill an unfillable remainder with the selected single weak word, never truncate authored context. Alternate weak blocks and neutral tokens, then seeded-shuffle blocks while preserving each context. Ordinary trends ignore these records.

There are no HTTP APIs in this phase unless an explicit network contract appears above. Return errors rather than panicking across the application boundary. Bubble Tea commands perform I/O and report immutable messages to the model. `Update` owns mutable UI/session state and does not wait on I/O.

## Dependencies and operation sequence

Required predecessor plans: 3

1. Collect current permitted observations; load the bounded comparable regular-history window. If history is unavailable, use the current test alone and disclose that limit.
2. Rank and review evidence, remove dismissed items, and recalculate Start availability. Cancel mutates neither history nor configuration.
3. Generate the actual drill with neutral words from the same language, execute the shared session engine, persist separate practice status, and calculate item comparisons.
4. Return to regular restores the saved regular test configuration. Practice again retains the curriculum but generates a fresh attempt and interleave.

## Non-functional and cross-cutting rules

- Typing input processing performs no filesystem, audio-device, or network waits. No input event is discarded to catch up with rendering. Coalesce redraws, never accepted typing events.
- Use the same active clock for input, metrics, timer, and charts. Application overlays pause once at the outermost entry and resume once after the last blocking view closes.
- Authenticate nothing: this is a local single-user executable. Validate all new input at its boundary and show path/resource/action-specific errors. Local diagnostics contain no typing text unless the user explicitly requested it.
- Network rate limiting belongs exclusively to plan 016. History indexing and concurrency belong to plan 003. This phase consumes those policies only when they are prerequisites.
- Measure ready-screen <=250 ms, event-to-visible-input <=50 ms, and state-preserving resize <=100 ms on the documented reference setup when this phase changes those paths. Use 100 samples and report p50, p95, p99, and maximum. Slow terminal behavior is recorded rather than described as passing.

## Failure modes and required handling

| Path and failure | Handling and caller visibility | Required coverage |
| --- | --- | --- |
| No usable sample | Explain insufficient evidence and keep regular Next available | +3 sparse cases |
| History unavailable | Current-only selection with explicit sample label | +2 degraded cases |
| Private source fragments requested | Enforce privacy before candidate construction | +3 source cases |
| Context or neutral pack cannot fit | Disable Start with remedy, never truncate quote text | +3 generation cases |
| Practice contaminates ordinary records | Explicit practice flag/reasons in projection and queries | +2 integration cases |

The coverage above is required new coverage, not a claim that these tests exist today. No silent, unhandled failure in a changed path satisfies this phase.

## Test plan

| Layer | Named coverage | Minimum new cases |
| --- | --- | --- |
| Unit | Ranking, timing medians, slot/context counts, comparisons | 15 |
| Integration | Current/history merge, privacy and persisted practice | 5 |
| E2E | Review/cancel and complete/return | 2 |

Use deterministic seeds, clocks, and temporary data directories. E2E cases mean scripted terminal/model journeys with real generation and storage where used, plus native-terminal inspection where capabilities cannot be simulated. Run `GOCACHE=/private/tmp/terminal-typer-go-cache make verify`, `GOCACHE=/private/tmp/terminal-typer-go-cache make smoke`, and `git diff --check`. Update `README.md` and `man.md` together when behavior becomes public. Regenerate with `make assets` when the manual or source assets change and verify `gzip -t tt.1.gz`. Do not edit `src/packed.go` by hand.

## Rollout and rollback

Additive v1 JSON fields allow prior readers to ignore practiceDetail. Existing regular records remain valid. Rollback removes the practice action but keeps practice files and their explicit practice flag; compatible old History ignores unknown optional fields. No original prompt or legacy store is migrated.

Release, deployment, publishing binaries, and uploading user data require separate explicit human authorization. Update `CHANGELOG.md` when a release occurs, not while this proposal is written.

## Definition of done

- [ ] AC-1: Known error/slow fixtures select the documented ranked items, explain their evidence, and generate exactly 25 words with a 15:10 weak-to-neutral slot ratio.
- [ ] AC-2: Cancelling review records no test and leaves the prior result/configuration unchanged. A private test without retention has no extracted word candidates.
- [ ] AC-3: A completed practice record is `practice=true`, is PB-ineligible, and appears only in the practice History view by default.
- [ ] AC-4: Comparison shows null/unavailable speed when either baseline or drill has fewer than 3 complete item occurrences; it never claims improvement from one sparse sample.
- [ ] AC-5: Practice again generates a new attempt; dismiss/repeat/return routes work entirely by keyboard.
