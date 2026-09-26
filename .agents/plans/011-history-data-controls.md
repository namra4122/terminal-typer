# Plan 011: Control local history and privacy

Status: proposed implementation specification. No feature in this document is implemented by the act of writing it. Source: [PRD](../../PRD.md). Planning baseline: 2026-09-26 working tree, including existing uncommitted `src/test.go`, `src/test_test.go`, and `src/tt.go` changes. Reinspect those files before implementation and preserve unrelated work.

## Goal and user job

Users can filter history, export retained data, inspect storage and confirm deletion without losing unrelated data.

As a terminal-heavy developer, I can exercise this capability entirely from the keyboard while keeping my input and progress local. This phase is one reviewable PR, estimated at 1-3 focused implementation sessions. The file table is the implementation boundary, including tests and public documentation. Generated outputs are declared separately.

## Decisions and assumptions

- Use UTC stored timestamps, displayed in local time with timezone. Time filters are last7/30/90 days or all, measured from an injectable current instant. Page size25. Default is regular completed/expired sessions, practice separate.
- Explicit approval is per resolved local resource SHA256 revision, not a path/name wildcard. Private-file/stdin fragment retention is a global saved bool initiallyfalse, with a confirmation describing local fragments. Snapshot consent at attempt start; it applies prospectively and never reconstructs past text.
- Deletion moves complete files into a transaction-specific trash directory before commit. Visible history excludes committed trash. Cleanup happens after commit; a cleanup failure says deletion succeeded with residual disk bytes, rather than claiming rollback.
- Go, Bubble Tea v2, Lip Gloss v2, the existing Makefile, and local-only application data are inherited from plan 001. No server, account, telemetry, executable plugins, or release action is part of this phase.

## Scope

- Filter chronological History by period, mode, configured length, pack, Normal difficulty and eligibility, with count/summary and selected Analysis. Add individual, all-practice and all-history confirmed deletion.
- Data shows actual root path, committed/trash/cache/legacy byte sizes, retention explanation, version and recovery guidance. Settings reset never deletes history.
- Export complete retained v1 history JSON and flat session CSV to an explicitly entered local destination. Never overwrite existing destinations without confirmation. Export is independent of invocation `-json`/`-csv`.
- Add approved-local-resource management and private fragment retention consent. Keep explicit legacy mistake output compatible. Recovery can export readable records with an explicit skipped-file list when the store is unavailable; never overwrite damaged data.

## Explicitly deferred scope

- History import/automatic repairs/cloud sync: post-v1.
- PB/trend projections refresh through the store generation contract and land in plan 012.

## Requirement coverage

UI-04, UI-08 Data; DATA-02 through DATA-12, DATA-15; CFG-10, CFG-11; PRACTICE-07; PB-05 deletion integration.

Every listed requirement is checked by the acceptance criteria and the named fixtures below. Shared IDs can have incremental coverage in several plans. The index maps the final release gates.

## Acceptance criteria

1. Every period/mode/length/pack/difficulty/eligibility filter matches exact fixture counts. Resetting settings changes0 history files.
2. JSON/CSV export includes only retained fields, a format version, no incidental private paths and no secret prompt sentinel; failed writes leave existing destination bytes unchanged.
3. Cancel all deletion scopes and preserve hashes. Commit each scope and remove exactly the confirmed IDs; interrupted precommit deletion restores all prior visible rows on recovery.
4. Deleting while another writer owns the store returns Busy/Retry rather than racing. Corrupt history remains intact and export explicitly lists skipped/unreadable records.
5. Private consent is off initially, requires confirmation, applies only to future started attempts, and can be revoked. Unknown local packs remain private until specific revision approval.

## Keyboard journey

1. Open History, filter via f, select a row and open Analysis.
2. Press Delete, inspect exact scope/count, confirm with explicit Delete button or Escape cancel.
3. Open Data, inspect path/bytes/privacy, choose Export and type a local destination.
4. Review overwrite/retention/approval consequences before confirming; return to the same screen on failure.

## Verified reusable implementation

