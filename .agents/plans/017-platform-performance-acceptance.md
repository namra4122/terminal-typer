# Plan 017: Verify the v1 experience on supported terminals

Status: proposed implementation specification. No feature in this document is implemented by the act of writing it. Source: [PRD](../../PRD.md). Planning baseline: 2026-09-26 working tree, including existing uncommitted `src/test.go`, `src/test_test.go`, and `src/tt.go` changes. Reinspect those files before implementation and preserve unrelated work.

## Goal and user job

The complete keyboard workflow is verified on Linux, macOS and Windows Terminal with measured performance and recoverable local data.

As a terminal-heavy developer, I can exercise this capability entirely from the keyboard while keeping my input and progress local. This phase is one reviewable PR, estimated at 1-3 focused implementation sessions. The file table is the implementation boundary, including tests and public documentation. Generated outputs are declared separately.

## Decisions and assumptions

- Preserve XDG_DATA_HOME/tt and HOME/.local/share/tt wherever present. On Windows without HOME use os. UserHomeDir then the same .local/share/tt contract, rather than introducing an undocumented AppData migration. Existing ~/.tt and /etc/tt resource precedence stays defined.
- Reference hardware profiles are Apple Silicon M1-or-newer with 8 GiB RAM, Linux x86_64 4-core/8 GiB local SSD, Windows11 x86_64 4-core/8 GiB local SSD using modern Windows Terminal. Actual make/model/OS/terminal versions must be recorded in verification evidence before claiming support. These are benchmark test profiles, not current verified user hardware.
- Canonical Makefile owns tests/build/smoke/assets; CI uses a Linux/macOS/Windows matrix with Go minimum1.26.0 plus the contributor's current supported compiler. Do not add a second build system or publish artifacts.
- Go, Bubble Tea v2, Lip Gloss v2, the existing Makefile, and local-only application data are inherited from plan 001. No server, account, telemetry, executable plugins, or release action is part of this phase.

## Scope

- Complete all 15 PRD end-to-end gates with real local generation/storage and deterministic clocks/seeds for invariants. Record actual pass/fail/evidence paths.
- Verify native input/correction/control keys, Unicode, resize/too-small, color/ASCII, audio failure, paths, piped stdin, output and terminal restoration on each advertised platform.
- Add repeatable ready/input/resize/progress benchmark harness and sustained 200 WPM ordered-input fixture. Keep slow persistence/audio/network off the typing loop and test failures.
- Harden home-path errors and platform-specific file replacement/permissions based on the existing contracts. Document measured support/limitations and tested migration rollback. Mark v1 complete only with 100 reviewed passages plus all gates passed.

## Explicitly deferred scope

- Binary release/publication/deployment is a separate explicit human action. Future architectures are advertised only after corresponding verification.
- Every post-v1 capability in PRD3.2 and Section 13 remains outside this gate.

## Requirement coverage

PERF-01 through PERF-06; Section 12.1; A-01 through A-15; DATA-13 through DATA-15; CLI-01 through CLI-07; Section 8.1.

Every listed requirement is checked by the acceptance criteria and the named fixtures below. Shared IDs can have incremental coverage in several plans. The index maps the final release gates.

## Acceptance criteria

1. A-01 through A-15 pass with evidence, including 100 actual reviewed passages. No gate is marked passed from planning text alone.
2. 100 warm-launch samples have ready screen p95<=250 ms; 1000 key visibility samples p95<=50 ms;100 resize samples p95<=100 ms;100 Progress opens on 10,000 records p95<=500 ms. Record p50/p95/p99/max and all outliers with conditions.
3. A10-minute sustained input test at 200 WPM (1000 scalars/minute including spaces, events every 60 ms), with corrections/redraws and slow dependencies, has 0 lost/reordered accepted events.
4. Platform-specific native-terminal fixtures pass on all 3 named targets; cross-compilation alone does not count as Windows input/audio/terminal verification.
5. Interrupted/unknown migrations, settings/history rollback, deletion journals and private sentinel tests preserve original data and supply visible recovery routes.
6. README/man agree with actual executable behavior and validated platform claims; canonical verify/smoke/assets/manual integrity and diff checks pass. No artifacts are published.

## Keyboard journey

