# Plan 010: Explain a completed session

Status: proposed implementation specification. No feature in this document is implemented by the act of writing it. Source: [PRD](../../PRD.md). Planning baseline: 2026-09-26 working tree, including existing uncommitted `src/test.go`, `src/test_test.go`, and `src/tt.go` changes. Reinspect those files before implementation and preserve unrelated work.

## Goal and user job

Detailed Analysis shows timing, character outcomes, words and pairs with honest unavailable states for historical/private data.

As a terminal-heavy developer, I can exercise this capability entirely from the keyboard while keeping my input and progress local. This phase is one reviewable PR, estimated at 1-3 focused implementation sessions. The file table is the implementation boundary, including tests and public documentation. Generated outputs are declared separately.

## Decisions and assumptions

- Analysis is read-only. No exact historical prompt is stored or reconstructed from private IDs. In-memory analysis may use current session text; historical analysis uses only permitted fragments and catalog metadata.
- Show top5 slowest/fastest complete words with >=100 ms observation time, top5 error words and top5 pairs with >=3 observations. Pair is two adjacent Unicode scalars within a word, preserving case/punctuation. Deep heatmaps and burst scores are unavailable in v1.
- Go, Bubble Tea v2, Lip Gloss v2, the existing Makefile, and local-only application data are inherited from plan 001. No server, account, telemetry, executable plugins, or release action is part of this phase.

## Scope

- Add WPM/raw-WPM/error time series charts and corresponding text summaries. Distinguish interval speed from final retained effective WPM per metric definition.
- Show correct/incorrect/extra/missed, total/corrected/uncorrected errors, active/pause durations, mode/length/pack/modifiers/effective correction controls, outcome, retry/practice status, eligibility reasons and full attribution.
- Add current/historical word and pair diagnostics with sample size, time range and unavailable explanation. Never enlarge retention to fill an empty chart/label.
- Register Analysis route and preserve origin/result/history row when returning. Charts collapse to text in compact layout.

## Explicitly deferred scope

- Long-term pair/symbol drills, burst and keyboard heatmaps: post-v1.
- Aggregated comparable trends/records: plan 012.

## Requirement coverage

UI-03; METRIC-01 through METRIC-06; CONTENT-08; PB-03; DESIGN-03, DESIGN-05; DATA-04, DATA-05.

Every listed requirement is checked by the acceptance criteria and the named fixtures below. Shared IDs can have incremental coverage in several plans. The index maps the final release gates.

## Acceptance criteria

1. Known metric fixtures show the same values on Results and Analysis with explicit final/interval labels; corrected/error outcomes match replay.
2. 80x24 shows three shared-axis charts;52x14 shows text summaries and selectable sections. No chart exceeds its available cell bounds.
3. A private historical record shows aggregates/series but no secret labels, prompt, fragments or incidental path. Missing public catalog revision shows attribution and exact text unavailable.
4. Word/pair rankings use stated thresholds/ties and include sample counts. Sparse series show `not enough samples` rather than invented points.
5. Escape returns to the same Results or History selection, creating0 new result records.

## Keyboard journey

1. Open Analysis from Results or a History row.
2. Use Tab for Summary/Timeline/Words/Details and arrows/PageUp/PageDown to scroll.
3. Read unavailable-field explanations, then Escape returns to the origin.

## Verified reusable implementation

- `src/tt.go:94` supplies current summary report behavior, replaced on Charm by plan 002 Results. `src/layout.go:161` already recognizes cell widths, while new rendering uses the approved Charm APIs.
- Plans002/003 supply Measurements, MetricSample, nullable values and safe history fragments; plan 008 supplies public source metadata.

Current-state citations refer to the inspected baseline, not hypothetical future line numbers. Files introduced by predecessor plans are cited by interface name below.

## Components and file boundary

| File | Change |
| --- | --- |
| `src/analysis.go` (new) | Read-only projections, word/pair rankings and text/charts. |
| `src/analysis_test.go` (new) | Metric equality, thresholds and terminal bounds. |
| `src/metrics.go` (plan 002) | Derive pair observations during replay. |
| `src/history.go` (plan 003) | Persist permitted bounded pair fragments only. |
| `src/app.go` (plan 001) | Analysis navigation, tabs and origin restoration. |
| `src/app_test.go` (plan 001) | Private/history/current journeys. |
| `README.md:81`, `man.md` | Analysis fields/limits and keys. |

## Data model and constraints

```go
type PairObservation struct { Pair string; Attempts, CorrectAttempts, Errors int; ActiveMS int64 }
type AnalysisView struct {
    ResultID string; Metrics Measurements; Source map[string]string
    SlowWords, FastWords, ErrorWords []PracticeItem; Pairs []PairObservation
    Unavailable map[string]string
}
```
`Source` keys are collection,source,title,speaker,provenance,license. Add permitted pair aggregates to existing `fragments` with kind pair. Pair timing is the interval between consecutive accepted scalars for the pair, including idle but excluding pauses; corrections count new observations at the same canonical pair if both positions are entered, and never cross word boundaries. Historical limits remain20 fragments total and 50 observations/item. Allocate10 word slots (top5 error/slow union then fastest unused),10 pair slots. Rank errors descending error rate then observations then lexical item; slow/fast by median ms/scalar then lexical item. Private policy strips these fields unless consent enabled before that attempt.

