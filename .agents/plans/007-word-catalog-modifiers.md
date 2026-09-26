# Plan 007: Choose word packs and independent modifiers

Status: proposed implementation specification. No feature in this document is implemented by the act of writing it. Source: [PRD](../../PRD.md). Planning baseline: 2026-09-26 working tree, including existing uncommitted `src/test.go`, `src/test_test.go`, and `src/tt.go` changes. Reinspect those files before implementation and preserve unrelated work.

## Goal and user job

Users can select every valid existing word pack and generate predictable punctuation, number, and capitalization variations.

As a terminal-heavy developer, I can exercise this capability entirely from the keyboard while keeping my input and progress local. This phase is one reviewable PR, estimated at 1-3 focused implementation sessions. The file table is the implementation boundary, including tests and public documentation. Generated outputs are declared separately.

## Decisions and assumptions

- Catalog entries keep existing identifiers: 1000en/200en, de/de-ch, es, fi, fr, it, pl, pt, ru, sv. Display names are English 1k/200, German/German (Swiss), Spanish, Finnish, French, Italian, Polish, Portuguese, Russian, Swedish. English1k remains default.
- All toggles work for these packs. Use Unicode letter detection/case mapping without transliteration. Numbers use ASCII decimal. Punctuation uses ASCII `. , ? ! : ;`. Exact token-position rules below are reversible product assumptions.
- Metadata uses a new embedded `catalog/words.json`; extend the pack command input rather than mixing metadata into `words/` and polluting `-list words`. Historical resource licenses are recorded honestly as SPDX identifier if supported by provenance, otherwise `NOASSERTION` with maintainer review notes. Do not invent attribution.
- Go, Bubble Tea v2, Lip Gloss v2, the existing Makefile, and local-only application data are inherited from plan 001. No server, account, telemetry, executable plugins, or release action is part of this phase.

## Scope

- Search/display all nonempty valid UTF-8 bundled word resources with language/variant labels. Local resource shadowing retains lookup order and becomes private unless explicitly approved.
- Validate a word list into distinct nonempty whitespace tokens; require >=2 distinct entries for random generation. A one-entry legacy resource remains deterministic and cannot spin in adjacent-duplicate rejection; show the unavoidable duplicate warning.
- Expose three independent modifiers for generated TUI word tests, persist on Start, display in test/result context, and use them in comparison identity. Custom/quote/file/stdin text is never modified.
- Prevent adjacent duplicate final tokens and base words across timed buffer boundaries; choose another eligible base token with bounded attempts then deterministic next distinct entry.

## Explicitly deferred scope

- New word languages, code/snippet packs, difficulty, layouts and Funbox: post-v1.
- Quote/dialogue catalog: plan 008. Rich eligibility projections: plan 012.

## Requirement coverage

CONTENT-01, CONTENT-05, CONTENT-07, CONTENT-09; MODE-01, MODE-02; UI-10; CFG-03; CLI-06; PB-01 (configuration identity).

Every listed requirement is checked by the acceptance criteria and the named fixtures below. Shared IDs can have incremental coverage in several plans. The index maps the final release gates.

## Acceptance criteria

1. Every valid existing bundled word identifier appears with the specified display name/language. `-list words` remains the word-resource list without catalog metadata filenames.
2. All 8 toggle combinations produce the exact documented fixture; selected count is unchanged, and original quote/private text is unchanged.
3. 100 seeded 500-word tests and 10 timed refills have no adjacent duplicate base/final tokens when >=2 distinct valid tokens exist.
4. Empty, malformed UTF-8 and one-token lists return the stated validation behavior without panic or unbounded loop. Explicit paths still outrank local/system/embedded resources.
5. Browsing/toggle previews do not save, while Start persists modifiers and pack, clears inherited legacy modifiers, and keeps source privacy correct.

## Keyboard journey

1. Open Select content, search a language and see identifier/origin.
2. Choose it, enable any modifier combination in Configure or quick picker, and preview 12 words.
3. Start and read pack/modifier context above the prompt.
4. Finish and verify result configuration records the chosen combination.

