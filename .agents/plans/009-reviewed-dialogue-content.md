# Plan 009: Ship a reviewed 100-passage catalog

Status: proposed implementation specification. No feature in this document is implemented by the act of writing it. Source: [PRD](../../PRD.md). Planning baseline: 2026-09-26 working tree, including existing uncommitted `src/test.go`, `src/test_test.go`, and `src/tt.go` changes. Reinspect those files before implementation and preserve unrelated work.

## Goal and user job

A user can choose at least 100 reviewed, redistributable passages across genres and lengths.

As a terminal-heavy developer, I can exercise this capability entirely from the keyboard while keeping my input and progress local. This phase is one reviewable PR, estimated at 1-3 focused implementation sessions. The file table is the implementation boundary, including tests and public documentation. Generated outputs are declared separately.

## Decisions and assumptions

- Use newly authored original material for the first official 100 passages, with contributor licensing under the repository's MIT license and explicit authorship provenance. Public-domain/permissive additions are allowed only with individually documented rights; recognizable modern movie/TV quotations are not required.
- Curate5 genre collections of 20 passages within one official pack: comedy, drama, science-fiction, fantasy, workplace. Each collection has 8 short,8 medium,4 long passages using plan 008 bands. Reviewers are real named contributors; generation/import alone never sets reviewed=true.
- Go, Bubble Tea v2, Lip Gloss v2, the existing Makefile, and local-only application data are inherited from plan 001. No server, account, telemetry, executable plugins, or release action is part of this phase.

## Scope

- Author/catalog 100 distinct original passages, verify safe-workplace policy, and document author/reviewer/date/rights for each.
- Replace the dev-only default catalog with official collection choices using the existing manifest loader/queue. Keep the developer fixture available only to tests.
- Add automated release-gate validation for count, unique IDs/text hashes, required metadata, length distribution, and signed review ledger consistency. Require human review for authenticity, safety and rights.

## Explicitly deferred scope

- Famous licensed dialogue, new languages, additional collections: post-v1.
- No new runtime screen or storage model is introduced here.

## Requirement coverage

CONTENT-02 through CONTENT-06, CONTENT-08, CONTENT-09; A-11 content release gate.

Every listed requirement is checked by the acceptance criteria and the named fixtures below. Shared IDs can have incremental coverage in several plans. The index maps the final release gates.

## Acceptance criteria

1. The distributed official catalog has >=100 unique passages, one official pack with 5 initial genre collections of 20 each and the8/8/4 length distribution, excluding the dev fixture.
2. Every passage has an actual author, reviewer, review date, license and provenance, with a matching review-ledger row. No unresolved rights claim is marked approved.
3. Content validation rejects duplicate normalized text hashes, missing metadata, prohibited terminal controls, invalid counts and unreviewed official entries.
4. A keyboard journey browses each collection, displays full attribution and completes one passage without changing authored text.
5. Human review confirms no slurs, explicit sexual content, graphic violence, harassment or shock material. Release remains gated while any row lacks review.

## Keyboard journey

1. Open content selection, choose an official collection, filter length/difficulty and preview.
2. Start and finish a passage.
3. Inspect its author/title/speaker/license/provenance and select Next.

## Verified reusable implementation

- `quotes/en` exists but its attribution-only format does not establish passage redistribution rights (`src/quotetest.go:8`). Preserve it as legacy content rather than asserting it satisfies review gates.
- `Makefile:38` and `scripts/pack` already embed source directories. Plan 007 includes catalog/ and plan 008 defines manifests/queue.

Current-state citations refer to the inspected baseline, not hypothetical future line numbers. Files introduced by predecessor plans are cited by interface name below.

## Components and file boundary

| File | Change |
| --- | --- |
| `catalog/dialogue-v1.json` (new) | 100 reviewed original passages in 5 genre collections. |
| `catalog/REVIEW.md` (new) | Human rights/safety ledger and original-author license declarations. |
| `src/dialogue_test.go` (plan 008) | Strict official catalog release-gate checks. |
| `README.md:67`, `man.md` | Advertise reviewed collections only after ledger completion. |

## Data model and constraints

Use the complete plan 008 manifest and Passage shape unchanged. REVIEW.md rows are `passageId | author | reviewer | reviewedDate | license | provenance | safetyApproved`; safetyApproved is literal yes/no. Match IDs across manifests and ledger. Pack revisions are SHA256 of canonical JSON excluding revision itself. Document the canonical encoding (UTF-8 compact JSON with lexicographically sorted object keys and authored array order). No new persistent schema.
The one official pack ID is `tt-dialogue-v1`, display name `Terminal Typer Dialogues`, language `en`, MIT, with 100 passage entries. Genre values are comedy,drama,science-fiction,fantasy,workplace and serve as collection selectors. Passage IDs are `<genre>-001` through `<genre>-010`; entries001-008 short,009-006 medium,007-010 long. Difficulty values: short easy, medium standard, long challenging. This is declared authored metadata, not an inferred engine difficulty. The developer fixture is excluded from the official pack registry and release count.


## Go and CLI contracts

