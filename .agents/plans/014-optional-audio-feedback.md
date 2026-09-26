# Plan 014: Enable optional nonblocking feedback

Status: proposed implementation specification. No feature in this document is implemented by the act of writing it. Source: [PRD](../../PRD.md). Planning baseline: 2026-09-26 working tree, including existing uncommitted `src/test.go`, `src/test_test.go`, and `src/tt.go` changes. Reinspect those files before implementation and preserve unrelated work.

## Goal and user job

Users can enable distinct key/error/completion/PB feedback while audio failure never interrupts typing.

As a terminal-heavy developer, I can exercise this capability entirely from the keyboard while keeping my input and progress local. This phase is one reviewable PR, estimated at 1-3 focused implementation sessions. The file table is the implementation boundary, including tests and public documentation. Generated outputs are declared separately.

## Decisions and assumptions

- Reuse existing beep playback and nk-cream resource. New completion/PB sound choices initially use the existing resource with clear labels; new licensed sound assets are outside this phase. Sounds remain off by default.
- Worker queue capacity32, maximum8 simultaneous streamers, and drop only excess sound jobs. Audio drops/errors never drop typing input. Decode and device initialization run in a command before playback, never the session reducer.
- Go, Bubble Tea v2, Lip Gloss v2, the existing Makefile, and local-only application data are inherited from plan 001. No server, account, telemetry, executable plugins, or release action is part of this phase.

## Scope

- Add individually enabled key/error/completion/PB sound selections and preview in Sound configuration. Failed preview keeps settings unchanged and explains resource/device issue.
- Honor explicit -sound/-error-sound as active invocation overrides while displaying saved future values. Preserve WAV/MP3 name/extension and resource lookup behavior.
- Wire brief 200 ms completion feedback (or immediate reduced-motion), and PB feedback only after committed eligible improvement. No animation blocks navigation/input.
- Surface unavailable audio once per invocation with quiet status; allow retry/disable. Close device on every exit path.

## Explicitly deferred scope

- Additional sound packs, volume/equalizers, arbitrary executable audio handlers: post-v1.

## Requirement coverage

DESIGN-10, DESIGN-11; PB-04; CLI-04 sound compatibility; PERF-06 audio path; CFG-05, UI-08 Sound.

Every listed requirement is checked by the acceptance criteria and the named fixtures below. Shared IDs can have incremental coverage in several plans. The index maps the final release gates.

## Acceptance criteria

1. Fresh settings play0 sounds. Each enabled event triggers only its matching resource, with error fallback to key sound following existing CLI behavior.
2. Missing/malformed WAV/MP3, unavailable device and worker saturation leave the test completable; no input is lost and visible latency stays <=50 ms in reference measurements.
3. CLI sound values are active-only and TUI save changes future values without overriding active flags.
4. Completion/PB feedback never repeats on replayed callbacks or tied PBs. Reduced-motion completes visual transition immediately.
5. Worker/device close on normal exit, Ctrl-C and startup error without goroutine growth across 100 tests.

## Keyboard journey

1. Open Sound, choose a resource, preview, and confirm or cancel.
2. Start typing and hear selected feedback. Complete and receive optional completion/PB feedback.
3. On audio error, read status and Retry/Disable while keeping the same test.

## Verified reusable implementation

- `src/tt.go:353` already loads WAV/MP3, resamples and buffers; `src/typer.go:145` supplies key/error precedence. Reuse these functions through an asynchronous adapter.
- `src/tt.go:603` currently treats initialization as fatal; the new path reports audio unavailable and continues.
- Plans005/013 define sound strings and reduced motion; plan 012 supplies committed PB notifications.

Current-state citations refer to the inspected baseline, not hypothetical future line numbers. Files introduced by predecessor plans are cited by interface name below.

## Components and file boundary

| File | Change |
| --- | --- |
| `src/audio.go` (new) | Bounded worker/device lifecycle and immutable status events. |
| `src/audio_test.go` (new) | Fake backend, precedence, failure and queue/close tests. |
| `src/tt.go:353` | Expose existing decoding as reusable error-returning loader. |
| `src/app.go` (plan 001) | Sound configuration, event dispatch and quiet errors. |
| `src/app_test.go` (plan 001) | Slow/failing backend typing fixtures. |
| `README.md:83`, `man.md` | Optional feedback and fail-open audio behavior. |

## Data model and constraints

```go
type SoundJob struct { AttemptID, EventID, Kind, Resource string }
type AudioStatus struct { Available bool; Resource, Error string }
type AudioWorker struct { Jobs chan SoundJob; Done chan struct{} }
```
Kind is key/error/completion/pb/preview. Resource is existing local/bundled identifier in memory; saved paths are allowed only for deliberately selected sound preferences, never incidental CLI override paths. History stores no audio resource names because they do not affect measurement. Deduplicate completion/PB jobs by EventID; key events are not deduplicated. Playback buffers are capped at 16 MiB per resource and 8 cached resources LRU; oversized resources show actionable unavailable error.