1. Use a fresh temporary data root on each platform, launch and complete the default test.
2. Configure/relaunch, open pausing overlays, complete/retry, review practice and inspect progress.
3. Export/delete/recover with disposable fixture data; exercise custom stdin/file outputs.
4. Inspect native rendering/input/resize/exit, record evidence and publish only local verification documentation.

## Verified reusable implementation

- `Makefile:7`, `Makefile:19`, `Makefile:21`, `Makefile:38`, and `Makefile:44` already own build/verification/smoke/assets/platform build commands.
- `.github/workflows/verify.yml:1` currently verifies only ubuntu-latest; extend this workflow rather than adding duplicate pipelines.
- `src/db.go:19` requires HOME today; verify and correct platform fallback while preserving the established data paths. `src/typer.go:85` assumes /dev/tty on old paths; plan 015 replaces ownership with Charm.

Current-state citations refer to the inspected baseline, not hypothetical future line numbers. Files introduced by predecessor plans are cited by interface name below.

## Components and file boundary

| File | Change |
| --- | --- |
| `src/platform_test.go` (new) | Paths, atomic replacement, terminal lifecycle contracts. |
| `src/performance_test.go` (new) | Deterministic input/progress/resize measurements and benchmarks. |
| `src/db.go:14` | Error-returning platform home resolution and explicit path failures. |
| `Makefile:19` | Canonical benchmark/acceptance commands and supported cross-build checks. |
| `.github/workflows/verify.yml:1` | Verified OS/compiler matrix, no release upload. |
| `docs/v1-acceptance.md` (new) | Reference setup,15 gates, measurements and rollback evidence. |
| `README.md`, `man.md` | Only verified platforms/performance methodology/behavior. |

## Data model and constraints

Acceptance evidence schema in docs/v1-acceptance.md: `date`, `commit`, `dirtyDiffHash`, `hardware`, `RAM`, `storage`, `OS`, `terminalVersion`, `locale`, `colorProfile`, `GoVersion`, `warmOrCold`, `fixture`, `sampleCount`, `p50MS`, `p95MS`, `p99MS`, `maxMS`, `failures`, `gate`, `evidencePath`. All timing fields are numeric or explicitly unavailable with a reason; no invented measurements. Record reference setup for each platform. Benchmark seed is 42; timestamps use injected clock unless measuring actual terminal timing.
Fixtures live in temporary directories and contain10,000 valid varied records with a healthy persisted index. Measure indexed cold-process Progress open and repeated in-process opens separately; separately report missing-index rebuild time without claiming it meets the healthy-index target. Cold launch is separately reported, not substituted for the PRD warm target. Session timing/metrics use the same saved contract on each OS.

## Go and CLI contracts

```go
func ResolveDataDir(env map[string]string, userHome func()(string,error)) (string,error)
```
Resolve valid nonempty XDG first, then nonempty HOME, then os. UserHomeDir. Reject empty/unresolvable home with an actionable error before writing. No change to legacy file names.
Makefile adds `bench` for Go benchmarks and `acceptance` for deterministic model/storage journeys, without replacing verify/smoke. PTY/native-terminal checks are documented separately where CI has no interactive terminal. Windows CI uses a documented POSIX-shell invocation compatible with existing Makefile commands; native Windows Terminal validation is an actual local/manual gate, not a CI matrix label.
For timing evidence mark events with monotonic timestamps at input receipt and view publication; use actual native-terminal capture or a verified PTY visible-frame observer for event-to-visible timing, not Update duration alone. If observing actual frame emission is unavailable, report PERF-02 unverified instead of treating reducer timing as visible latency.

There are no HTTP APIs in this phase unless an explicit network contract appears above. Return errors rather than panicking across the application boundary. Bubble Tea commands perform I/O and report immutable messages to the model. `Update` owns mutable UI/session state and does not wait on I/O.

## Dependencies and operation sequence

Required predecessor plans: 9, 16

1. Run repository checks, regenerate intended outputs, and execute deterministic acceptance fixtures from disposable roots.
2. Measure warm/cold launch, sustained input, resize, and 10k healthy-index Progress on documented reference environments. Record all actual percentiles/outliers.
3. Run native-terminal journeys on each target, including Windows handle/path/audio/color behavior. Fail support claims when verification is absent.
4. Run prior-build/new-build migration and rollback fixtures using copied data roots; verify hashes and recovery messages.
5. Update public docs only to match passed evidence; the final release checklist references completed gate evidence and remains separate from any publication command.