```go
func ValidateOfficialCatalog(packs []DialoguePack, ledger []ReviewEntry) error
```
`ReviewEntry` has string fields matching the ledger plus `SafetyApproved bool`. Errors name pack/passage/field. Validation is a Go test invoked by make verify, not a second build system. Test reference counts >=100 and separately check the initial five-collection distribution. Existing `LoadDialogue`, `FilterPassages`, `ReservePassage` contracts are consumed unchanged.

There are no HTTP APIs in this phase unless an explicit network contract appears above. Return errors rather than panicking across the application boundary. Bubble Tea commands perform I/O and report immutable messages to the model. `Update` owns mutable UI/session state and does not wait on I/O.

## Dependencies and operation sequence

Required predecessor plans: 8

1. Author original material, add metadata and license statements, then obtain actual individual review before setting approval.
2. Run mechanical checks and resolve every named failure. Human reviewers check rights/tone/readability, rather than passing those claims from a keyword filter.
3. Regenerate embedded assets with make assets, browse each collection through the real app, and verify attribution. Archive evidence in REVIEW.md.
4. If reviewer availability blocks approval, keep the catalog clearly unreviewed and report CONTENT-02 incomplete; do not release or claim v1 completion.

## Non-functional and cross-cutting rules

- Typing input processing performs no filesystem, audio-device, or network waits. No input event is discarded to catch up with rendering. Coalesce redraws, never accepted typing events.
- Use the same active clock for input, metrics, timer, and charts. Application overlays pause once at the outermost entry and resume once after the last blocking view closes.
- Authenticate nothing: this is a local single-user executable. Validate all new input at its boundary and show path/resource/action-specific errors. Local diagnostics contain no typing text unless the user explicitly requested it.
- Network rate limiting belongs exclusively to plan 016. History indexing and concurrency belong to plan 003. This phase consumes those policies only when they are prerequisites.
- Measure ready-screen <=250 ms, event-to-visible-input <=50 ms, and state-preserving resize <=100 ms on the documented reference setup when this phase changes those paths. Use 100 samples and report p50, p95, p99, and maximum. Slow terminal behavior is recorded rather than described as passing.

## Failure modes and required handling

| Path and failure | Handling and caller visibility | Required coverage |
| --- | --- | --- |
| Imported passage has unclear rights | Exclude from official approved count until evidence is recorded | Ledger inspection plus mechanical missing-provenance test |
| Unreviewed generated text counted | Cross-check real review ledger; release gate fails visibly | +3 review fixtures |
| Duplicate IDs/text | Validator names both entries, no distribution | +2 duplicate fixtures |
| Embedded catalog stale | Regenerate from source and compare loaded hashes | +1 integration journey |

The coverage above is required new coverage, not a claim that these tests exist today. No silent, unhandled failure in a changed path satisfies this phase.

## Test plan

| Layer | Named coverage | Minimum new cases |
| --- | --- | --- |
| Unit | Official count/distribution/metadata/ledger/duplicate checks | 8 |
| Integration | Embedded loader hashes/attribution | 2 |
| E2E | One passage per collection | 5 |

Use deterministic seeds, clocks, and temporary data directories. E2E cases mean scripted terminal/model journeys with real generation and storage where used, plus native-terminal inspection where capabilities cannot be simulated. Run `GOCACHE=/private/tmp/terminal-typer-go-cache make verify`, `GOCACHE=/private/tmp/terminal-typer-go-cache make smoke`, and `git diff --check`. Update `README.md` and `man.md` together when behavior becomes public. Regenerate with `make assets` when the manual or source assets change and verify `gzip -t tt.1.gz`. Do not edit `src/packed.go` by hand.

## Rollout and rollback

This content-only PR touches5 files and adds no service. Author and individually review the five20-passage batches in 1-3 focused sessions, then land one verified catalog PR. Generated outputs are packed.go and manual. Rollback reverts official pack assets, preserving recorded pack revisions/attribution in history; missing historical text becomes unavailable. No publishing or release is authorized. If actual individual review is unfinished, the implementation/release gate remains incomplete with the exact outstanding ledger IDs; it must not be represented as shipped reviewed content.

Release, deployment, publishing binaries, and uploading user data require separate explicit human authorization. Update `CHANGELOG.md` when a release occurs, not while this proposal is written.

## Definition of done

- [ ] AC-1: The distributed official catalog has >=100 unique passages, one official pack with 5 initial genre collections of 20 each and the8/8/4 length distribution, excluding the dev fixture.
- [ ] AC-2: Every passage has an actual author, reviewer, review date, license and provenance, with a matching review-ledger row. No unresolved rights claim is marked approved.
- [ ] AC-3: Content validation rejects duplicate normalized text hashes, missing metadata, prohibited terminal controls, invalid counts and unreviewed official entries.
- [ ] AC-4: A keyboard journey browses each collection, displays full attribution and completes one passage without changing authored text.
- [ ] AC-5: Human review confirms no slurs, explicit sexual content, graphic violence, harassment or shock material. Release remains gated while any row lacks review.