## Verified reusable implementation

- `src/util.go:201` resolves explicit/local/system/embedded sources and `src/wordtest.go:5` parses word lists. Retain their source identifiers.
- `src/util.go:83` prevents adjacent duplicates but can loop on a single distinct word; bound this behavior in the new validated generator.
- `src/test.go:27` already has three boolean modifier fields. Plans005-006 provide validated configuration/pickers and timed refill.

Current-state citations refer to the inspected baseline, not hypothetical future line numbers. Files introduced by predecessor plans are cited by interface name below.

## Components and file boundary

| File | Change |
| --- | --- |
| `src/catalog.go` (new) | Manifest/origin loader, validated word tokens and modifiers. |
| `src/catalog_test.go` (new) | Catalog, all modifier combinations and generator boundaries. |
| `catalog/words.json` (new) | Exact metadata for existing resources. |
| `Makefile:41` | Include catalog/ in embedded source inputs. |
| `src/test.go:102` | Route TUI generation through the catalog without replacing legacy closures. |
| `src/app.go` (plan 001) | Content picker labels and modifier preview. |
| `README.md:56`, `man.md` | Available packs/modifiers and local origin behavior. |

## Data model and constraints

Manifest: `{version:1,packs:[]WordPack}`. `WordPack` is `{id:string,displayName:string,language:string(BCP47),variant:string,type:string(words),resource:string,license:string,source:string,attribution:string,maintainerNotes:string,revision:string(SHA256 resource)}`. Source is a repository-relative asset path or provenance URL, never an incidental user's path. Local manifests may add metadata but cannot declare public/privacy approval by themselves.
`ResourceOrigin` is the plan 003 type `{Kind:string,PackID:string,Path:string,Embedded:bool,Revision:string}` in memory. Persist pack ID/revision only for approved/public catalog content. Display origin as bundled/user/system/explicit path, with full paths confined to current UI. Use local RNG per prompt; persist no seed that reconstructs a private prompt.

## Go and CLI contracts

```go
func LoadWordCatalog() ([]WordPack, error)
func ResolveResource(kind, name string) ([]byte, ResourceOrigin, error) // consume plan 003 resolver unchanged
func ValidateWordTokens(data []byte) ([]string, error)
func GenerateWords(pack WordPack, count int, previous string, mods TestModifiers, rng *rand.Rand) ([]string, error)
```
Apply after base-word selection: Numbers replaces zero-based positions divisible by 5 with a random integer0..9999, rejecting a consecutive equal final token. Capitalization uppercases the first Unicode letter at positions divisible by 4; tokens without letters are unchanged. Punctuation appends one mark at positions divisible by 3, cycling `. , ? ! : ;` by punctuation occurrence. Operation order: numbers -> capitalization -> punctuation. Count is always the number of whitespace-delimited tokens.
Fixture base `[alpha beta gamma delta epsilon zeta]`, RNG number outputs42,314: none=`alpha beta gamma delta epsilon zeta`; punctuation=`alpha. beta gamma delta, epsilon zeta`; capitalization=`Alpha beta gamma delta Epsilon zeta`; numbers=`42 beta gamma delta epsilon 314`; all=`42. beta gamma delta, Epsilon 314`. Other combinations are the corresponding compositional outputs. Number-only short tests can contain no letters; disclose this in preview. Preserve authored/resource text bytes before canonical reflow.

There are no HTTP APIs in this phase unless an explicit network contract appears above. Return errors rather than panicking across the application boundary. Bubble Tea commands perform I/O and report immutable messages to the model. `Update` owns mutable UI/session state and does not wait on I/O.

## Dependencies and operation sequence

Required predecessor plans: 6

1. Read metadata and resolve each actual resource through retained lookup precedence. Validate UTF-8/tokens and compute revision from resolved bytes, so shadowed content is not mistaken for embedded PB-approved content.
2. Generate base tokens with bounded duplicate avoidance, transform at absolute token positions (including refill offsets), and return tokens to the shared engine. Never modify private custom or authored quote source routes.
3. Preview uses a separate RNG so browsing does not consume the session's prompt queue. Start commits catalog ID/modifiers and generates from a fresh per-test RNG.
4. Catalog errors identify resource/origin/remedy. Exclude invalid catalog entries with a visible list warning while leaving legacy explicit invocation errors intact.

