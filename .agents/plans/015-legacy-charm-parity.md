# Plan 015: Run legacy custom input through Charm

Status: proposed implementation specification. No feature in this document is implemented by the act of writing it. Source: [PRD](../../PRD.md). Planning baseline: 2026-09-26 working tree, including existing uncommitted `src/test.go`, `src/test_test.go`, and `src/tt.go` changes. Reinspect those files before implementation and preserve unrelated work.

## Goal and user job

Existing file, stdin, resource and scripting invocations use the Charm application with preserved CLI contracts.

As a terminal-heavy developer, I can exercise this capability entirely from the keyboard while keeping my input and progress local. This phase is one reviewable PR, estimated at 1-3 focused implementation sessions. The file table is the implementation boundary, including tests and public documentation. Generated outputs are declared separately.

## Decisions and assumptions

- The cutover covers every registered flag and source. Remove migration dispatch switches once parity passes. Charm owns every interactive screen; no two terminal owners run together. Existing unused tcell drawing helpers/types may remain compiled during this bounded PR; removing dead legacy helpers is a separate maintenance task, not a prerequisite to the user's Charm TUI.
- Keep basic invocation JSON fields/CSV rows/order/types. Under the approved metric change, wpm/cpm use metric-v1 correct-retained speeds (integers truncated toward0) and accuracy uses attempted-input accuracy. Document examples of changed correction semantics. Rich export remains separate.
- Baseline -v writes version to stderr and exits1. Preserve that contract rather than silently normalizing it. Capture the current -help behavior in a fixture. Ctrl-C exits1; one-shot normal completion exits0.
- Go, Bubble Tea v2, Lip Gloss v2, the existing Makefile, and local-only application data are inherited from plan 001. No server, account, telemetry, executable plugins, or release action is part of this phase.

## Scope

- Adapt existing source closures for words/quotes/files/stdin, groups/progress/exhaustion/resource-through-dash/raw-multi and cached previous/next/retry navigation.
- Preserve every flag syntax and source precedence, report suppression/one-shot, list/help/version, theme/sound lookup and maximum line width. No remembered timer/modifier leaks into legacy input.
- Route TUI/ANSI output to controlling terminal or stderr separately from structured stdout. Stdin source is fully read before opening controlling terminal input; never reuse exhausted piped stdin for keyboard events.
- Apply new private history policy, rich metrics/eligibility, state-preserving resize and restart safeguard. Preserve legacy mistake store and explicit mistake output while new history respects private consent.
- Register CLI equivalents in Help and remove temporary TT_UI switch documentation.

## Explicitly deferred scope

- New headless/unattended mode, CLI presets, completions, history import: post-v1.
- Dead tcell helper cleanup has no promised runtime feature and follows as bounded maintenance after parity.

## Requirement coverage

CLI-01 through CLI-07; MODE-05; SESSION-01 through SESSION-10; DATA-05 through DATA-07, DATA-13; UI-12 compatibility help; A-13.

Every listed requirement is checked by the acceptance criteria and the named fixtures below. Shared IDs can have incremental coverage in several plans. The index maps the final release gates.

## Acceptance criteria

1. All matrix fixtures listed below pass with Charm as the sole terminal owner; no unsupported-source fallback remains.
2. Raw wins over multi, paragraph/source exhaustion and -start0/explicit paragraph offsets match the baseline. Retry has exact canonical prompt text; resize changes0 typed/correction state.
3. JSON parses with exact existing wpm/cpm/accuracy/timestamp/mistakes fields and types. CSV preserves test/mistake row shapes and order; quoting handles commas/newlines without extra decoration. Consent/notices/colors never enter structured stdout.
4. One-shot/report suppression/list/help/version preserve exit/output contracts and trigger0 release checks. Explicit machine flags retain interactive input semantics.
5. Private paths/text appear only in explicit legacy outputs/stores or current UI, never default rich history. Runtime errors restore terminal and identify affected source/remedy.

## Keyboard journey

1. Run a legacy CLI invocation and inspect resolved source/length context.
2. Type/correct/resize, use Ctrl-P, navigate previous/next paragraphs and retry.
3. Complete in normal or one-shot/no-report mode; parse structured stdout separately from terminal UI.
4. Relaunch the same file and verify saved progress and explicit -start behavior.

## Verified reusable implementation

