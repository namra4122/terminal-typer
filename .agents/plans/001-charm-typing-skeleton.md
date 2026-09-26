# Plan 001: A real typing test on Charm

Status: proposed implementation specification. No feature in this document is implemented by the act of writing it. Source: [PRD](../../PRD.md). Planning baseline: 2026-09-26 working tree, including existing uncommitted `src/test.go`, `src/test_test.go`, and `src/tt.go` changes. Reinspect those files before implementation and preserve unrelated work.

## Goal and user job

An opt-in Charm screen runs a real English word-count test from generation through correction, completion, and a basic result.

As a terminal-heavy developer, I can exercise this capability entirely from the keyboard while keeping my input and progress local. This phase is one reviewable PR, estimated at 1-3 focused implementation sessions. The file table is the implementation boundary, including tests and public documentation. Generated outputs are declared separately.

## Decisions and assumptions

- The user explicitly selected Charm, overriding PRD Section 2's existing-stack recommendation. Keep Go and Makefile. Raise `go.mod` to Go 1.26.0. Pin `charm.land/bubbletea/v2 v2.0.10` and `charm.land/lipgloss/v2 v2.0.6`. Retain `beep v1.1.0` and `tcell v1.4.0` for compatibility paths. Pin `github.com/rivo/uniseg v0.4.7` for grapheme-safe drawing. Bubbles enters in plan 006 at v2.2.1.
- These stable tags and Go requirements were checked on 2026-09-26: [Bubble Tea module](https://raw.githubusercontent.com/charmbracelet/bubbletea/v2.0.10/go.mod), [Lip Gloss module](https://raw.githubusercontent.com/charmbracelet/lipgloss/v2.0.6/go.mod). Do not substitute v1 examples: [v2 Model/View source](https://raw.githubusercontent.com/charmbracelet/bubbletea/v2.0.10/tea.go) defines `View() tea.View`.
- Until plan 005, expose the walking skeleton with `TT_UI=charm ./bin/tt` for contributors. Handle only terminal stdin, no positional file, and no flags except `-n`/`-g`/`-t` and the six live-setting overrides. Other invocations dispatch to the existing renderer. The default route remains unchanged. This reversible migration switch is not a new CLI flag.
- Meaningful input means at least 5 accepted scalar values OR at least 2 seconds active time after first input. Restart requires a second Escape within 1000 ms after meaningful input. The safeguard overlay pauses. Expiry cancels the safeguard and resumes. Pre-input Escape restarts immediately.
- Do not retain abandoned attempts in v1. An attempt is in-memory only until completed/expired. Retry allocates a new attempt ID and retains the exact prompt ID.
- Go, Bubble Tea v2, Lip Gloss v2, the existing Makefile, and local-only application data are inherited from plan 001. No server, account, telemetry, executable plugins, or release action is part of this phase.

## Scope

- Generate using existing `newTestGenerator`; preserve original segments. Render a borderless prompt, mode/count context, caret, time/progress, and hints. Scroll long prompts around the current word.
- Implement a pure session reducer and a Bubble Tea root model. Support character entry, skip/no-skip, Backspace, word deletion, Next/Previous, Ctrl-C, Ctrl-L, and Ctrl-P with the existing six Settings controls and CLI override labels.
- Use full >=80x24, compact >=52x14, and too-small state below either compact threshold. Too-small is an explicit pause reason. Restore exactly the same session and overlay when usable size returns.
- Use a minimal inline native-dark palette: background #1a1a1a, primary #fafafa, muted #a3a3a3, orange #f35815, error #ff8080. Underline/inverse identifies errors without color. Theme search is not part of this slice.

## Explicitly deferred scope

- Rich metrics and Results actions: plan 002.
- Durable history and practice: plans 003-004.
- Bare-launch timed default and saved configuration: plan 005.
- All legacy source/render paths on Charm: plan 015. The compatibility renderer stays usable in the meantime.

## Requirement coverage

P-02, P-03, P-05, P-06, P-07, P-09; CLI-01 through CLI-06 (unchanged default route); SESSION-01 through SESSION-09; UI-01, UI-11, UI-13; Section 8.1.

Every listed requirement is checked by the acceptance criteria and the named fixtures below. Shared IDs can have incremental coverage in several plans. The index maps the final release gates.

## Acceptance criteria

1. `TT_UI=charm tt -n 10 -g 2` produces 20 words in 2 groups, finishes, and shows finite WPM/CPM/accuracy. `tt` without the switch retains the existing 50-word behavior.
2. Before character entry, Settings, Ctrl-L, and navigation keep active duration at 0. A 3-second Settings visit after input adds 3000 ms pause and 0 ms active duration.
3. Skip, backspace, and word deletion match baseline fixtures. A resize 80x24 -> 52x14 -> 40x10 -> 80x24 preserves prompt, cursor, retained input, and correction state.
4. One Escape after meaningful input preserves the attempt. A second within 1000 ms repeats the exact prompt with a new attempt ID. Outside that interval the attempt resumes.
5. Settings save failure keeps the overlay open and leaves prior active/saved values unchanged. Ctrl-C restores terminal state on the Charm route.
6. Every unsupported switch/source combination reaches the legacy path, with its registered flags and existing output unchanged.

## Keyboard journey

1. Set the contributor switch and launch a 10-word test.
2. Type, correct a mistake, open Ctrl-P, save a preference, and resume.
3. Resize through compact and too-small states, then complete.
4. Read the basic result. Enter starts fresh text, Escape returns to retry, and Ctrl-C exits.

## Verified reusable implementation

- `src/test.go:63` resolves legacy input; `src/test.go:102` wraps real generators; `src/test.go:141` copies prompt segments before reflow.
- `src/settings.go:70`, `src/settings.go:133`, `src/settings.go:184`, and `src/settings.go:384` already provide defaults, atomic settings writes, CLI overrides, and dirty-row merge. Reuse these pure functions; replace only screen rendering on the Charm path.
- `src/typer.go:356`, `src/typer.go:481`, and `src/tt.go:632` supply correction, skip, and basic report fixtures. The control-key timing defect at `src/typer.go:436` is fixed only in the new reducer.

Current-state citations refer to the inspected baseline, not hypothetical future line numbers. Files introduced by predecessor plans are cited by interface name below.

## Components and file boundary

| File | Change |
| --- | --- |
| `go.mod:3`, `go.sum` | Pin approved Charm dependencies and Go minimum; preserve compatibility dependencies. |
| `src/tt.go:401` | Add reversible dispatch before terminal initialization. Keep existing flag parsing. |
| `src/app.go` (new) | Root `tea.Model`, typing/result/settings views and terminal ownership. |
| `src/session.go` (new) | Pure session reducer, clock, IDs, pause reasons, and display-to-prompt mapping. |
| `src/app_test.go` (new) | Reducer, screen journey, dispatch and terminal cleanup fixtures. |
| `README.md:23` | Describe Go minimum and contributor migration switch. |
| `man.md` | State the new source-build minimum without changing public launch behavior. |

## Data model and constraints

```go
type SessionState string // ready, running, paused, completed, expired
type InputKind string // text, backspace, delete-word, skip, tick, pause, resume
type SessionInput struct { Kind InputKind; Text string; AtNS int64; Reason string }
type InputEvent struct { Seq uint64; ActiveNS int64; Kind InputKind; Text string; CursorBefore int; CursorAfter int }
type Session struct {
    AttemptID string; PromptID string; RetryOf string; Test *Test
    State SessionState; Cursor int; Typed []rune; Events []InputEvent
    StartedAtNS int64; LastAtNS int64; ActiveNS int64; PauseNS int64
    PauseReasons map[string]bool; RestartUntilNS int64
}
```
IDs are 16 cryptographically random bytes encoded as 32 lowercase hex characters. Use injectable ID/clock functions in tests. `PromptID` is an in-memory identity, not persisted text. `Typed` has one slot per expected scalar, with zero marking skipped slots. Newlines auto-advance for legacy reflow. Display line breaks never change the canonical prompt. Raw layout support is completed in plan 015.
Every accepted input advances `Seq`. `AtNS` is monotonic process time. Before applying any input, settle elapsed time from the previous event. Only printable accepted text starts the clock. Repeated pause reasons do not double-count. Ready/terminal states accrue no active time. Idle running time counts. Too-small plus another overlay resumes only after both reasons clear.

## Go and CLI contracts

```go
func NewSession(test *Test, attemptID, promptID string) *Session
func (s *Session) Apply(in SessionInput) error
func (s *Session) Snapshot() Session // deep copy; no shared slices/maps
func RunCharm(test *Test, saved runtimeSettings, overrides settingsOverrides, flags flagValues) ([]result, int, error)
```
`Apply` rejects decreasing times, invalid text, and unknown input kinds with `ErrInvalidInput`; it ignores edits when paused or terminal. Recognize Ctrl-C/Ctrl-L/Ctrl-P/Escape/arrows before text classification. Literal `?`, `/`, `j`, `k` remain text. Ignore mouse/release events. Handle `tea.KeyPressMsg`, `tea.WindowSizeMsg`, and command replies. Preserve text key order.
`RunCharm` uses `tea.NewView`, `View.AltScreen=true`, native cursor metadata, and returns after terminal restoration. It never writes result JSON/CSV itself. Root process owns exit serialization. At startup or recovery errors, return an error for stderr and restore the terminal. No network operations.

There are no HTTP APIs in this phase unless an explicit network contract appears above. Return errors rather than panicking across the application boundary. Bubble Tea commands perform I/O and report immutable messages to the model. `Update` owns mutable UI/session state and does not wait on I/O.

## Dependencies and operation sequence

Required predecessor plans: None. This starts from the inspected checkout.

1. Parse existing flags, decide renderer, resolve/generate an actual test, then enter Charm. Unhandled route predicates reach existing tcell without initializing two renderers.
2. Use monotonic event time, settle active/pause duration, dispatch controls, mutate session, and derive view. A 25 ms tick is only for expiry/redraw, never the authoritative clock. Clamp expiry to configured duration before processing late input.
3. Settings edits a draft, reloads current disk state, merges dirty rows, saves, and only then updates saved/effective values. Save errors show Retry save / Discard changes; discard is an explicit button and restores the prior session.
4. Completion drains no input into prompt text. Guard result action activation for 200 ms; Ctrl-C remains available. Next/Previous use cached `Test` objects with a new attempt, and navigating to an already attempted prompt marks it retry.
5. Too-small pushes a pause reason and displays required dimensions plus Ctrl-C/Ctrl-L. Expanding restores the prior view and removes only that reason.

## Non-functional and cross-cutting rules

- Typing input processing performs no filesystem, audio-device, or network waits. No input event is discarded to catch up with rendering. Coalesce redraws, never accepted typing events.
- Use the same active clock for input, metrics, timer, and charts. Application overlays pause once at the outermost entry and resume once after the last blocking view closes.
- Authenticate nothing: this is a local single-user executable. Validate all new input at its boundary and show path/resource/action-specific errors. Local diagnostics contain no typing text unless the user explicitly requested it.
- Network rate limiting belongs exclusively to plan 016. History indexing and concurrency belong to plan 003. This phase consumes those policies only when they are prerequisites.
- Measure ready-screen <=250 ms, event-to-visible-input <=50 ms, and state-preserving resize <=100 ms on the documented reference setup when this phase changes those paths. Use 100 samples and report p50, p95, p99, and maximum. Slow terminal behavior is recorded rather than described as passing.

## Failure modes and required handling

| Path and failure | Handling and caller visibility | Required coverage |
| --- | --- | --- |
| Dispatch selects both terminal owners | Initialize exactly one owner and always close it; visible startup error | +3 route fixtures |
| Input reducer sees controls as content | Classify controls first and keep ready time 0; visible only as intended action | +8 reducer cases |
| Resize changes prompt coordinates | Maintain canonical scalar index and recompute cells; minimum-size pause visible | +3 resize journeys |
| Settings disk write fails | Retain draft/open overlay and prior state; show path and Retry/Discard | +2 injected failures |
| Duplicate completion/restart callbacks | Terminal-state latch and new attempt IDs; no duplicate result | +2 lifecycle fixtures |

The coverage above is required new coverage, not a claim that these tests exist today. No silent, unhandled failure in a changed path satisfies this phase.

## Test plan

| Layer | Named coverage | Minimum new cases |
| --- | --- | --- |
| Unit | Timing, control classification, skip/correction, restart, snapshot | 16 |
| Integration | Real generator, dirty settings merge, route/cleanup | 7 |
| E2E | Complete, modal-resume, resize-recover | 3 |

Use deterministic seeds, clocks, and temporary data directories. E2E cases mean scripted terminal/model journeys with real generation and storage where used, plus native-terminal inspection where capabilities cannot be simulated. Run `GOCACHE=/private/tmp/terminal-typer-go-cache make verify`, `GOCACHE=/private/tmp/terminal-typer-go-cache make smoke`, and `git diff --check`. Update `README.md` and `man.md` together when behavior becomes public. Regenerate with `make assets` when the manual or source assets change and verify `gzip -t tt.1.gz`. Do not edit `src/packed.go` by hand.

## Rollout and rollback

Ship opt-in only. The legacy public path remains default, so renderer rollback is to unset `TT_UI` or revert this PR. Existing settings keep their version-1 schema; no new persistent migration occurs. Go minimum remains a documented approved stack change. Regenerate the manual, not unrelated assets, through the canonical target.

Release, deployment, publishing binaries, and uploading user data require separate explicit human authorization. Update `CHANGELOG.md` when a release occurs, not while this proposal is written.

## Definition of done

- [ ] AC-1: `TT_UI=charm tt -n 10 -g 2` produces 20 words in 2 groups, finishes, and shows finite WPM/CPM/accuracy. `tt` without the switch retains the existing 50-word behavior.
- [ ] AC-2: Before character entry, Settings, Ctrl-L, and navigation keep active duration at 0. A 3-second Settings visit after input adds 3000 ms pause and 0 ms active duration.
- [ ] AC-3: Skip, backspace, and word deletion match baseline fixtures. A resize 80x24 -> 52x14 -> 40x10 -> 80x24 preserves prompt, cursor, retained input, and correction state.
- [ ] AC-4: One Escape after meaningful input preserves the attempt. A second within 1000 ms repeats the exact prompt with a new attempt ID. Outside that interval the attempt resumes.
- [ ] AC-5: Settings save failure keeps the overlay open and leaves prior active/saved values unchanged. Ctrl-C restores terminal state on the Charm route.
- [ ] AC-6: Every unsupported switch/source combination reaches the legacy path, with its registered flags and existing output unchanged.