- `src/db.go:31` separates legacy progress/mistakes/settings. Maintain that boundary.
- `src/tt.go:67` and `src/tt.go:82` define per-invocation formats, which this feature does not repurpose.
- Plan 003 supplies HistoryRecord/cache/lock; plan 005 supplies config transactions; plan 010 supplies safe historical Analysis.

Current-state citations refer to the inspected baseline, not hypothetical future line numbers. Files introduced by predecessor plans are cited by interface name below.

## Components and file boundary

| File | Change |
| --- | --- |
| `src/data.go` (new) | Filtering, explicit export/deletion/recovery and storage usage. |
| `src/data_test.go` (new) | Export schemas, failures, privacy and delete journal recovery. |
| `src/history.go` (plan 003) | Mutation generations and confirmed deletion transactions. |
| `src/config.go` (plan 005) | Prospective privacy/approval fields. |
| `src/app.go` (plan 001) | Filters, Data/export/confirmation views. |
| `src/app_test.go` (plan 001) | Cancelled/committed actions and context recovery. |
| `README.md:81`, `man.md` | Data paths, retention/export/privacy/deletion and rollback. |

## Data model and constraints

Add optional Configuration `privacy:{retainPrivateFragments:bool,approvedResources:[]Approval}`; Approval is `{kind:string(words/quotes),revision:string(SHA256),label:string,approvedUnixMS:int64}`. Missing privacy defaultsfalse/empty. Approval stores no path. Snapshot policy on Session and HistoryRecord as `retentionConsent:bool`; approved local records retain pack revision/label, still PB-ineligible.
HistoryQuery adds `SinceUnixMS *int64; Modes,Packs,Lengths []string; Difficulty string; Eligibility *bool`. Length key `duration:<ms>`/`count:<n>`/`passage:<length-band>`/`custom`. Conditions combine AND between categories, OR within each list.
Export JSON: `{exportVersion:1,exportedUnixMS:int64,metricVersions:[]string,records:[]HistoryRecord,skippedFiles:[]string}`. Normal export refuses unavailable store; explicit recovery export includes only validated files and lists relative failed filenames. CSV header is `export_version,id,finished_unix_ms,mode,pack_id,pack_revision,length_key,difficulty,punctuation,numbers,capitalization,outcome,practice,retry_of,eligible,exclusion_reasons,active_ms,pause_ms,wpm,raw_wpm,cpm,accuracy,consistency,correct,incorrect,extra,missed,total_errors,corrected_errors,uncorrected_errors`. Use encoding/csv quoting, blank for nullable metrics, reasons joined with `;`, metric numbers2 decimals. No word fragments in flat CSV.
Deletion journal: `{version:1,id:string,state:string(prepared/committed),ids:[]string,sourceGeneration:string}`. Transaction dir `<history>/trash/<id>/`; committed trash files are not history. An unfinished prepared journal restores original locations before writes resume. Unknown/conflicting recovery preserves all bytes and marks store unavailable.

## Go and CLI contracts

```go
func ExportHistory(root, destination, format string, overwrite bool) error
func DeleteHistory(root string, ids []string, expectedGeneration string) error
func StorageUsage(root string) (map[string]int64, error)
```
Deletion validates selected IDs/count against current generation under writer lock; `ErrStaleSelection` refreshes confirmation. Move selected files to trash, write/sync committed journal, invalidate index and emit new generation. If a precommit move fails, restore every moved file, preserving conflicting files and marking unavailable if restoration fails. Cleanup committed trash is best-effort with explicit residual usage/error. Export snapshots under read/commit validation, writes temp destination0600, Sync, and commits with no-overwrite semantics unless approved. Destination errors identify whether it changed. Export no unknown/private unretained fields.

There are no HTTP APIs in this phase unless an explicit network contract appears above. Return errors rather than panicking across the application boundary. Bubble Tea commands perform I/O and report immutable messages to the model. `Update` owns mutable UI/session state and does not wait on I/O.

## Dependencies and operation sequence

Required predecessor plans: 10

