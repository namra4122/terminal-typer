# Plan 003: Completed results survive restart

Status: proposed implementation specification. No feature in this document is implemented by the act of writing it. Source: [PRD](../../PRD.md). Planning baseline: 2026-09-26 working tree, including existing uncommitted `src/test.go`, `src/test_test.go`, and `src/tt.go` changes. Reinspect those files before implementation and preserve unrelated work.

## Goal and user job

Completed Charm tests appear in a durable local History list without retaining private input text.

As a terminal-heavy developer, I can exercise this capability entirely from the keyboard while keeping my input and progress local. This phase is one reviewable PR, estimated at 1-3 focused implementation sessions. The file table is the implementation boundary, including tests and public documentation. Generated outputs are declared separately.

## Decisions and assumptions

- The user selected one versioned JSON file per session with atomic writes and a rebuildable index. Use the standard library rather than SQLite. The store is `<data>/history-v1/`; the existing `.db`, `.errors`, and `settings.json` are preserved byte-for-byte by this phase.
- All non-embedded resources are private by default. Explicit local resource approval and private-retention controls arrive in plan 011. Legacy explicit CLI mistake output/files keep their contract. The new store never imports old aggregate mistakes as fabricated tests.
- Use store-only random opaque content IDs for private sources; no absolute source path enters rich history. Public resources use SHA-256 of pack revision plus canonical prompt solely for matching, with the disclosed guessability limitation.
- Go, Bubble Tea v2, Lip Gloss v2, the existing Makefile, and local-only application data are inherited from plan 001. No server, account, telemetry, executable plugins, or release action is part of this phase.

## Scope

- Write completed/expired results asynchronously and idempotently. Keep the current result visible while pending, saved, or failed. Retry save uses the same result ID.
- Add History through a Results action: 25-row pages, descending finish time then ID, regular/practice toggle, empty/error states, selected summary, and Escape back. Read-only navigation never starts or abandons a test.
- Persist aggregate measurements, series, permitted fragments, catalog IDs, and eligibility reasons. Never persist complete prompt, full typed response, raw event text, or incidental CLI paths.

## Explicitly deferred scope

- Filters/export/deletion/retention settings: plan 011.
- PB and trend projections: plan 012.
- Practice selection: plan 004.

## Requirement coverage

DATA-01 through DATA-07, DATA-10 through DATA-15; UI-04 (chronological list); METRIC-04, METRIC-06; CLI-05.

Every listed requirement is checked by the acceptance criteria and the named fixtures below. Shared IDs can have incremental coverage in several plans. The index maps the final release gates.

## Acceptance criteria

1. Finish two tests, restart the process, and find exactly two records in chronological History. Saving one ID twice produces one file and one row.
2. A private stdin/file fixture containing a sentinel secret produces no sentinel, file path, or word fragment in any new store/index file; aggregates remain usable.
3. Injected write/sync/rename failures leave previous committed files unchanged, keep Results visible, and show `not stored` with Retry save.
4. Malformed or unknown-version records keep their bytes intact, make History unavailable with the exact affected path, and allow a fresh typing test without replacing the store.
5. POSIX directories are 0700 and files 0600. Windows uses inherited user-profile access controls and is verified in plan 017. Existing `.db`/`.errors`/settings hashes are unchanged.

## Keyboard journey

1. Complete a test and see Saving then Saved on Results.
2. Open History, select a row, inspect the summary, and return to the same result.
3. Restart and browse the same records.
4. Inject an unreadable file, see a recovery explanation, and continue typing.

## Verified reusable implementation

- `src/db.go:14` owns the XDG/home path contract; extend its path resolver rather than creating a second root.
- `src/settings.go:133` demonstrates same-directory temporary writes, permissions, Sync, and Rename. Extend durability and error propagation for the new store; `src/db.go:46` direct writes are insufficient.
- Plan 002 supplies immutable `SessionResult`, metric version, outcome and nullable metrics. Plan 001 supplies IDs and command messages.

Current-state citations refer to the inspected baseline, not hypothetical future line numbers. Files introduced by predecessor plans are cited by interface name below.

## Components and file boundary

| File | Change |
| --- | --- |
| `src/history.go` (new) | Versioned codec, safe append/read, lock, index and privacy projection. |
| `src/history_test.go` (new) | Idempotency, crash/corruption, origin/privacy and real save/relaunch journeys. |
| `src/util.go:201` | Origin-returning resolver retaining resource lookup precedence. |
| `src/test.go:102` | Carry captured actual resource origin on generated Test. |
| `src/wordtest.go:5` | Reuse word-generation logic through a bytes-taking helper without a second resource read. |
| `src/app.go` (plan 001) | Save command/status and chronological History view. |
| `README.md:81`, `man.md` | Explain rich-history location, retention and recovery. |

## Data model and constraints

The JSON envelope is `{schemaVersion:1, session:HistoryRecord}`. Required fields and types are:

