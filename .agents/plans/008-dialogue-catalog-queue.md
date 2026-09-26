# Plan 008: Browse authored passages without repeats

Status: proposed implementation specification. No feature in this document is implemented by the act of writing it. Source: [PRD](../../PRD.md). Planning baseline: 2026-09-26 working tree, including existing uncommitted `src/test.go`, `src/test_test.go`, and `src/tt.go` changes. Reinspect those files before implementation and preserve unrelated work.

## Goal and user job

Users can browse passage metadata and complete authored text using a persistent shuffled queue.

As a terminal-heavy developer, I can exercise this capability entirely from the keyboard while keeping my input and progress local. This phase is one reviewable PR, estimated at 1-3 focused implementation sessions. The file table is the implementation boundary, including tests and public documentation. Generated outputs are declared separately.

## Decisions and assumptions

- Add versioned catalog manifests alongside, not in place of, legacy `quotes/en` JSON. Existing legacy quotes remain available without invented source/speaker/license metadata. New official passages require original/permitted provenance.
- Queue is per pack revision plus canonical metadata filter. Retain the last10 passage IDs across restarts, capped at pack size-1. A shuffled cycle contains every filtered passage once. Retry bypasses queue consumption.
- Length bands are short1-39 words, medium40-79, long80-150; difficulty easy/standard/challenging is authored metadata, never silently inferred. V1 passages are capped at 150 words to fit a short session.
- Go, Bubble Tea v2, Lip Gloss v2, the existing Makefile, and local-only application data are inherited from plan 001. No server, account, telemetry, executable plugins, or release action is part of this phase.

## Scope

- Add genre/source browsing, title/speaker/provenance/license details, length/difficulty filters and preview. Preserve authored punctuation/case/newline content; renderer wrapping changes cells only.
- Implement a real small original three-passage fixture pack to exercise end-to-end browsing/queue/storage. Mark it a developer fixture, not the reviewed 100 release catalog.
- Persist queue IDs, order, recent set and cursor, with config-style safe writes/errors. Collection/category stays muted during typing; full attribution appears on Results and historical metadata when catalog revision is available.
- Invalid/empty filtered packs disable Start with remedy. Legacy local quote arrays remain valid and are private by default.

## Explicitly deferred scope

- 100 reviewed official passages: plan 009.
- Detailed metadata on Analysis: plan 010.
- Advanced resource formats/import: post-v1.

## Requirement coverage

MODE-03; CONTENT-03 through CONTENT-09; UI-10; CFG-03; PB-01, PB-05 (identity); DATA-04.

Every listed requirement is checked by the acceptance criteria and the named fixtures below. Shared IDs can have incremental coverage in several plans. The index maps the final release gates.

## Acceptance criteria

1. A3-passage fixture visits each passage once per cycle, avoids immediate repetition after process restart, and Retry repeats without advancing the queue.
2. Genre/source/length/difficulty filters yield exact matching entries and an honest empty state. Quote tests inherit no word timer/modifiers.
3. Expected authored text remains byte-equal in the canonical prompt; only display wrapping changes. Full source/title/speaker/provenance/license reaches Results.
4. Malformed manifests, duplicate passage IDs, empty text and unknown versions leave originals intact and show the affected resource/remedy.
5. Legacy `[{text,attribution}]` resources still work; absence of new metadata does not block the CLI path.

## Keyboard journey

1. Select Quotes/dialogue and browse collection/genre, then filter metadata.
2. Preview a passage and its source. Cancel restores context without queue consumption.
3. Start commits the configuration and reserves the next passage.
4. Complete, inspect attribution, Retry identical text, or choose Next to advance the shuffled queue.

## Verified reusable implementation

- `src/quotetest.go:8` already loads legacy segment arrays; retain this loader contract rather than forcing local format migrations.
- `src/typer.go:26` segment attribution and `src/test.go:134` Test attribution already carry authored metadata minimally.
- Plans003/005 supply safe local writes, version handling and configuration transactions; plan 007 supplies catalog embedding/origin.

