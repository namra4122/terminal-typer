# Plan 005: Launch the remembered test immediately

Status: proposed implementation specification. No feature in this document is implemented by the act of writing it. Source: [PRD](../../PRD.md). Planning baseline: 2026-09-26 working tree, including existing uncommitted `src/test.go`, `src/test_test.go`, and `src/tt.go` changes. Reinspect those files before implementation and preserve unrelated work.

## Goal and user job

Bare tt starts a fresh saved test, defaulting to English 1k for 30 seconds, with intentional test configuration saved on Start.

As a terminal-heavy developer, I can exercise this capability entirely from the keyboard while keeping my input and progress local. This phase is one reviewable PR, estimated at 1-3 focused implementation sessions. The file table is the implementation boundary, including tests and public documentation. Generated outputs are declared separately.

## Decisions and assumptions

- Make Charm the default for bare or presentation-only launches after this phase. Explicit legacy test-defining input continues through its compatibility route until plan 015. `TT_UI=legacy` is the temporary rollback switch; `TT_UI=charm` retains contributor forcing within supported predicates.
- Use version-2 `settings.json` at the existing path, retaining all six live settings and adding the exact new shape below. Migration is backed up and validated before replacement. Teach compatibility readers/writers to read v2 and merge only live fields, so coexistence cannot erase remembered test configuration.
- Expose basic configuration with Ctrl-K opening Configure directly in this phase. Plan 006 upgrades that entry to the full palette without changing the Start contract.
- Go, Bubble Tea v2, Lip Gloss v2, the existing Makefile, and local-only application data are inherited from plan 001. No server, account, telemetry, executable plugins, or release action is part of this phase.

## Scope

- Deliver timed words with presets 15/30/60/120 and custom 5-3600; word count 10/25/50/100 and custom 1-500; completion-based existing quote mode. Normal is the only difficulty.
- Maintain timed content by generating an initial 200-word buffer and appending 100 words whenever fewer than 50 unvisited words remain. Retain all generated words in memory so Retry repeats the complete material generated for that attempt. The timer cannot end on buffer exhaustion.
- Configure groups Test, Content, Typing, Display, Sound, Data, Help, with only implemented controls actionable. Start validates/saves intentional choices and starts fresh content. Preview/Cancel do not save. Paths/piped content never enter startup settings.
- Preserve six live defaults, dirty-row merge, active CLI versus saved values, and transactional save failure. Provide reset section/test/appearance/all settings; confirm reset-all and leave history untouched.
- First-run hints name Ctrl-K configuration, Ctrl-P Settings and Help. Store discovered/dismissed hint IDs without a wizard.

## Explicitly deferred scope

- Full palette/pickers: plan 006.
- Catalog metadata/modifiers: plan 007.
- New reviewed dialogue: plans 008-009.
- New theme/focus/audio/data controls: plans 011, 013-014, 016.

## Requirement coverage

CFG-01 through CFG-11; MODE-01, MODE-02, MODE-03 (existing quote pack); CLI-01 through CLI-07; DATA-13 through DATA-15; UI-08; SESSION-08.

Every listed requirement is checked by the acceptance criteria and the named fixtures below. Shared IDs can have incremental coverage in several plans. The index maps the final release gates.

## Acceptance criteria

1. Fresh bare `tt` resolves timed/1000en/30000 ms, Normal, modifiers off, native dark and sound off. No time accrues before accepted text.
2. All preset/custom boundaries pass; 4 or 3601 seconds and 0 or 501 words are inline-invalid. A fast-input timed fixture exhausts 200 initial words yet continues until the exact timeout.
3. Start a 25-word test, exit/relaunch, and receive 25 fresh words. Browse/cancel a 60-second preview and verify saved bytes are unchanged.
4. Every row of the compatibility matrix below resolves exactly, regardless of conflicting saved mode/modifiers. Explicit CLI presentation overrides affect only the invocation.
5. Migration of valid v1 settings preserves all six values and makes a verified backup. Corrupt/unknown settings stay intact and use read-only defaults with recovery guidance. Failed Start/Settings save leaves original session/config active.
6. Reset-all requires confirmation, restores defaults, and does not change a history file. Saved test configuration never contains private paths or stdin content.

## Keyboard journey

1. Launch into ready typing. Open Ctrl-K Configure and move between group tabs with Tab/Shift-Tab.
2. Use arrows to select, Enter/Space to change, and Enter in numeric fields to validate. Start or Cancel is an explicit footer action.
3. After meaningful input, Start first shows replacement confirmation. Cancel returns to identical typing state.
4. Successful Start remembers the test and generates fresh text; Ctrl-P retains its six-control save-and-resume route.

## Verified reusable implementation

- `src/tt.go:433` registers legacy flags, `src/tt.go:485` captures visited flags, and `src/test.go:63` establishes source precedence. Extend resolution without parsing a second set of flags.
- `src/settings.go:24`, `src/settings.go:101`, `src/settings.go:133`, and `src/settings.go:384` supply schema validation, safe writes, and dirty merges.
- `src/wordtest.go:5` and plan 001 generation/reducer supply real words; plans 002-004 already provide results/history/practice.

