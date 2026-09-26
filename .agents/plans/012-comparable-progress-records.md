# Plan 012: Compare progress and personal records

Status: proposed implementation specification. No feature in this document is implemented by the act of writing it. Source: [PRD](../../PRD.md). Planning baseline: 2026-09-26 working tree, including existing uncommitted `src/test.go`, `src/test_test.go`, and `src/tt.go` changes. Reinspect those files before implementation and preserve unrelated work.

## Goal and user job

Progress shows comparable trends and reproducible personal records from surviving eligible regular history.

As a terminal-heavy developer, I can exercise this capability entirely from the keyboard while keeping my input and progress local. This phase is one reviewable PR, estimated at 1-3 focused implementation sessions. The file table is the implementation boundary, including tests and public documentation. Generated outputs are declared separately.

## Decisions and assumptions

- Record grouping is exact mode+configured length+packId+packRevision+metricVersion+modifiers+Normal+skipWord+allowBackspace. Count uses configured count; timed uses configured duration; quotes use short/medium/long length band, pack revision and same modifiers/controls, not passage ID.
- Only fresh completed/expired official embedded catalog sessions with normal skip/backspace rules (both enabled) qualify. Exclude private/custom/local-approved input, retries/practice, non-default correction restrictions, missing/unreviewed catalog provenance, or incompatible metric version. Show every reason.
- PB compares full-precision WPM, then accuracy. Equal WPM/accuracy ties retain the earliest timestamp then lowest ID as incumbent. Mark ties without improvement. Deleting an incumbent recomputes from surviving eligible history.
- Go, Bubble Tea v2, Lip Gloss v2, the existing Makefile, and local-only application data are inherited from plan 001. No server, account, telemetry, executable plugins, or release action is part of this phase.

## Scope

- Add Records grouped by eligible comparison key, incumbent details and prior best improvement. Results computes PB only after successful durable save and shows Saved/PB or unavailable-on-failed-save.
- Add Trends for 7/30/90/all ranges: effective WPM, accuracy, consistency, volume, recent PBs, weak words/pairs. Default regular only; comparison group explicitly selected, never average unlike test lengths/modes silently.
- Display sample count/range; fewer than 5 comparable sessions shows insufficient trend evidence but keeps individual values and volume visible. Practice tab remains separate and makes no ordinary PB claim.
- Register statistics/stats/trends and records commands. Refresh all projections on History generation changes.

## Explicitly deferred scope

- Difficulty/layout/tag/preset records and cloud leaderboards: post-v1.
- PB sound is wired through plan 014.

## Requirement coverage

UI-05, UI-06; PB-01 through PB-05; METRIC-04, METRIC-06; DATA-03, DATA-09; UI-02 PB comparison.

Every listed requirement is checked by the acceptance criteria and the named fixtures below. Shared IDs can have incremental coverage in several plans. The index maps the final release gates.

## Acceptance criteria

1. 15s and 30s, different revisions/modifiers/controls, and quote length bands form separate exact record groups. Ineligible fixtures show explicit reasons.
2. Ties follow timestamp/ID rules and never announce a positive improvement. Deleting the incumbent promotes the correct surviving record and refreshes Trends.
3. A fixture with 4 comparable sessions displays sample count4 and insufficient trend evidence. Mixed incomparable sessions are never silently combined into a WPM trend.
4. Saving a candidate twice gives one record/PB comparison. Failed save keeps Results but shows records unavailable for that result.
5. 10,000 healthy indexed records open Progress within 500 ms on the documented reference setup, with repeated page opens using in-memory cached projections.

## Keyboard journey

1. Open Progress and switch History/Trends/Records tabs with Tab.
2. Choose period and a comparison group, inspect samples and weak-item aggregates.
3. Open a record's Analysis, then return. Delete a record through History and see refreshed records.
4. Complete an eligible fresh test and read restrained `New personal best: N WPM faster` feedback.

## Verified reusable implementation

- `src/tt.go:637` keeps only invocation results today; no persistent PB service exists to reuse.
- Plans003/011 supply typed history/index/generations and deletion. Plan 002 supplies one formula; plan 010 supplies bounded word/pair fragments.

Current-state citations refer to the inspected baseline, not hypothetical future line numbers. Files introduced by predecessor plans are cited by interface name below.

## Components and file boundary

| File | Change |
| --- | --- |
| `src/progress.go` (new) | Eligibility, comparison keys, records and trend projections. |
| `src/progress_test.go` (new) | Ties, grouping/deletion/sparse sample and 10k benchmark. |
| `src/history.go` (plan 003) | Compute eligibility under official-catalog policy before save. |
| `src/app.go` (plan 001) | Progress tabs and post-save PB notice. |
| `src/app_test.go` (plan 001) | Records/trends/refresh journeys. |
| `README.md:81`, `man.md` | Comparable groups, sample limits and PB rules. |

## Data model and constraints

```go
type ComparisonKey struct { Mode, Length, PackID, PackRevision, MetricVersion, Difficulty string; Modifiers TestModifiers; SkipWord, AllowBackspace bool }
type PersonalRecord struct { Key ComparisonKey; SessionID string; WPM, Accuracy float64; PreviousWPM, Improvement *float64; Tie bool }
type TrendPoint struct { DayUTC string; Count int; WPM, Accuracy, Consistency *float64 }
type ProgressView struct { Generation string; Records []PersonalRecord; Points []TrendPoint; SampleCount int; SinceUnixMS int64; Sufficient bool }
```
Use UTC calendar-day buckets, arithmetic mean of non-null individual session values, and explicit count of values for each metric. Missing consistency does not become0. Weak-word/pair aggregates combine permitted observed attempts/errors by exact case-sensitive item within the selected group; require >=3 observations and show totals. Projections are rebuildable in-memory caches keyed by store generation/query, with no authoritative PB file. Persisted eligibilityReasons are recomputable from retained config/privacy/catalog revision; unknown official revision cannot be silently promoted.