Current-state citations refer to the inspected baseline, not hypothetical future line numbers. Files introduced by predecessor plans are cited by interface name below.

## Components and file boundary

| File | Change |
| --- | --- |
| `src/dialogue.go` (new) | Metadata validation, filters and shuffled queue. |
| `src/dialogue_test.go` (new) | Repeat/restart/authored-text and manifest cases. |
| `catalog/dialogue-dev.json` (new) | Three original fixture passages with full metadata. |
| `src/test.go:108` | New catalog passage generation beside legacy quote loader. |
| `src/app.go` (plan 001) | Browsing/filter/preview/attribution UI. |
| `src/history.go` (plan 003) | Record passage ID/revision/public attribution. |
| `README.md:67`, `man.md` | Authored passage/queue/local-format behavior. |

## Data model and constraints

Manifest: `{version:1,id:string,displayName:string,language:string,type:"dialogue",license:string,source:string,attribution:string,maintainerNotes:string,revision:string,passages:[]Passage}`. `Passage` has required `{id:string,text:string,title:string,source:string,speaker:string,provenance:string,license:string,wordCount:int,difficulty:string,tags:[]string,genre:string,reviewed:bool,reviewer:string,reviewedDate:string}`. Speaker may be empty for prose. ReviewedDate is ISO date or empty for dev fixtures. IDs match `[a-z0-9][a-z0-9-]{0,63}`. Word count is derived by Unicode whitespace splitting and must match declared metadata. Tags max10, each1..32 scalars. Text is valid UTF-8 without terminal control sequences except newline/tab. Preview expands tabs visually without editing authored text.
Queue file `<data>/dialogue-queue-v1.json`: `{version:1,queues:map[string]QueueState}`, each `{packRevision:string,filterKey:string,order:[]string,cursor:int,recent:[]string}`. FilterKey is SHA256 of sorted genre/source/length/difficulty selections. Store no passage text. Add SavedTest `passageFilter` optional `{genres:[]string,sources:[]string,lengths:[]string,difficulties:[]string}`; missing means all. Queue reservation is persisted before Start; a cancelled preview consumes nothing. Failed queue save leaves the current test and remembered configuration intact.

## Go and CLI contracts

```go
type PassageFilter struct { Genres, Sources, Lengths, Difficulties []string }
func LoadDialogue(name string) (DialoguePack, error)
func FilterPassages(pack DialoguePack, filter PassageFilter) []Passage
func ReservePassage(pack DialoguePack, filter PassageFilter, seed int64) (Passage, error)
```
Reserve under the queue write lock, reread latest state, reject unknown versions, shuffle exhausted cycles using Fisher-Yates. Stable-partition recently used IDs toward the tail; if every ID is recent shrink recent window to size-1. A single passage necessarily repeats and shows that limitation. Pack revision change discards only the old queue state after preserving a backup, not history. Commit config and queue as a recoverable Start transaction: prepare/validate both, write a small journal with prior/new config hash and selected ID, reserve queue, commit config, then remove journal. If config commit fails, release the reservation under lock when no intervening queue revision exists; otherwise keep the consumed ID and show `test not started; passage reserved`. Never fabricate a rollback that overwrites another process's queue.

There are no HTTP APIs in this phase unless an explicit network contract appears above. Return errors rather than panicking across the application boundary. Bubble Tea commands perform I/O and report immutable messages to the model. `Update` owns mutable UI/session state and does not wait on I/O.

## Dependencies and operation sequence

Required predecessor plans: 7

1. Validate pack and filters before showing choices. Browser previews never alter queue/config.
2. Start prepares resource and confirmation, reserves the passage and commits config through the journal. Open the shared session with completion length and no modifiers.
3. Finish stores public catalog identity/attribution, then Next reserves a fresh passage. Retry reuses the cached Test and leaves queue untouched.
4. If catalog text cannot be resolved for historical analysis, retain safe attribution and show exact-prompt unavailable. Never persist complete text to repair missing metadata.