- `src/test.go:102` already dispatches to all generators. `src/datatest.go:3` enforces raw-before-multi; `src/filetest.go:8` supplies paragraph progress semantics.
- `src/tt.go:433` defines all flags; `src/tt.go:63` defines basic JSON/CSV; `src/util.go:201` defines resource lookup; `src/tt.go:336` writes legacy mistakes.
- Plans001-014 provide the real Charm session/config/history/navigation/presentation/audio implementations.

Current-state citations refer to the inspected baseline, not hypothetical future line numbers. Files introduced by predecessor plans are cited by interface name below.

## Components and file boundary

| File | Change |
| --- | --- |
| `src/tt.go:401` | Use one Charm entrypoint after noninteractive early exits; separate serializer/terminal cleanup. |
| `src/test.go:102` | Origin-aware adapters for legacy closures and groups. |
| `src/filetest.go:8` | Error-returning safe progress writes preserving position semantics. |
| `src/datatest.go:3` | Retain raw/multi/exhaustion semantics through canonical mappings. |
| `src/app.go` (plan 001) | All-source routing, raw/long text and output isolation. |
| `src/compatibility_test.go` (new) | Flag/source/output/PTY golden matrix. |
| `README.md:54`, `man.md` | Final Charm stack, approved behavior differences and CLI contracts. |

## Data model and constraints

Reuse TestConfig fields exactly, including legacy wordsPerGroup/groups/raw/multi/startParagraph. Extend Test with in-memory ResourceOrigin and canonical scalar-to-cell mapping; never persist its private SourceID path.
Legacy file state stays JSON map absolute-file-path:string -> paragraph-index:int in `.db`; preserve existing zero-based saved cursor/-start behavior by golden fixtures. Make writes safe under its own lock and atomic commit; corrupt/unknown data remains intact with explicit progress-unavailable warning and session-only progress, rather than silently erasing it.
Legacy mistakes stay []mistake `{word:string,typed:string}` in `.errors`; preserve original on malformed/read failure and report that additional mistakes were not saved. New automatic history uses the approved separate privacy policy. Do not fabricate old sessions.
The baseline saved file cursor is the last paragraph index generated, not a promise to skip completed paragraphs. `-start 0` starts the first paragraph, `-start 3` starts the fourth paragraph (zero-based offset), and one-shot exit before requesting another paragraph preserves that last generated cursor. Keep these concrete behaviors.

Basic JSON remains []result `{wpm:int,cpm:int,accuracy:float64,timestamp:int64(seconds),mistakes:[]mistake}`. No new fields. Accuracy unavailable exports0 to preserve numeric type, with short/zero-duration explanation in interactive Results. Empty mistakes is[]; capture and retain existing zero-result JSON semantics (null) and no-row CSV.

## Go and CLI contracts

```go
func RunInvocation(cfg TestConfig, generator func() *Test, output OutputOptions) ([]result,int,error)
type OutputOptions struct { JSON, CSV, OneShot, NoReport bool }
func LegacyResult(r SessionResult, mistakes []mistake) result
```
Compatibility fixtures include bare/saved launches; `-n10 -g5`; `-t30`; all six live flags with explicit false where supported; -w40; -notheme/-theme custom; -quotes en/custom/-; -words fr/custom/-; stdin plain/multi/raw/raw+multi; positional file and -start0/-start3; words+quotes+stdin+file precedence; -sound/-error-sound WAV/MP3; -oneshot/-noreport; -json/-csv/both; -list words/quotes/themes/sounds; -help/-v; custom text containing commas/newlines/Unicode; empty stdin/quote list/one-token word list.
On Unix use controlling terminal reader for UI (not piped stdin), writing UI to terminal or stderr. On Windows use supported console handles through Charm, respecting redirected stdout. If there is no interactive terminal, return a clean error on stderr without ANSI/consent or an invented headless test. JSON/CSV flags alone do not disable input.

There are no HTTP APIs in this phase unless an explicit network contract appears above. Return errors rather than panicking across the application boundary. Bubble Tea commands perform I/O and report immutable messages to the model. `Update` owns mutable UI/session state and does not wait on I/O.

## Dependencies and operation sequence

Required predecessor plans: 14

1. Perform help/list/version early exits before consent/update logic. Resolve explicit flags/input through plan 005 matrix, read source resources/stdin once, then select controlling terminal handles.
2. Drive source closures with real source exhaustion and progress semantics. Cache generated Test instances for navigation/retry. Keep raw bytes/newlines intact in canonical text; legacy structural newlines/tabs display/auto-advance as baseline fixtures dictate and never become artificial metric attempts.
3. Finish using metric-v1, save allowed rich history asynchronously and legacy mistake store with visible error handling, serialize compatible invocation results only after terminal cleanup. CSV uses encoding/csv with original field order.
4. Cut over only after PTY fixtures compare baseline source selection/prompts/progress/output. Approved defaults/accuracy/restart/resize differences use explicit new expected fixtures rather than being treated as regressions.