Current-state citations refer to the inspected baseline, not hypothetical future line numbers. Files introduced by predecessor plans are cited by interface name below.

## Components and file boundary

| File | Change |
| --- | --- |
| `src/config.go` (new) | Version-2 codec, migration, flags-to-mode matrix and validation. |
| `src/config_test.go` (new) | Matrix, migration/rollback and default/custom bounds. |
| `src/settings.go:101` | Read v1/v2 and merge six live values without dropping v2 fields. |
| `src/test.go:63` | Accept new mode/count/timed generator while preserving legacy resolver. |
| `src/tt.go:485` | Dispatch default Charm and pass visited flags/origin. |
| `src/app.go` (plan 001) | Configure/reset/hints and buffered timed words. |
| `README.md:56`, `man.md` | Document approved new default, reset and precedence. |

## Data model and constraints

```go
type SavedTest struct { Mode string; Pack string; DurationSeconds, Count int; Modifiers TestModifiers; Difficulty string }
type Appearance struct { Theme string; Focus, ShowErrors, ReducedMotion bool; KeySound, ErrorSound, CompletionSound, PBSound string }
type Configuration struct {
    Version int // 2
    Settings runtimeSettings; Test SavedTest; Appearance Appearance
    DiscoveredHints []string
}
```
JSON keys: `version`, `settings` (existing six keys unchanged), `test` (`mode`,`pack`,`durationSeconds`,`count`,`modifiers`,`difficulty`), `appearance` (`theme`,`focus`,`showErrors`,`reducedMotion`,`keySound`,`errorSound`,`completionSound`,`pbSound`), `discoveredHints`. All required. `SavedTest.Mode` is timed/count/quote; pack is an embedded or explicitly approved catalog identifier, never a path. Normal is stored as `normal`. Defaults: timed,1000en,30,50,all modifiers false; theme tt-dark; all sound strings empty; flags false except existing live defaults. Duration/count inactive values persist for switching modes but only active length defines a result. Unimplemented appearance controls keep defaults and do not appear as actions.
Existing version1 backup path is `<data>/backups/settings-v1-<32hex>.json`. Preserve exact original bytes; write backup with 0600, Sync, validate by hash, then atomically commit v2. Invalid/unknown config remains at the original path; successful preservation of a separate backup does not authorize overwriting the unknown original.

## Go and CLI contracts

```go
func ResolveLaunch(saved Configuration, flags testOptions, visited map[string]bool) (TestConfig, error)
func ValidateSavedTest(test SavedTest) error
func CommitConfiguration(path string, draft Configuration, dirty []string) error
```
Resolve defaults -> saved -> explicit input/visited flags. No presets in v1. Exact fixtures:

| Invocation with conflicting saved timed/modifiers | Resolved behavior |
| --- | --- |
| `tt` / `tt -showwpm` / `tt -json` | Saved test (new user:30s/1000en); output alone is not headless |
| `tt -n 10` / `tt -g 2` | Legacy words 10x1 / 50x2, unlimited time unless explicit -t; modifiers off |
| `tt -t 15` | Legacy 50x1 words with 15s maximum; no saved modifiers |
| `tt -words fr` | Legacy fr/50x1/unlimited unless -t; no saved modifiers |
| `tt -quotes en` | Authored quote/unlimited unless -t; modifiers off |
| `cat text \| tt file` | stdin ahead of positional file; no inherited timer/modifiers |
| `tt -start 0 file` | First paragraph, saved-mode timer/modifiers cleared |
| `tt -words fr -quotes en file` | Explicit words outrank quotes/stdin/file |
| `tt -raw -multi` with stdin | Full raw input replay outranks paragraph splitting |
| `tt -theme name -oneshot -noreport -csv` | Saved/default test, explicit presentation/report/output; no consent/update network |

Any of `n,g,t,words,quotes,start,raw,multi`, positional file or non-TTY stdin selects the legacy test-resolution branch. Other flags do not alter saved test selection. `-words -` and `-quotes -` consume stdin as the resource, not as a second custom source. Empty/malformed resource errors remain resource-specific. `CommitConfiguration` locks config writes, reloads current version, merges dirty paths, validates, then commits all fields in one atomic document. Failed commit returns wrapped error and leaves previous active configuration. Preserve successful v1 runtime edits from other processes during migration by performing read/backup/commit under the same config lock.

There are no HTTP APIs in this phase unless an explicit network contract appears above. Return errors rather than panicking across the application boundary. Bubble Tea commands perform I/O and report immutable messages to the model. `Update` owns mutable UI/session state and does not wait on I/O.

## Dependencies and operation sequence

Required predecessor plans: 4

