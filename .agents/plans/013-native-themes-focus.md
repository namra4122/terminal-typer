# Plan 013: Use accessible native themes and Focus

Status: proposed implementation specification. No feature in this document is implemented by the act of writing it. Source: [PRD](../../PRD.md). Planning baseline: 2026-09-26 working tree, including existing uncommitted `src/test.go`, `src/test_test.go`, and `src/tt.go` changes. Reinspect those files before implementation and preserve unrelated work.

## Goal and user job

Users can preview native/existing themes and enable a focused, accessible layout at every supported size.

As a terminal-heavy developer, I can exercise this capability entirely from the keyboard while keeping my input and progress local. This phase is one reviewable PR, estimated at 1-3 focused implementation sessions. The file table is the implementation boundary, including tests and public documentation. Generated outputs are declared separately.

## Decisions and assumptions

- Native semantic roles are pinned below; contrast is measured, not claimed from seed colors. Terminal-default theme respects emulator colors and uses shape indicators when color is absent.
- Use Lip Gloss rendering and semantic hexadecimal tokens, not a new UI framework. Keep existing six-key theme files valid. Full collection search uses resource origins, with native/terminal-default/Gruvbox-dark/Dracula/Nord/high-contrast curated first.
- Focus keeps prompt/caret/time-or-progress plus a compact `Ctrl-K commands` hint; Ctrl-K/Settings can disable Focus. Optional WPM/error chrome disappears in Focus. No font/image/hardware-key controls.
- Go, Bubble Tea v2, Lip Gloss v2, the existing Makefile, and local-only application data are inherited from plan 001. No server, account, telemetry, executable plugins, or release action is part of this phase.

## Scope

- Add tt-dark/tt-light resources, semantic native styles, searchable theme selection, temporary preview and save-on-confirm. Appearance preview cancel restores rendering and saved bytes.
- Add Focus, showErrors, reducedMotion and color/ASCII fallback controls through the existing version2 appearance shape. Retain six live Settings controls and active/saved CLI override labels.
- Use text/cell-width-aware prompt mapping, clipped long text and scrollable dense screens. Accent orange indicates selection/caret/progress/PB; ordinary headings/rules remain neutral.
- Add full/compact/too-small snapshot fixtures across typing, result, analysis, progress, configuration, settings and help, including CJK/combining text. Theme changes never restart or alter timing.

## Explicitly deferred scope

- Tape/mouse/custom keybindings/more sounds: post-v1 or plan 014.
- Physical keyboard visualizations/heatmaps: post-v1.

## Requirement coverage

DESIGN-01 through DESIGN-09, DESIGN-11; Section 8.1; UI-01, UI-08, UI-10, UI-11; CFG-05, CFG-07; SESSION-09.

Every listed requirement is checked by the acceptance criteria and the named fixtures below. Shared IDs can have incremental coverage in several plans. The index maps the final release gates.

## Acceptance criteria

1. Native text/error/muted roles have measured contrast >=4.5:1 against their backgrounds; selection/caret affordances >=3:1, with a non-color shape/weight cue. Record numeric reports.
2. Native names tt-dark/tt-light and all existing valid themes remain selectable/searchable. Cancel preview saves0 bytes; apply saves once without restarting the attempt.
3. At80x24 full and 52x14 compact every tested screen retains selected item/primary result/exit-help controls. At51x14 or52x13 the app pauses recoverably and states the52x14 minimum.
4. Focus hides optional metrics/chrome and preserves prompt/caret/essential progress and command hint. WPM/error defaults remain off.
5. 256/basic/no-color and ASCII fixtures communicate correct/error/extra/missed/selection without relying on color. Emoji are not required for any control.

## Keyboard journey

1. Open Switch theme, search and preview a choice with arrows.
2. Enter applies/saves; Escape cancels preview. Open Settings to toggle Focus/errors/reduced motion.
3. Resize through supported tiers and return to the same context.
4. Open Help to read emulator-controlled font/screen-reader limits and exit controls.

## Verified reusable implementation

- `src/theme.go:10` already names semantic roles, and `src/tt.go:240` parses existing theme keys. Translate roles rather than invent a different per-screen palette.
- `src/layout.go:161` and `src/layout.go:226` already handle cell widths/combining runes; new Charm views reuse the concept with uniseg/LipGloss metrics.
- Plans001/005/006 provide size/pause state, appearance schema and preview transactions.