## Non-functional and cross-cutting rules

- Typing input processing performs no filesystem, audio-device, or network waits. No input event is discarded to catch up with rendering. Coalesce redraws, never accepted typing events.
- Use the same active clock for input, metrics, timer, and charts. Application overlays pause once at the outermost entry and resume once after the last blocking view closes.
- Authenticate nothing: this is a local single-user executable. Validate all new input at its boundary and show path/resource/action-specific errors. Local diagnostics contain no typing text unless the user explicitly requested it.
- Network rate limiting belongs exclusively to plan 016. History indexing and concurrency belong to plan 003. This phase consumes those policies only when they are prerequisites.
- Measure ready-screen <=250 ms, event-to-visible-input <=50 ms, and state-preserving resize <=100 ms on the documented reference setup when this phase changes those paths. Use 100 samples and report p50, p95, p99, and maximum. Slow terminal behavior is recorded rather than described as passing.

## Failure modes and required handling

| Path and failure | Handling and caller visibility | Required coverage |
| --- | --- | --- |
| Piped stdin consumed as keyboard | Read source first, open controlling terminal; clean no-TTY error | +4 PTY source cases |
| Progress/mistake write corrupts legacy file | Atomic writes/locks, preserve damaged file; visible unsaved warning | +4 fault cases |
| ANSI/update notice contaminates output | Separate terminal/stderr and stdout serialization | +4 parser cases |
| Source/timeout precedence changes | Visited-flag matrix with conflicting saved config | +12 resolution fixtures |
| Raw cell wrap alters prompt | Canonical map/display-only reflow and newline fixtures | +4 raw/resize journeys |

The coverage above is required new coverage, not a claim that these tests exist today. No silent, unhandled failure in a changed path satisfies this phase.

## Test plan

| Layer | Named coverage | Minimum new cases |
| --- | --- | --- |
| Unit | All flag mapping/output adapter/exit contracts | 25 |
| Integration | Source resources/progress/mistakes/output streams | 12 |
| E2E | PTY invocation matrix (representative paired combinations) | 16 |

Use deterministic seeds, clocks, and temporary data directories. E2E cases mean scripted terminal/model journeys with real generation and storage where used, plus native-terminal inspection where capabilities cannot be simulated. Run `GOCACHE=/private/tmp/terminal-typer-go-cache make verify`, `GOCACHE=/private/tmp/terminal-typer-go-cache make smoke`, and `git diff --check`. Update `README.md` and `man.md` together when behavior becomes public. Regenerate with `make assets` when the manual or source assets change and verify `gzip -t tt.1.gz`. Do not edit `src/packed.go` by hand.

## Rollout and rollback

Final renderer cutover is conditional on the entire compatibility matrix. Remove contributor TT_UI controls, keep exact source formats and legacy paths. Back up legacy progress/mistake files before the first new safe-write operation; never change their schema. Rollback runs the prior renderer build with version2-compatible settings, or restores a chosen verified v1 backup for the historical baseline. Keep history-v1 intact. Regenerate public manual/assets through canonical commands.

Release, deployment, publishing binaries, and uploading user data require separate explicit human authorization. Update `CHANGELOG.md` when a release occurs, not while this proposal is written.

## Definition of done

- [ ] AC-1: All matrix fixtures listed below pass with Charm as the sole terminal owner; no unsupported-source fallback remains.
- [ ] AC-2: Raw wins over multi, paragraph/source exhaustion and -start0/explicit paragraph offsets match the baseline. Retry has exact canonical prompt text; resize changes0 typed/correction state.
- [ ] AC-3: JSON parses with exact existing wpm/cpm/accuracy/timestamp/mistakes fields and types. CSV preserves test/mistake row shapes and order; quoting handles commas/newlines without extra decoration. Consent/notices/colors never enter structured stdout.
- [ ] AC-4: One-shot/report suppression/list/help/version preserve exit/output contracts and trigger0 release checks. Explicit machine flags retain interactive input semantics.
- [ ] AC-5: Private paths/text appear only in explicit legacy outputs/stores or current UI, never default rich history. Runtime errors restore terminal and identify affected source/remedy.