## Go and CLI contracts

```go
func BuildAnalysis(current *Session, record HistoryRecord, catalog *DialoguePack) AnalysisView
func RenderSeries(samples []MetricSample, width, height int, ascii bool) string
```
`current=nil` uses retained historical data only. A malformed record returns an unavailable view through the history service's error policy. Chart plots use at most min(width-8,60) buckets,5 data rows+axis at full size; resample bucket values as arithmetic mean of interval rates and sum of errors, never change stored data. All three share active-time endpoints. Draw ASCII `*`, `-`, `|`; optional block glyph mode is secondary. Nulls render gaps, no interpolated points. Text summary includes range, count, min/max/mean with units. Charts are local pure rendering, no chart dependency.

There are no HTTP APIs in this phase unless an explicit network contract appears above. Return errors rather than panicking across the application boundary. Bubble Tea commands perform I/O and report immutable messages to the model. `Update` owns mutable UI/session state and does not wait on I/O.

## Dependencies and operation sequence

Required predecessor plans: 8, 4

1. Resolve selected immutable result and permitted metadata, build one projection, and render its tabs.
2. For current sessions derive diagnostics from memory, then intersect with stored privacy policy only when persisting. For historical private sessions leave absent word/pair details unavailable.
3. On resize recompute chart bounds without resampling stored measurements. Compact mode substitutes readable summaries and scrollable sections.
4. Returning pops the navigation frame and restores source row/selection. Deleted/missing result yields `Result no longer available` and returns to refreshed History.

## Non-functional and cross-cutting rules

- Typing input processing performs no filesystem, audio-device, or network waits. No input event is discarded to catch up with rendering. Coalesce redraws, never accepted typing events.
- Use the same active clock for input, metrics, timer, and charts. Application overlays pause once at the outermost entry and resume once after the last blocking view closes.
- Authenticate nothing: this is a local single-user executable. Validate all new input at its boundary and show path/resource/action-specific errors. Local diagnostics contain no typing text unless the user explicitly requested it.
- Network rate limiting belongs exclusively to plan 016. History indexing and concurrency belong to plan 003. This phase consumes those policies only when they are prerequisites.
- Measure ready-screen <=250 ms, event-to-visible-input <=50 ms, and state-preserving resize <=100 ms on the documented reference setup when this phase changes those paths. Use 100 samples and report p50, p95, p99, and maximum. Slow terminal behavior is recorded rather than described as passing.

## Failure modes and required handling

| Path and failure | Handling and caller visibility | Required coverage |
| --- | --- | --- |
| Charts overflow/lose meaning on resize | Pure bounded rendering with text summaries | +4 size fixtures |
| Historical private labels leak | Projection never reads private contentId as text | +3 sentinel cases |
| Sparse rankings imply confidence | Thresholds and sample labels/null fields | +4 evidence cases |
| Deleted record remains selected | Missing-result view and refreshed origin | +1 navigation case |

The coverage above is required new coverage, not a claim that these tests exist today. No silent, unhandled failure in a changed path satisfies this phase.

## Test plan

| Layer | Named coverage | Minimum new cases |
| --- | --- | --- |
| Unit | Projection/formula equality, rankings, pairs, chart buckets | 16 |
| Integration | Retention/catalog-revision/private availability | 5 |
| E2E | Current and historical Analysis return | 2 |

Use deterministic seeds, clocks, and temporary data directories. E2E cases mean scripted terminal/model journeys with real generation and storage where used, plus native-terminal inspection where capabilities cannot be simulated. Run `GOCACHE=/private/tmp/terminal-typer-go-cache make verify`, `GOCACHE=/private/tmp/terminal-typer-go-cache make smoke`, and `git diff --check`. Update `README.md` and `man.md` together when behavior becomes public. Regenerate with `make assets` when the manual or source assets change and verify `gzip -t tt.1.gz`. Do not edit `src/packed.go` by hand.

## Rollout and rollback

Additive pair fragments use the existing v1 kind field. Existing word-only history remains valid and pair views are unavailable for it. Rollback removes Analysis navigation and leaves permitted data intact; no exact text is added to history. Revert the PR without touching stored results.

Release, deployment, publishing binaries, and uploading user data require separate explicit human authorization. Update `CHANGELOG.md` when a release occurs, not while this proposal is written.

## Definition of done

- [ ] AC-1: Known metric fixtures show the same values on Results and Analysis with explicit final/interval labels; corrected/error outcomes match replay.
- [ ] AC-2: 80x24 shows three shared-axis charts;52x14 shows text summaries and selectable sections. No chart exceeds its available cell bounds.
- [ ] AC-3: A private historical record shows aggregates/series but no secret labels, prompt, fragments or incidental path. Missing public catalog revision shows attribution and exact text unavailable.
- [ ] AC-4: Word/pair rankings use stated thresholds/ties and include sample counts. Sparse series show `not enough samples` rather than invented points.
- [ ] AC-5: Escape returns to the same Results or History selection, creating0 new result records.