Current-state citations refer to the inspected baseline, not hypothetical future line numbers. Files introduced by predecessor plans are cited by interface name below.

## Components and file boundary

| File | Change |
| --- | --- |
| `src/presentation.go` (new) | Charm styles, theme adapter, Focus/capability/layout rendering. |
| `src/presentation_test.go` (new) | Contrast, size/tier and Unicode/no-color snapshots. |
| `themes/tt-dark` (new), `themes/tt-light` (new) | Native source assets with legacy-compatible keys plus semantic roles. |
| `src/app.go` (plan 001) | Route all Charm views through presentation and new live controls. |
| `src/config.go` (plan 005) | Validate theme/capability preferences. |
| `README.md:83`, `man.md` | Theme/Focus/fallback/emulator-controlled behavior. |

## Data model and constraints

| Role | tt-dark | tt-light |
| --- | --- | --- |
| background | #1a1a1a | #fafafa |
| primary/correct | #fafafa | #1a1a1a |
| muted/pending | #a3a3a3 | #666666 |
| accent/caret | #f35815 | #b9400a |
| error | #ff8080 | #b42318 |
| warning/missed | #ffd080 | #815600 |
| border/subtle | #666666 | #b3b3b3 |
| selectedBackground | #3a2a22 | #eee6dd |

Borders are decorative; essential selection uses primary text+inverse/marker. Native theme files expose bgcol/fgcol/hicol/hicol2/hicol3/errcol for old parser compatibility and additional named roles for Charm. `PresentationPreferences` adds optional `colorMode:string(auto/none/basic/256/truecolor)` and `ascii:bool`; missing means auto/false. Auto honors `NO_COLOR` and terminal capability, with no capability query blocking readiness. Invalid explicit theme returns a named error before replacement; unavailable saved theme falls back to native dark with a warning.
Extra characters use `+` marker and inverse background; incorrect uses underline; missed analysis uses `missing` labels. Width computation uses terminal-selected grapheme mode from Charm where available, with deterministic fallback for snapshot fixtures.
`AppView` is a read-only projection `{Screen:string, Session:*Session, Result:*SessionResult, Navigation:Navigation, Analysis:*AnalysisView, Progress:*ProgressView, History:*HistoryPage, Config:Configuration, Message:string}`. Views read only their relevant member, and nil members produce an explicit unavailable state rather than panic.


## Go and CLI contracts

```go
func LoadPresentationTheme(name string) (PresentationTheme, error)
func ResolveTier(width, height int) string // full, compact, too-small
func RenderApp(view AppView, prefs PresentationPreferences, width, height int) tea.View
```
`PresentationTheme` contains the exact roles in the table as hex strings or terminal-default colors. `AppView` is the existing root screen/session/result/navigation projection, not a second mutable application model. Legacy theme mapping: bgcol background,fgcol primary,hicol correct,hicol2 accent,hicol3 warning,errcol error; muted/border derived from fgcol with terminal fallback, preserving supplied colors. ReducedMotion true skips the 200 ms completion presentation transition immediately; retain fresh-activation gating so a buffered key still cannot skip Results. Contrast failures disable a native token change at verification, not a user's custom theme; custom themes show capability/contrast warning without being rewritten.
Full typing layout: brand/context at row1, prompt viewport width min(width-16,80), centered horizontally, max6 visible lines centered in remaining height, progress row two rows beneath it, footer at height-2. Compact: margins2 cells, max4 prompt lines, context row0, essential progress at height-3, command hint at height-1. Keep caret's current line visible. Dense full views use margins4, heading at row1, tab row3, body row5 through height-4, footer at height-2. Results primary WPM/accuracy/consistency occupy the top3 body rows; timeline consumes next7 rows, then details/actions. At compact size use vertically scrollable summary rows without charts. Selected rows use a leading ASCII > plus inverse/weight, not orange alone. Configuration uses left section list width14 and right controls at full size, horizontal section tabs at compact size. Focus removes brand/context/footer hints except one compact commands hint, leaving prompt and essential progress. Existing long text scrolls vertically; never introduce horizontal clipping of typed state.


There are no HTTP APIs in this phase unless an explicit network contract appears above. Return errors rather than panicking across the application boundary. Bubble Tea commands perform I/O and report immutable messages to the model. `Update` owns mutable UI/session state and does not wait on I/O.

## Dependencies and operation sequence

Required predecessor plans: 12