| Field | Type and constraint |
| --- | --- |
| id, contentId | string; 32-hex ID, private opaque ID or public SHA-256 string |
| metricVersion | string; `tt-metrics/1` |
| finishedUnixMS | int64; UTC epoch milliseconds >=0 |
| mode | string; words, quote, custom, timed, count, practice |
| sourceKind, sourceLabel, privacy | string; embedded-word/embedded-quote/approved-local/private-word/private-quote/file/stdin; safe label with no path; public/private |
| packId, packRevision, passageId | string; empty allowed for private/custom |
| durationMS, wordCount, wordsPerGroup, groups | int64/int/int/int; copy resolved configuration; duration -1 is unlimited |
| modifiers | object; punctuation/numbers/capitalization bool |
| difficulty | string; `normal` |
| skipWord, allowBackspace, raw, multi | bool; effective result-affecting input controls |
| outcome, retryOf, practice | string completed/expired; string empty or prior ID; bool |
| measurements | object; plan 002 Measurements without Words text/index data; Series and nullable numeric values retained |
| fragments | array of `{item:string, kind:string(word/pair), occurrences:int, attempts:int, correctAttempts:int, errors:int, activeMS:int64, context:string}`; <=20 entries, item <=64 scalars, context <=5 words |
| attribution | object `{source:string,title:string,speaker:string,provenance:string,license:string}`; public catalog-only, empty for private |
| eligibilityReasons | []string; empty only when ordinary eligibility rules pass |

The rich codec uses lowerCamelCase JSON keys. Required fields are validated. Unknown fields within schemaVersion 1 are ignored for additive evolution; unknown envelope versions are refused. Practice-specific additive fields are introduced in plan 004.
`index.json` is `{schemaVersion:1,generation:string,entries:[]IndexEntry}` with each entry `{id:string,file:string,size:int64,mtimeNS:int64,summary:HistoryRecord}` where summary omits Series/fragments. Filenames must match `<id>.json`, with no traversal. `store.json` is `{schemaVersion:1}`. Files are `<data>/history-v1/sessions/<id>.json`. Index is a cache, not authoritative data. Never overwrite a session with different content under an existing ID.
JSON names for Measurements are `wpm`,`rawWpm`,`cpm`,`accuracy`,`consistency`,`activeMS`,`pauseMS`,`characters`,`errors`,`attempts`,`correctAttempts`,`series`. CharacterTotals keys are `correct`,`incorrect`,`extra`,`missed`; ErrorTotals keys are `total`,`corrected`,`uncorrected`; samples use `endMS`,`windowMS`,`wpm`,`rawWpm`,`errors`. Counts are int, times int64 and rates nullable float64. Store activeNS/pauseNS:int64 beside millisecond presentation values so metric timing can be audited without reconstructing discarded private events. Historical numeric values are authoritative within their metricVersion; public-catalog identity never authorizes re-running unrelated new metric formulas.


## Go and CLI contracts

```go
type SaveState string // pending, saved, failed
type HistoryQuery struct { Practice bool; Offset, Limit int } // Limit 1..100; default 25
type HistoryPage struct { Records []HistoryRecord; Total int; Generation string }
func SaveHistory(root string, r HistoryRecord) error
func ReadHistory(root string, q HistoryQuery) (HistoryPage, error)
func ProjectHistory(r SessionResult, origin ResourceOrigin, privacy PrivacyPolicy) HistoryRecord
```
`ResourceOrigin` carries Kind/PackID/Revision/Path in memory; only approved label/catalog fields survive projection. `PrivacyPolicy` initially allows fragments only for embedded word/quote sources. Trim fragments/context so neither can contain a complete short prompt; if a fragment/context equals or reconstructs the whole prompt, omit it.
Return `ErrUnavailable` for invalid/unknown/unreadable authoritative data, `ErrBusy` for active writer lock, `ErrConflict` for an ID with different bytes, and wrapped filesystem errors. A successful idempotent duplicate is nil. No history HTTP interface.
Define `type ResourceOrigin struct { Kind, PackID, Revision, Path string; Embedded bool }` and `func ResolveResource(kind,name string) ([]byte,ResourceOrigin,error)` now, before any privacy projection. It reads exactly one winning resource and returns its actual origin: dash/stdin, explicit readable path, user directory, system directory, embedded. Keep existing `readResource` as a bytes-only compatibility wrapper over this resolver. Test gains `Origin ResourceOrigin`. For generated word sources, resolve once in newTestGenerator and call a bytes-taking helper extracted from generateWordTest; preserve that wrapper for existing callers. Mark embedded words public and any path/stdin word list private. Do not infer origin from the resource name or probe paths after generation. The Charm route at this phase only runs words; unclassified legacy quote/file/stdin Test values are conservatively private. Official dialogue assigns public origin in 008;015 closes all legacy adapters without granting unreviewed legacy quote content public retention automatically. Derive the history root from the existing resolved settings directory; no second root resolver is introduced.


There are no HTTP APIs in this phase unless an explicit network contract appears above. Return errors rather than panicking across the application boundary. Bubble Tea commands perform I/O and report immutable messages to the model. `Update` owns mutable UI/session state and does not wait on I/O.

## Dependencies and operation sequence

Required predecessor plans: 2