## Go and CLI contracts

```go
func Eligibility(r HistoryRecord, official OfficialCatalog) []string
func KeyFor(r HistoryRecord) ComparisonKey
func BuildProgress(records []HistoryRecord, key ComparisonKey, sinceUnixMS int64) ProgressView
```
Reason codes: retry,practice,private-source,custom-source,unapproved-pack,unreviewed-content,restricted-correction,aborted,unknown-metric. Public catalog whitelist includes valid existing embedded word packs and individually reviewed official dialogues; legacy unreviewed `quotes/en` stays available but does not gain approved-dialogue eligibility. Configuration changes alter comparison keys, not historic results. Empty history returns count0/no records. Export `eligible` is recomputed through this same function.

There are no HTTP APIs in this phase unless an explicit network contract appears above. Return errors rather than panicking across the application boundary. Bubble Tea commands perform I/O and report immutable messages to the model. `Update` owns mutable UI/session state and does not wait on I/O.

## Dependencies and operation sequence

Required predecessor plans: 11

1. Filter regular completed/expired records, evaluate reasons, partition exact keys, and select deterministic incumbents from surviving records.
2. Calculate current result comparison against the prior snapshot, commit history, reload changed generation, then show PB only if that result is incumbent. Concurrent newer results can change incumbent; disclose current record rather than stale celebration.
3. Build UTC day series and weakness aggregates for selected group/period, excluding practice unless its separate tab was requested.
4. Deletion invalidates generation-keyed caches and recomputes every affected group; no mutable stored PB rows need repair.

## Non-functional and cross-cutting rules

- Typing input processing performs no filesystem, audio-device, or network waits. No input event is discarded to catch up with rendering. Coalesce redraws, never accepted typing events.
- Use the same active clock for input, metrics, timer, and charts. Application overlays pause once at the outermost entry and resume once after the last blocking view closes.
- Authenticate nothing: this is a local single-user executable. Validate all new input at its boundary and show path/resource/action-specific errors. Local diagnostics contain no typing text unless the user explicitly requested it.
- Network rate limiting belongs exclusively to plan 016. History indexing and concurrency belong to plan 003. This phase consumes those policies only when they are prerequisites.
- Measure ready-screen <=250 ms, event-to-visible-input <=50 ms, and state-preserving resize <=100 ms on the documented reference setup when this phase changes those paths. Use 100 samples and report p50, p95, p99, and maximum. Slow terminal behavior is recorded rather than described as passing.

## Failure modes and required handling

| Path and failure | Handling and caller visibility | Required coverage |
| --- | --- | --- |
| Unlike configs compared | Exact key partitions and explicit group UI | +8 grouping fixtures |
| Tied/deleted PB becomes stale | Rebuild from surviving history and deterministic ties | +4 tie/delete fixtures |
| Failed/concurrent save announces false PB | Compute only from committed refreshed generation | +3 save cases |
| Sparse/missing metric averaged as 0 | Nullable aggregates/value sample counts | +3 sparse cases |

The coverage above is required new coverage, not a claim that these tests exist today. No silent, unhandled failure in a changed path satisfies this phase.

## Test plan

| Layer | Named coverage | Minimum new cases |
| --- | --- | --- |
| Unit | Eligibility, keys/ties/nullable trends/word-pair aggregation | 20 |
| Integration | Saved/deleted/concurrent generation refresh | 6 |
| E2E | Progress tabs and PB delete/recompute | 2 |

Use deterministic seeds, clocks, and temporary data directories. E2E cases mean scripted terminal/model journeys with real generation and storage where used, plus native-terminal inspection where capabilities cannot be simulated. Run `GOCACHE=/private/tmp/terminal-typer-go-cache make verify`, `GOCACHE=/private/tmp/terminal-typer-go-cache make smoke`, and `git diff --check`. Update `README.md` and `man.md` together when behavior becomes public. Regenerate with `make assets` when the manual or source assets change and verify `gzip -t tt.1.gz`. Do not edit `src/packed.go` by hand.

## Rollout and rollback

No authoritative new persistent data; projections derive from versioned history. Revert UI/projection code and retain all records. Existing stored reasons are recomputed under documented policy; changing policy in another release requires a new explicit eligibility/metric revision rather than silent historical reinterpretation.

Release, deployment, publishing binaries, and uploading user data require separate explicit human authorization. Update `CHANGELOG.md` when a release occurs, not while this proposal is written.

## Definition of done

- [ ] AC-1: 15s and 30s, different revisions/modifiers/controls, and quote length bands form separate exact record groups. Ineligible fixtures show explicit reasons.
- [ ] AC-2: Ties follow timestamp/ID rules and never announce a positive improvement. Deleting the incumbent promotes the correct surviving record and refreshes Trends.
- [ ] AC-3: A fixture with 4 comparable sessions displays sample count4 and insufficient trend evidence. Mixed incomparable sessions are never silently combined into a WPM trend.
- [ ] AC-4: Saving a candidate twice gives one record/PB comparison. Failed save keeps Results but shows records unavailable for that result.
- [ ] AC-5: 10,000 healthy indexed records open Progress within 500 ms on the documented reference setup, with repeated page opens using in-memory cached projections.