## Non-functional and cross-cutting rules

- Typing input processing performs no filesystem, audio-device, or network waits. No input event is discarded to catch up with rendering. Coalesce redraws, never accepted typing events.
- Use the same active clock for input, metrics, timer, and charts. Application overlays pause once at the outermost entry and resume once after the last blocking view closes.
- Authenticate nothing: this is a local single-user executable. Validate all new input at its boundary and show path/resource/action-specific errors. Local diagnostics contain no typing text unless the user explicitly requested it.
- Network rate limiting belongs exclusively to plan 016. History indexing and concurrency belong to plan 003. This phase consumes those policies only when they are prerequisites.
- Measure ready-screen <=250 ms, event-to-visible-input <=50 ms, and state-preserving resize <=100 ms on the documented reference setup when this phase changes those paths. Use 100 samples and report p50, p95, p99, and maximum. Slow terminal behavior is recorded rather than described as passing.

## Failure modes and required handling

| Path and failure | Handling and caller visibility | Required coverage |
| --- | --- | --- |
| Single distinct word loops | Deterministic duplicate-warning generation for legacy; TUI pack disabled | +2 degenerate lists |
| Shadowed pack gets public retention/PB | Resolve origin/revision before privacy projection | +3 precedence cases |
| Modifiers change quote/private text | Gate by generated TUI words mode and origin | +4 invariance cases |
| Refill resets duplicate/position state | Carry last token and absolute position | +3 refill cases |

The coverage above is required new coverage, not a claim that these tests exist today. No silent, unhandled failure in a changed path satisfies this phase.

## Test plan

| Layer | Named coverage | Minimum new cases |
| --- | --- | --- |
| Unit | 8 combinations, Unicode, validation, duplicate/refill rules | 20 |
| Integration | Catalog embed/list/origin and private projection | 5 |
| E2E | Select/toggle/preview/start | 1 |

Use deterministic seeds, clocks, and temporary data directories. E2E cases mean scripted terminal/model journeys with real generation and storage where used, plus native-terminal inspection where capabilities cannot be simulated. Run `GOCACHE=/private/tmp/terminal-typer-go-cache make verify`, `GOCACHE=/private/tmp/terminal-typer-go-cache make smoke`, and `git diff --check`. Update `README.md` and `man.md` together when behavior becomes public. Regenerate with `make assets` when the manual or source assets change and verify `gzip -t tt.1.gz`. Do not edit `src/packed.go` by hand.

## Rollout and rollback

Extend embedded assets with catalog metadata and regenerate through `make assets`; generated outputs are `src/packed.go` and `tt.1.gz`. Configuration fields already exist in v2. Rollback restores old embedded assets and UI while retaining readable modifier values in saved/history data; baseline legacy inputs continue with modifiers off. Do not remove existing word assets.

Release, deployment, publishing binaries, and uploading user data require separate explicit human authorization. Update `CHANGELOG.md` when a release occurs, not while this proposal is written.

## Definition of done

- [ ] AC-1: Every valid existing bundled word identifier appears with the specified display name/language. `-list words` remains the word-resource list without catalog metadata filenames.
- [ ] AC-2: All 8 toggle combinations produce the exact documented fixture; selected count is unchanged, and original quote/private text is unchanged.
- [ ] AC-3: 100 seeded 500-word tests and 10 timed refills have no adjacent duplicate base/final tokens when >=2 distinct valid tokens exist.
- [ ] AC-4: Empty, malformed UTF-8 and one-token lists return the stated validation behavior without panic or unbounded loop. Explicit paths still outrank local/system/embedded resources.
- [ ] AC-5: Browsing/toggle previews do not save, while Start persists modifiers and pack, clears inherited legacy modifiers, and keeps source privacy correct.
