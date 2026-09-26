# Plan 006: Reach every implemented workflow by keyboard

Status: proposed implementation specification. No feature in this document is implemented by the act of writing it. Source: [PRD](../../PRD.md). Planning baseline: 2026-09-26 working tree, including existing uncommitted `src/test.go`, `src/test_test.go`, and `src/tt.go` changes. Reinspect those files before implementation and preserve unrelated work.

## Goal and user job

A searchable command palette and contextual quick pickers provide a consistent route to every implemented screen.

As a terminal-heavy developer, I can exercise this capability entirely from the keyboard while keeping my input and progress local. This phase is one reviewable PR, estimated at 1-3 focused implementation sessions. The file table is the implementation boundary, including tests and public documentation. Generated outputs are declared separately.

## Decisions and assumptions

- Pin `charm.land/bubbles/v2 v2.2.1` for textinput/list/viewport components: [stable module](https://raw.githubusercontent.com/charmbracelet/bubbles/v2.2.1/go.mod). It inherits the approved Go 1.26 minimum via Bubble Tea. Keep third-party components out of the session reducer.
- Palette search uses case-insensitive substring matching over label+aliases, stable catalog order, max 12 visible rows; no new fuzzy-search dependency. List navigation uses arrows and optional j/k only when no text field is focused.
- Go, Bubble Tea v2, Lip Gloss v2, the existing Makefile, and local-only application data are inherited from plan 001. No server, account, telemetry, executable plugins, or release action is part of this phase.

## Scope

- Ctrl-K opens the palette everywhere except an already-open palette, where it closes. Help remains reachable during typing while literal ? remains prompt text.
- Route implemented mode, length, content, Settings, theme (only available choices), Results, History, Practice, Restart, Next, Help and Quit. Add commands as their owning phase lands; never advertise unavailable actions.
- Pickers preserve origin view, selection and draft. Enter applies test changes through the existing confirmation/Start contract. Theme preview changes only temporary rendering and restores on Cancel.
- Add Help with contextual keys/full keymap, CLI equivalents, privacy/input limits, and back route. All overlays share outermost pause accounting and preserve search text/selection through resize.

## Explicitly deferred scope

- Analysis/Records/Statistics/export/update commands register in plans 010-012/016.
- More theme choices are supplied by plan 013. Custom keybindings/mouse remain post-v1.

## Requirement coverage

UI-08 through UI-13; CFG-08, CFG-09; SESSION-02, SESSION-06; Section 6.2; P-07.

Every listed requirement is checked by the acceptance criteria and the named fixtures below. Shared IDs can have incremental coverage in several plans. The index maps the final release gates.

## Acceptance criteria

1. Every currently implemented screen/action is reachable via Ctrl-K and Enter, with Escape returning to the same originating state.
2. Queries `stats`/`colors` match their implemented actions when registered; no unmatched query invokes an action. `j`, `k`, `/`, and `?` can appear literally in focused search text.
3. Opening nested picker/Help for 3000 ms adds exactly 3000 ms pause once. Returning keeps canonical prompt, typed values and correction history.
4. A replacement cancelled after meaningful input preserves the old attempt. A successful Start uses plan 005's single commit/generate transaction.
5. At 52x14 the selected action, query, essential description and Escape/Ctrl-C controls stay visible.

## Keyboard journey

1. Press Ctrl-K during a test, type a query, navigate results with arrows, and Enter.
2. Use a quick picker or Configure; Cancel returns without saving previews.
3. Open Help from the palette, then Escape through the prior context.
4. Outside text fields, use ? for Help and / to focus list search.

## Verified reusable implementation

- `src/settings.go:242` and `src/settings.go:289` already define declarative row metadata; reuse its labels/value formatters.
- Plan 001 provides pause reasons and root model; plan 005 provides validated drafts, `ResolveLaunch` and `CommitConfiguration`. Existing `src/tt.go:274` usage supplies actual CLI equivalents.

Current-state citations refer to the inspected baseline, not hypothetical future line numbers. Files introduced by predecessor plans are cited by interface name below.

## Components and file boundary

| File | Change |
| --- | --- |
| `go.mod:5`, `go.sum` | Add pinned Bubbles. |
| `src/navigation.go` (new) | Command registry, context stack, palette/picker/help state. |
| `src/navigation_test.go` (new) | Key focus, registry and pause journeys. |
| `src/app.go` (plan 001) | Dispatch to routing and registration hooks. |
| `src/app_test.go` (plan 001) | Confirm/cancel/context recovery tests. |
| `README.md:69`, `man.md` | Keyboard map and supported commands. |

## Data model and constraints

```go
type Command struct { ID, Label string; Aliases []string; Enabled bool; DisabledReason string; Route string }
type NavigationFrame struct { Screen, Origin, Query string; Selected int; Draft *Configuration }
type Navigation struct { Stack []NavigationFrame; Commands []Command }
```
Routes use stable IDs `change-mode`, `change-length`, `select-content`, `configure`, `settings`, `theme`, `results`, `history`, `practice`, `restart`, `next`, `help`, `quit`. Register additive IDs `analysis`,`statistics`,`records`,`export-history`,`updates` only when implemented. Aliases: statistics=stats,trends; theme=colors,colours; configure=config; help=keys. Registry is in memory. Persist discovered hint IDs through Configuration. Do not persist search queries.
`AppContext` is `{Session:*Session, Result:*SessionResult, Config:Configuration, HasHistory:bool, CanPractice:bool, Capabilities:map[string]bool}`. Capabilities use the registry IDs as keys, supplied by actually linked phase implementations. Navigation frames use root screen IDs typing/results/practice-review/practice-results/history/analysis/progress/configure/palette/picker/settings/help/confirm/error; only implemented IDs route successfully.


## Go and CLI contracts

```go
func RegisteredCommands(ctx AppContext) []Command
func SearchCommands(commands []Command, query string) []Command
func (n *Navigation) Push(frame NavigationFrame)
func (n *Navigation) Pop() (NavigationFrame, bool)
```
`AppContext` supplies current session/result/config and capability booleans. Disabled actionable conditions may show a reason, but never route to unfinished code. Route strings are a validated closed enumeration; unknown routes return `ErrUnknownRoute` and preserve context. Root model forwards text to a focused textinput before evaluating letter shortcuts. Ctrl-C exits globally. Ctrl-L requests redraw globally. Escape saves/closes Settings according to its separate contract, cancels other previews, and never restarts a paused typing session accidentally.

There are no HTTP APIs in this phase unless an explicit network contract appears above. Return errors rather than panicking across the application boundary. Bubble Tea commands perform I/O and report immutable messages to the model. `Update` owns mutable UI/session state and does not wait on I/O.

## Dependencies and operation sequence

Required predecessor plans: 5

1. Build registry from actual capabilities and context, push origin frame, and add one navigation pause reason.
2. Search filters labels/aliases without mutating registry. On Enter dispatch the selected route; zero matches retain palette with `No matching commands`.
3. Picker drafts are copied; Cancel pops without persistence, Apply delegates to plan 005 Start/appearance transaction. Help pushes a nested frame.
4. Pop restores exact query/selection/view and removes the navigation pause reason only when the outermost navigation frame closes.

## Non-functional and cross-cutting rules

- Typing input processing performs no filesystem, audio-device, or network waits. No input event is discarded to catch up with rendering. Coalesce redraws, never accepted typing events.
- Use the same active clock for input, metrics, timer, and charts. Application overlays pause once at the outermost entry and resume once after the last blocking view closes.
- Authenticate nothing: this is a local single-user executable. Validate all new input at its boundary and show path/resource/action-specific errors. Local diagnostics contain no typing text unless the user explicitly requested it.
- Network rate limiting belongs exclusively to plan 016. History indexing and concurrency belong to plan 003. This phase consumes those policies only when they are prerequisites.
- Measure ready-screen <=250 ms, event-to-visible-input <=50 ms, and state-preserving resize <=100 ms on the documented reference setup when this phase changes those paths. Use 100 samples and report p50, p95, p99, and maximum. Slow terminal behavior is recorded rather than described as passing.

## Failure modes and required handling

| Path and failure | Handling and caller visibility | Required coverage |
| --- | --- | --- |
| Letter navigation intercepts search | Focused textinput consumes literal text first | +4 text fixtures |
| Escape restarts behind overlay | Central frame dispatch, visible back route | +3 nested cases |
| Capability registered too early | Registry capability predicates and unknown-route error | +3 catalog cases |
| Nested overlays double-pause | Single root reason across stack | +2 clock cases |

The coverage above is required new coverage, not a claim that these tests exist today. No silent, unhandled failure in a changed path satisfies this phase.

## Test plan

| Layer | Named coverage | Minimum new cases |
| --- | --- | --- |
| Unit | Search aliases, focus classification, registry and stack | 12 |
| Integration | Configuration, Settings, nested pause and confirmations | 5 |
| E2E | Keyboard-only action audit | 2 |

Use deterministic seeds, clocks, and temporary data directories. E2E cases mean scripted terminal/model journeys with real generation and storage where used, plus native-terminal inspection where capabilities cannot be simulated. Run `GOCACHE=/private/tmp/terminal-typer-go-cache make verify`, `GOCACHE=/private/tmp/terminal-typer-go-cache make smoke`, and `git diff --check`. Update `README.md` and `man.md` together when behavior becomes public. Regenerate with `make assets` when the manual or source assets change and verify `gzip -t tt.1.gz`. Do not edit `src/packed.go` by hand.

## Rollout and rollback

Replace Ctrl-K's temporary Configure shortcut with the documented palette. Persist only hint discovery through existing v2 schema. Revert this PR to recover the direct Configure route, leaving test/history/settings intact. No new migrations or network access.

Release, deployment, publishing binaries, and uploading user data require separate explicit human authorization. Update `CHANGELOG.md` when a release occurs, not while this proposal is written.

## Definition of done

- [ ] AC-1: Every currently implemented screen/action is reachable via Ctrl-K and Enter, with Escape returning to the same originating state.
- [ ] AC-2: Queries `stats`/`colors` match their implemented actions when registered; no unmatched query invokes an action. `j`, `k`, `/`, and `?` can appear literally in focused search text.
- [ ] AC-3: Opening nested picker/Help for 3000 ms adds exactly 3000 ms pause once. Returning keeps canonical prompt, typed values and correction history.
- [ ] AC-4: A replacement cancelled after meaningful input preserves the old attempt. A successful Start uses plan 005's single commit/generate transaction.
- [ ] AC-5: At 52x14 the selected action, query, essential description and Escape/Ctrl-C controls stay visible.