1. Load generation-tagged History, apply filters to index summaries, paginate and display sample/period summaries (count, mean speeds with comparable group disclosure).
2. Take confirmation snapshot, revalidate under lock, journal/move/commit, rebuild projections from surviving results, and restore a valid selection.
3. Export a consistent snapshot with filename destination and overwrite decision. Cache metadata never enters exported records.
4. Consent/approval changes use config commit then apply at the next session. Disabling retention affects new attempts; offer explicit deletion to remove previously retained data without implying automatic retroactive erasure.
5. Corrupt-history recovery reads files independently for explicit recovery export, logs only relative skipped filenames, and leaves the authoritative store intact.

## Non-functional and cross-cutting rules

- Typing input processing performs no filesystem, audio-device, or network waits. No input event is discarded to catch up with rendering. Coalesce redraws, never accepted typing events.
- Use the same active clock for input, metrics, timer, and charts. Application overlays pause once at the outermost entry and resume once after the last blocking view closes.
- Authenticate nothing: this is a local single-user executable. Validate all new input at its boundary and show path/resource/action-specific errors. Local diagnostics contain no typing text unless the user explicitly requested it.
- Network rate limiting belongs exclusively to plan 016. History indexing and concurrency belong to plan 003. This phase consumes those policies only when they are prerequisites.
- Measure ready-screen <=250 ms, event-to-visible-input <=50 ms, and state-preserving resize <=100 ms on the documented reference setup when this phase changes those paths. Use 100 samples and report p50, p95, p99, and maximum. Slow terminal behavior is recorded rather than described as passing.

## Failure modes and required handling

| Path and failure | Handling and caller visibility | Required coverage |
| --- | --- | --- |
| Deletion partially moves files | Prepared journal rollback, preserve conflicts; unavailable if recovery fails | +5 crash cases |
| Export overwrites without consent | No-overwrite commit/default, explicit overwrite transaction | +3 destination cases |
| Stale confirmation deletes new data | Expected generation/ID validation; refresh confirmation | +2 concurrent cases |
| Privacy approval follows renamed/shadowed content | Hash-specific approval and start snapshot | +3 consent cases |

The coverage above is required new coverage, not a claim that these tests exist today. No silent, unhandled failure in a changed path satisfies this phase.

## Test plan

| Layer | Named coverage | Minimum new cases |
| --- | --- | --- |
| Unit | Filter categories, JSON/CSV exact schemas and consent | 18 |
| Integration | Delete/export faults, locks and unknown recovery | 12 |
| E2E | Export/delete/cancel and prospective consent | 3 |

Use deterministic seeds, clocks, and temporary data directories. E2E cases mean scripted terminal/model journeys with real generation and storage where used, plus native-terminal inspection where capabilities cannot be simulated. Run `GOCACHE=/private/tmp/terminal-typer-go-cache make verify`, `GOCACHE=/private/tmp/terminal-typer-go-cache make smoke`, and `git diff --check`. Update `README.md` and `man.md` together when behavior becomes public. Regenerate with `make assets` when the manual or source assets change and verify `gzip -t tt.1.gz`. Do not edit `src/packed.go` by hand.

## Rollout and rollback

Add optional privacy fields without changing known-version required keys. Deletion rollback is possible only before commit; after user-confirmed durable deletion, revert code does not recreate records. Explicit exports are user-chosen backups. Document this limitation and test journal recovery. Old `.errors`/`.db` remain unchanged and are shown separately, never part of rich-history deletion. No user data is uploaded.

Release, deployment, publishing binaries, and uploading user data require separate explicit human authorization. Update `CHANGELOG.md` when a release occurs, not while this proposal is written.

## Definition of done

- [ ] AC-1: Every period/mode/length/pack/difficulty/eligibility filter matches exact fixture counts. Resetting settings changes0 history files.
- [ ] AC-2: JSON/CSV export includes only retained fields, a format version, no incidental private paths and no secret prompt sentinel; failed writes leave existing destination bytes unchanged.
- [ ] AC-3: Cancel all deletion scopes and preserve hashes. Commit each scope and remove exactly the confirmed IDs; interrupted precommit deletion restores all prior visible rows on recovery.
- [ ] AC-4: Deleting while another writer owns the store returns Busy/Retry rather than racing. Corrupt history remains intact and export explicitly lists skipped/unreadable records.
- [ ] AC-5: Private consent is off initially, requires confirmation, applies only to future started attempts, and can be revoked. Unknown local packs remain private until specific revision approval.