1. Resolve paths and load settings before starting terminal rendering. Migrate a valid v1 file under the config lock; if absent use defaults without persisting until a deliberate save/hint discovery.
2. Resolve visited flags through the matrix. Explicit private sources bypass saved test values entirely. Presentation CLI values are active-only, with saved-future labels in Settings.
3. Configure a draft, validate active length, ask replacement confirmation after meaningful input, save only on Start, then generate. Generation/read failure keeps the originating session and configuration unchanged: validate/load resource before committing the draft.
4. Settings save reloads and merges dirty rows into the v2 document; only success updates effective values. Reset uses the same transaction. Timed appends preserve the prior last word to avoid adjacent duplicates and use the session's random stream.
5. Rollback fixture restores the verified v1 backup to a separate test data root and exercises the baseline reader; do not automate overwriting a user's current v2 file.
6. Timed retries reuse the cached generated stream and its initial seed. Rewind the stream, reuse all cached tokens exactly, and extend beyond the cached tail only by continuing the same RNG sequence with the same absolute modifier positions. This preserves the exact previous generated prompt prefix while allowing the timed test to last its full duration at a faster retry speed. Retry remains ineligible for PB regardless of generated-tail extension.

## Non-functional and cross-cutting rules

- Typing input processing performs no filesystem, audio-device, or network waits. No input event is discarded to catch up with rendering. Coalesce redraws, never accepted typing events.
- Use the same active clock for input, metrics, timer, and charts. Application overlays pause once at the outermost entry and resume once after the last blocking view closes.
- Authenticate nothing: this is a local single-user executable. Validate all new input at its boundary and show path/resource/action-specific errors. Local diagnostics contain no typing text unless the user explicitly requested it.
- Network rate limiting belongs exclusively to plan 016. History indexing and concurrency belong to plan 003. This phase consumes those policies only when they are prerequisites.
- Measure ready-screen <=250 ms, event-to-visible-input <=50 ms, and state-preserving resize <=100 ms on the documented reference setup when this phase changes those paths. Use 100 samples and report p50, p95, p99, and maximum. Slow terminal behavior is recorded rather than described as passing.

## Failure modes and required handling

| Path and failure | Handling and caller visibility | Required coverage |
| --- | --- | --- |
| Saved timer leaks into legacy input | Explicit test branch clears timer/modifiers; resolved summary visible | +10 matrix fixtures |
| Migration interrupted | Backup before replacement, preserve original, read-only defaults | +5 interruption fixtures |
| Timed buffer runs out | Extend before frontier; generator error pauses with Retry/Cancel | +2 high-rate cases |
| Start saves invalid/browsed content | Validate/read first and commit only Start; same session on failure | +4 draft cases |
| Concurrent live edit drops test fields | Dirty-path merge under config lock; Busy/Retry visible | +2 merge cases |

The coverage above is required new coverage, not a claim that these tests exist today. No silent, unhandled failure in a changed path satisfies this phase.

## Test plan

| Layer | Named coverage | Minimum new cases |
| --- | --- | --- |
| Unit | Resolution matrix, all length boundaries, config validation | 25 |
| Integration | V1 migration, interrupted save, merge and rollback | 9 |
| E2E | Fresh launch, remember/cancel, timed refill, resets | 4 |

Use deterministic seeds, clocks, and temporary data directories. E2E cases mean scripted terminal/model journeys with real generation and storage where used, plus native-terminal inspection where capabilities cannot be simulated. Run `GOCACHE=/private/tmp/terminal-typer-go-cache make verify`, `GOCACHE=/private/tmp/terminal-typer-go-cache make smoke`, and `git diff --check`. Update `README.md` and `man.md` together when behavior becomes public. Regenerate with `make assets` when the manual or source assets change and verify `gzip -t tt.1.gz`. Do not edit `src/packed.go` by hand.

## Rollout and rollback

Switch bare/presentation-only launches to Charm. Retain explicit legacy-source fallback and temporary `TT_UI=legacy`. Version2 writes preserve version1 backups and legacy file/mistake stores. Rollback requires baseline code plus a manually selected v1 backup in an isolated/restored data root; reverting code alone cannot read v2. Document this before enabling migration. Public README/man change together; regenerated manual is a declared output.

Release, deployment, publishing binaries, and uploading user data require separate explicit human authorization. Update `CHANGELOG.md` when a release occurs, not while this proposal is written.

## Definition of done

- [ ] AC-1: Fresh bare `tt` resolves timed/1000en/30000 ms, Normal, modifiers off, native dark and sound off. No time accrues before accepted text.
- [ ] AC-2: All preset/custom boundaries pass; 4 or 3601 seconds and 0 or 501 words are inline-invalid. A fast-input timed fixture exhausts 200 initial words yet continues until the exact timeout.
- [ ] AC-3: Start a 25-word test, exit/relaunch, and receive 25 fresh words. Browse/cancel a 60-second preview and verify saved bytes are unchanged.
- [ ] AC-4: Every row of the compatibility matrix below resolves exactly, regardless of conflicting saved mode/modifiers. Explicit CLI presentation overrides affect only the invocation.
- [ ] AC-5: Migration of valid v1 settings preserves all six values and makes a verified backup. Corrupt/unknown settings stay intact and use read-only defaults with recovery guidance. Failed Start/Settings save leaves original session/config active.
- [ ] AC-6: Reset-all requires confirmation, restores defaults, and does not change a history file. Saved test configuration never contains private paths or stdin content.