## Non-functional and cross-cutting rules

- Typing input processing performs no filesystem, audio-device, or network waits. No input event is discarded to catch up with rendering. Coalesce redraws, never accepted typing events.
- Use the same active clock for input, metrics, timer, and charts. Application overlays pause once at the outermost entry and resume once after the last blocking view closes.
- Authenticate nothing: this is a local single-user executable. Validate all new input at its boundary and show path/resource/action-specific errors. Local diagnostics contain no typing text unless the user explicitly requested it.
- Network rate limiting belongs exclusively to plan 016. History indexing and concurrency belong to plan 003. This phase consumes those policies only when they are prerequisites.
- Measure ready-screen <=250 ms, event-to-visible-input <=50 ms, and state-preserving resize <=100 ms on the documented reference setup when this phase changes those paths. Use 100 samples and report p50, p95, p99, and maximum. Slow terminal behavior is recorded rather than described as passing.

## Failure modes and required handling

| Path and failure | Handling and caller visibility | Required coverage |
| --- | --- | --- |
| CI cross-build mistaken for native support | Separate native gate/evidence, do not advertise unverified platform | +3 native journeys |
| HOME absent leads unsafe path | OsUserHomeDir fallback or explicit error before write | +5 path cases |
| Healthy performance target masks rebuild | Report cold-index/warm/rebuild separately | +3 dataset modes |
| Input benchmark times reducer only | Measure visible frame and record unavailable evidence honestly | +2 observer checks |
| Generated docs/assets stale | Canonical regeneration/hash/manual integrity | +2 artifact checks |

The coverage above is required new coverage, not a claim that these tests exist today. No silent, unhandled failure in a changed path satisfies this phase.

## Test plan

| Layer | Named coverage | Minimum new cases |
| --- | --- | --- |
| Unit | Platform path/replacement/permission cases | 10 |
| Integration | 10k history, failure dependencies, migration and sustained input | 8 |
| E2E | All15 gates across 3 native targets | 45 |

Use deterministic seeds, clocks, and temporary data directories. E2E cases mean scripted terminal/model journeys with real generation and storage where used, plus native-terminal inspection where capabilities cannot be simulated. Run `GOCACHE=/private/tmp/terminal-typer-go-cache make verify`, `GOCACHE=/private/tmp/terminal-typer-go-cache make smoke`, and `git diff --check`. Update `README.md` and `man.md` together when behavior becomes public. Regenerate with `make assets` when the manual or source assets change and verify `gzip -t tt.1.gz`. Do not edit `src/packed.go` by hand.

## Rollout and rollback

This phase introduces no data-schema change; platform-safe writes keep the prior contracts. CI/build edits do not publish binaries. Rollback reverts platform/harness/docs while preserving stores. Any platform-specific path correction must prove the old documented root still resolves and offer an explicit backup/recovery path for invalid old paths. Current version0.4.2 need not be bumped here; release version/history require a separately authorized release decision.

Release, deployment, publishing binaries, and uploading user data require separate explicit human authorization. Update `CHANGELOG.md` when a release occurs, not while this proposal is written.

## Definition of done

- [ ] AC-1: A-01 through A-15 pass with evidence, including 100 actual reviewed passages. No gate is marked passed from planning text alone.
- [ ] AC-2: 100 warm-launch samples have ready screen p95<=250 ms; 1000 key visibility samples p95<=50 ms;100 resize samples p95<=100 ms;100 Progress opens on 10,000 records p95<=500 ms. Record p50/p95/p99/max and all outliers with conditions.
- [ ] AC-3: A10-minute sustained input test at 200 WPM (1000 scalars/minute including spaces, events every 60 ms), with corrections/redraws and slow dependencies, has 0 lost/reordered accepted events.
- [ ] AC-4: Platform-specific native-terminal fixtures pass on all 3 named targets; cross-compilation alone does not count as Windows input/audio/terminal verification.
- [ ] AC-5: Interrupted/unknown migrations, settings/history rollback, deletion journals and private sentinel tests preserve original data and supply visible recovery routes.
- [ ] AC-6: README/man agree with actual executable behavior and validated platform claims; canonical verify/smoke/assets/manual integrity and diff checks pass. No artifacts are published.