## Go and CLI contracts

```go
func StartAudioWorker(loader func(string) (*beep.Buffer,error)) *AudioWorker
func (w *AudioWorker) TryPlay(job SoundJob) bool
func (w *AudioWorker) Close() error
```
TryPlay is nonblocking, returnsfalse when saturated/disabled; it never changes session acceptance. A worker initialization error returns an AudioStatus message. Commands load resources and post jobs; root model correlates attempt IDs. Per-key failure does not produce one toast per key. Cancelled previews ignore late load messages. On device failure disable playback for the invocation until explicit Retry.

There are no HTTP APIs in this phase unless an explicit network contract appears above. Return errors rather than panicking across the application boundary. Bubble Tea commands perform I/O and report immutable messages to the model. `Update` owns mutable UI/session state and does not wait on I/O.

## Dependencies and operation sequence

Required predecessor plans: 13

1. Load/validate preferences asynchronously, initialize worker, and keep renderer/session independent.
2. For accepted input, update the session first, then enqueue sound. On failure post one quiet status. Dropped sound jobs are bounded diagnostics without typing text.
3. Completion/PB use idempotent event identities and committed result state. Reduced-motion skips only visual delay.
4. Exit cancels loader commands, closes worker/device, then restores terminal and serializes CLI output.

## Non-functional and cross-cutting rules

- Typing input processing performs no filesystem, audio-device, or network waits. No input event is discarded to catch up with rendering. Coalesce redraws, never accepted typing events.
- Use the same active clock for input, metrics, timer, and charts. Application overlays pause once at the outermost entry and resume once after the last blocking view closes.
- Authenticate nothing: this is a local single-user executable. Validate all new input at its boundary and show path/resource/action-specific errors. Local diagnostics contain no typing text unless the user explicitly requested it.
- Network rate limiting belongs exclusively to plan 016. History indexing and concurrency belong to plan 003. This phase consumes those policies only when they are prerequisites.
- Measure ready-screen <=250 ms, event-to-visible-input <=50 ms, and state-preserving resize <=100 ms on the documented reference setup when this phase changes those paths. Use 100 samples and report p50, p95, p99, and maximum. Slow terminal behavior is recorded rather than described as passing.

## Failure modes and required handling

| Path and failure | Handling and caller visibility | Required coverage |
| --- | --- | --- |
| Device init/decoder fails | Quiet audio-unavailable status, test continues | +4 backend faults |
| Queue/audio rendering stalls | Nonblocking enqueue and bounded streams/cache | +3 saturation fixtures |
| Late preview applies after Cancel | Correlate request IDs and discard stale status | +2 preview cases |
| Repeated completion plays PB twice | Event dedup and committed-PB predicate | +2 completion cases |

The coverage above is required new coverage, not a claim that these tests exist today. No silent, unhandled failure in a changed path satisfies this phase.

## Test plan

| Layer | Named coverage | Minimum new cases |
| --- | --- | --- |
| Unit | Sound precedence, bounded queue/cache, dedup and close | 12 |
| Integration | Slow/failing backend with ordered 200 WPM input | 4 |
| E2E | Configure/preview/disable and completion | 2 |

Use deterministic seeds, clocks, and temporary data directories. E2E cases mean scripted terminal/model journeys with real generation and storage where used, plus native-terminal inspection where capabilities cannot be simulated. Run `GOCACHE=/private/tmp/terminal-typer-go-cache make verify`, `GOCACHE=/private/tmp/terminal-typer-go-cache make smoke`, and `git diff --check`. Update `README.md` and `man.md` together when behavior becomes public. Regenerate with `make assets` when the manual or source assets change and verify `gzip -t tt.1.gz`. Do not edit `src/packed.go` by hand.

## Rollout and rollback

No new data schema or sound assets. Existing sound strings in appearance become actionable. Revert the worker/UI PR to disable new feedback while keeping preferences/history. Audio failure is intentionally nonfatal, an approved reliability correction documented alongside unchanged sound flag syntax.

Release, deployment, publishing binaries, and uploading user data require separate explicit human authorization. Update `CHANGELOG.md` when a release occurs, not while this proposal is written.

## Definition of done

- [ ] AC-1: Fresh settings play0 sounds. Each enabled event triggers only its matching resource, with error fallback to key sound following existing CLI behavior.
- [ ] AC-2: Missing/malformed WAV/MP3, unavailable device and worker saturation leave the test completable; no input is lost and visible latency stays <=50 ms in reference measurements.
- [ ] AC-3: CLI sound values are active-only and TUI save changes future values without overriding active flags.
- [ ] AC-4: Completion/PB feedback never repeats on replayed callbacks or tied PBs. Reduced-motion completes visual transition immediately.
- [ ] AC-5: Worker/device close on normal exit, Ctrl-C and startup error without goroutine growth across 100 tests.