1. Load source theme and resolve capabilities, then construct shared styles before rendering screens. Avoid per-screen hardcoded colors.
2. Picker preview copies appearance and applies render-only tokens. Confirm commits appearance, while explicit CLI theme/weight/cursor remains active for that invocation and saved values remain future-only.
3. On resize remove optional live metrics, secondary hints, extra spacing/rules, then secondary metadata. Replace charts by summaries and scroll remaining content. Never clip the canonical prompt or change scalar cursor.
4. Too-small uses the existing pause reason and recovers the prior overlay. Help/settings accessibility pauses follow the same outermost policy.

## Non-functional and cross-cutting rules

- Typing input processing performs no filesystem, audio-device, or network waits. No input event is discarded to catch up with rendering. Coalesce redraws, never accepted typing events.
- Use the same active clock for input, metrics, timer, and charts. Application overlays pause once at the outermost entry and resume once after the last blocking view closes.
- Authenticate nothing: this is a local single-user executable. Validate all new input at its boundary and show path/resource/action-specific errors. Local diagnostics contain no typing text unless the user explicitly requested it.
- Network rate limiting belongs exclusively to plan 016. History indexing and concurrency belong to plan 003. This phase consumes those policies only when they are prerequisites.
- Measure ready-screen <=250 ms, event-to-visible-input <=50 ms, and state-preserving resize <=100 ms on the documented reference setup when this phase changes those paths. Use 100 samples and report p50, p95, p99, and maximum. Slow terminal behavior is recorded rather than described as passing.

## Failure modes and required handling

| Path and failure | Handling and caller visibility | Required coverage |
| --- | --- | --- |
| Low contrast native role | Numeric contrast fixture rejects change; fallback indicators visible | +6 role checks |
| Custom theme breaks rendering | Validate six legacy hex keys, preserve original, fallback warning for saved choice | +3 theme cases |
| Wide/combining text corrupts caret | Grapheme/cell mapping and clipped viewport | +5 Unicode snapshots |
| Preview becomes saved/overrides CLI | Separate draft/active/saved preferences and transaction | +3 preview cases |

The coverage above is required new coverage, not a claim that these tests exist today. No silent, unhandled failure in a changed path satisfies this phase.

## Test plan

| Layer | Named coverage | Minimum new cases |
| --- | --- | --- |
| Unit | Contrast, role mapping, capability tiers and Unicode | 18 |
| Integration | Preview/save/CLI overrides and paused resize | 6 |
| E2E | Screen snapshot matrix across 3 tiers/4 capability profiles | 12 |

Use deterministic seeds, clocks, and temporary data directories. E2E cases mean scripted terminal/model journeys with real generation and storage where used, plus native-terminal inspection where capabilities cannot be simulated. Run `GOCACHE=/private/tmp/terminal-typer-go-cache make verify`, `GOCACHE=/private/tmp/terminal-typer-go-cache make smoke`, and `git diff --check`. Update `README.md` and `man.md` together when behavior becomes public. Regenerate with `make assets` when the manual or source assets change and verify `gzip -t tt.1.gz`. Do not edit `src/packed.go` by hand.

## Rollout and rollback

Regenerate assets/manual. Appearance additions remain optional in Configuration v2; old readers retain required fields. Rollback reverts native resources/presentation and defaults to a valid existing theme without deleting saved/history data. Do not claim custom theme contrast or screen-reader behavior beyond tested emulator capabilities.

Release, deployment, publishing binaries, and uploading user data require separate explicit human authorization. Update `CHANGELOG.md` when a release occurs, not while this proposal is written.

## Definition of done

- [ ] AC-1: Native text/error/muted roles have measured contrast >=4.5:1 against their backgrounds; selection/caret affordances >=3:1, with a non-color shape/weight cue. Record numeric reports.
- [ ] AC-2: Native names tt-dark/tt-light and all existing valid themes remain selectable/searchable. Cancel preview saves0 bytes; apply saves once without restarting the attempt.
- [ ] AC-3: At80x24 full and 52x14 compact every tested screen retains selected item/primary result/exit-help controls. At51x14 or52x13 the app pauses recoverably and states the52x14 minimum.
- [ ] AC-4: Focus hides optional metrics/chrome and preserves prompt/caret/essential progress and command hint. WPM/error defaults remain off.
- [ ] AC-5: 256/basic/no-color and ASCII fixtures communicate correct/error/extra/missed/selection without relying on color. Emoji are not required for any control.