## Non-functional and cross-cutting rules

- Typing input processing performs no filesystem, audio-device, or network waits. No input event is discarded to catch up with rendering. Coalesce redraws, never accepted typing events.
- Use the same active clock for input, metrics, timer, and charts. Application overlays pause once at the outermost entry and resume once after the last blocking view closes.
- Authenticate nothing: this is a local single-user executable. Validate all new input at its boundary and show path/resource/action-specific errors. Local diagnostics contain no typing text unless the user explicitly requested it.
- Network rate limiting belongs exclusively to plan 016. History indexing and concurrency belong to plan 003. This phase consumes those policies only when they are prerequisites.
- Measure ready-screen <=250 ms, event-to-visible-input <=50 ms, and state-preserving resize <=100 ms on the documented reference setup when this phase changes those paths. Use 100 samples and report p50, p95, p99, and maximum. Slow terminal behavior is recorded rather than described as passing.

## Failure modes and required handling

| Path and failure | Handling and caller visibility | Required coverage |
| --- | --- | --- |
| Recent queue repeats on restart | Durable IDs and capped recent set; single-passage warning | +4 cycle fixtures |
| Authored text transformed | Canonical byte preservation, visual mapping only | +3 punctuation/newline cases |
| Queue/config partial commit | Journal and non-overwriting reservation recovery; visible consumed-ID status | +3 faults |
| Unknown metadata invents attribution | Legacy adapter uses empty unavailable fields | +2 legacy fixtures |

The coverage above is required new coverage, not a claim that these tests exist today. No silent, unhandled failure in a changed path satisfies this phase.

## Test plan

| Layer | Named coverage | Minimum new cases |
| --- | --- | --- |
| Unit | Manifest/filter/cycle/metadata validation | 15 |
| Integration | Queue locks, journal recovery and revision change | 7 |
| E2E | Browse/cancel/start/retry/next | 1 |

Use deterministic seeds, clocks, and temporary data directories. E2E cases mean scripted terminal/model journeys with real generation and storage where used, plus native-terminal inspection where capabilities cannot be simulated. Run `GOCACHE=/private/tmp/terminal-typer-go-cache make verify`, `GOCACHE=/private/tmp/terminal-typer-go-cache make smoke`, and `git diff --check`. Update `README.md` and `man.md` together when behavior becomes public. Regenerate with `make assets` when the manual or source assets change and verify `gzip -t tt.1.gz`. Do not edit `src/packed.go` by hand.

## Rollout and rollback

New manifests and queue namespace are additive. Preserve legacy quote assets and v1 queue backups before revision reset. Rollback reverts UI/loader changes and leaves queue/history files intact; old local quote arrays stay executable. The tiny fixture does not satisfy CONTENT-02 and is not advertised as reviewed production content.

Release, deployment, publishing binaries, and uploading user data require separate explicit human authorization. Update `CHANGELOG.md` when a release occurs, not while this proposal is written.

## Definition of done

- [ ] AC-1: A3-passage fixture visits each passage once per cycle, avoids immediate repetition after process restart, and Retry repeats without advancing the queue.
- [ ] AC-2: Genre/source/length/difficulty filters yield exact matching entries and an honest empty state. Quote tests inherit no word timer/modifiers.
- [ ] AC-3: Expected authored text remains byte-equal in the canonical prompt; only display wrapping changes. Full source/title/speaker/provenance/license reaches Results.
- [ ] AC-4: Malformed manifests, duplicate passage IDs, empty text and unknown versions leave originals intact and show the affected resource/remedy.
- [ ] AC-5: Legacy `[{text,attribution}]` resources still work; absence of new metadata does not block the CLI path.