1. Project the immutable result through source privacy first; acquire an exclusive directory lock via atomic `os.Mkdir(root/.write-lock,0700)`. Owner file stores pid:int, startedUnixMS:int64, token:string. Wait at most 100 ms in a command, then show Busy/Retry. Never break a lock automatically. After crash, recovery explains how to verify no tt writer is active and remove only the lock.
2. Write a unique same-directory temp file, chmod0600, encode, Sync, Close, rename to the absent target under the lock, and Sync the parent on POSIX. On Windows use non-existing destination for sessions; replace cache by rename-old-to-backup then rename-new, restoring old on failure. Release the lock after durable commit.
3. Committed session files are authoritative. Update index after commit. Index failure returns Saved, index rebuild required; it never changes the successful record commit. Readers validate store/version and directory listing against cached filename/size/mtime. Rebuild invalid/stale/missing index from all records without modifying them. Corrupt session data makes the whole history service unavailable. Opening a selected row revalidates its record.
4. Only the latest result's save command updates its UI status, using result ID correlation. On exit, wait up to 2 seconds for already queued history work; if still pending, show/stderr `not stored` and preserve the in-memory result until the user chooses exit. Completed bytes on disk are never discarded.
5. Store errors and config errors are separate: history never resets itself. Recovery permits rebuild of the cache only, or explicit local export of still-readable stored data in plan 011; no implicit import/repair.

## Non-functional and cross-cutting rules

- Typing input processing performs no filesystem, audio-device, or network waits. No input event is discarded to catch up with rendering. Coalesce redraws, never accepted typing events.
- Use the same active clock for input, metrics, timer, and charts. Application overlays pause once at the outermost entry and resume once after the last blocking view closes.
- Authenticate nothing: this is a local single-user executable. Validate all new input at its boundary and show path/resource/action-specific errors. Local diagnostics contain no typing text unless the user explicitly requested it.
- Network rate limiting belongs exclusively to plan 016. History indexing and concurrency belong to plan 003. This phase consumes those policies only when they are prerequisites.
- Measure ready-screen <=250 ms, event-to-visible-input <=50 ms, and state-preserving resize <=100 ms on the documented reference setup when this phase changes those paths. Use 100 samples and report p50, p95, p99, and maximum. Slow terminal behavior is recorded rather than described as passing.

## Failure modes and required handling

| Path and failure | Handling and caller visibility | Required coverage |
| --- | --- | --- |
| Crash or disk full before commit | Previous files untouched; not stored/path/Retry visible | +5 fault injection points |
| Two instances save/delete concurrently | Directory lock and immutable IDs; Busy visible | +3 multi-process fixtures |
| Index missing or stale | Rebuild from records, no synthetic history; refresh message | +3 rebuild fixtures |
| Private text leaks through identity/fragments | Privacy projection before I/O, opaque ID/no path; diagnostics unavailable | +5 sentinel cases |
| Corrupt/unknown authoritative JSON | Preserve bytes and block writes to this store; typing usable | +3 version/read cases |

The coverage above is required new coverage, not a claim that these tests exist today. No silent, unhandled failure in a changed path satisfies this phase.

## Test plan

| Layer | Named coverage | Minimum new cases |
| --- | --- | --- |
| Unit | Schema, privacy, limits, immutable IDs | 14 |
| Integration | Faults, lock, index rebuild, legacy hashes | 11 |
| E2E | Save-relaunch-read and history-unavailable typing | 2 |

Use deterministic seeds, clocks, and temporary data directories. E2E cases mean scripted terminal/model journeys with real generation and storage where used, plus native-terminal inspection where capabilities cannot be simulated. Run `GOCACHE=/private/tmp/terminal-typer-go-cache make verify`, `GOCACHE=/private/tmp/terminal-typer-go-cache make smoke`, and `git diff --check`. Update `README.md` and `man.md` together when behavior becomes public. Regenerate with `make assets` when the manual or source assets change and verify `gzip -t tt.1.gz`. Do not edit `src/packed.go` by hand.

## Rollout and rollback

Create a new namespace; no in-place migration of old data. Keep history indefinite. Rollback reverts code and leaves history-v1 intact for the next compatible build. Unknown-version stores remain unopened. Test rollback by running the baseline executable against the same data root and checking legacy hashes. Never delete the new history directory as part of rollback.

Release, deployment, publishing binaries, and uploading user data require separate explicit human authorization. Update `CHANGELOG.md` when a release occurs, not while this proposal is written.

## Definition of done

- [ ] AC-1: Finish two tests, restart the process, and find exactly two records in chronological History. Saving one ID twice produces one file and one row.
- [ ] AC-2: A private stdin/file fixture containing a sentinel secret produces no sentinel, file path, or word fragment in any new store/index file; aggregates remain usable.
- [ ] AC-3: Injected write/sync/rename failures leave previous committed files unchanged, keep Results visible, and show `not stored` with Retry save.
- [ ] AC-4: Malformed or unknown-version records keep their bytes intact, make History unavailable with the exact affected path, and allow a fresh typing test without replacing the store.
- [ ] AC-5: POSIX directories are 0700 and files 0600. Windows uses inherited user-profile access controls and is verified in plan 017. Existing `.db`/`.errors`/settings hashes are unchanged.
