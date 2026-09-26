# Terminal Typer v1 implementation plans

These are proposed, agent-ready implementation specifications derived from [PRD.md](../../PRD.md), after reading the current working tree on 2026-09-26. They change no executable behavior by themselves. Implement one bounded phase per PR in the dependency order below and rerun the canonical checks at each step. Public README/manual continue to describe the executable, not these proposals.

## Authority and approved engineering decisions

The user explicitly selected Go and [Charm](https://charm.land/) for the TUI, superseding PRD Section 2's existing-tcell recommendation. The user then confirmed JSON-per-session storage with atomic writes/rebuildable indexing, stable Charm v2 with its required Go minimum, and GitHub Releases from this fork `namra4122/terminal-typer`. These are planning decisions, not approval to publish or release.

Plan 001 pins Go 1.26.0, Bubble Tea 2.0.10, Lip Gloss 2.0.6 and uniseg 0.4.7. Plan 006 adds Bubbles 2.2.1. Existing beep is reused. Standard-library JSON/filesystem storage needs no database dependency. The inspected local compiler is Go 1.27.1. Version pin evidence: [Bubble Tea](https://raw.githubusercontent.com/charmbracelet/bubbletea/v2.0.10/go.mod), [Lip Gloss](https://raw.githubusercontent.com/charmbracelet/lipgloss/v2.0.6/go.mod), [Bubbles](https://raw.githubusercontent.com/charmbracelet/bubbles/v2.2.1/go.mod). Revalidate pinned dependencies before coding if this schedule is used after the planning date; an upgrade is an explicit change to plan 001, not an implementer preference.

The migration keeps the tcell route until replacement capabilities pass concrete fixtures. The contributor opt-in starts in 001, bare/saved launches switch in 005, and all legacy sources move to Charm in 015. Every phase leaves both the current public application and its newly demonstrable capability runnable. No schema-only or renderer-only dead-end phase is required.

## Inspected baseline and reuse map

| Current implementation | Evidence | Planning consequence |
| --- | --- | --- |
| Go 1.17/tcell 1.4/beep 1.1 | `go.mod:1` | Charm is an approved migration with Go minimum change, not an existing feature. |
| Legacy source resolver and stable generated Test | `src/test.go:63`, `src/test.go:93`, `src/test.go:102`, `src/test.go:141` | Reuse the user's uncommitted foundation and generation closures. Eligibility flags here are placeholders, not policy. |
| 50 words default and visited CLI flags | `src/tt.go:433`, `src/tt.go:485` | Default/saved modes need explicit compatibility resolution. |
| Word/quote/file/stdin generators | `src/wordtest.go:5`, `src/quotetest.go:8`, `src/filetest.go:8`, `src/datatest.go:3` | Keep legacy formats/progress/raw precedence. Current quote choice is random, not a durable shuffled queue. |
| Six settings, atomic save and dirty merges | `src/settings.go:24`, `src/settings.go:133`, `src/settings.go:184`, `src/settings.go:384`, `src/settings.go:699` | Reuse pure validation/formatting/persistence logic and migrate the document explicitly. |
| Direct local progress/mistake writes | `src/db.go:14`, `src/db.go:46`, `src/tt.go:336` | Preserve data paths/stores, add safe writes without fabricating result history. |
| Final scalar counts/basic JSON/CSV | `src/typer.go:262`, `src/tt.go:27`, `src/tt.go:63`, `src/tt.go:632` | Introduce one versioned rich metric definition and preserve scripting shape. |
| Settings pauses, but other controls start clock | `src/typer.go:418`, `src/typer.go:436` | Classify accepted input before starting active time. |
| Resize returns/restarts run | `src/typer.go:407`, `src/tt.go:649` | State-preserving resize needs a session model, not another reflow call. |
| Theme roles, cell-width helpers/resource lookup | `src/theme.go:10`, `src/layout.go:161`, `src/layout.go:226`, `src/util.go:201` | Reuse concepts/formats; replace renderer with Charm. |
| Canonical maintenance/generation and Linux CI | `Makefile:19`, `Makefile:38`, `.github/workflows/verify.yml:1` | Retain Makefile, generated asset flow and extend verification only when needed. |

At planning time `src/tt.go` was modified and `src/test.go`, `src/test_test.go`, and `PRD.md` were untracked. The previous plans have been discarded. At the owner's request, this replacement schedule starts at 001 and ends at 017. The index and every plan reference use this new numbering. Historical records remain recoverable from Git. The PRD's references to the discarded roadmap describe that earlier set, not these newly numbered plans; PRD.md is left unchanged.

## Working increments

### [001: A real typing test on Charm](001-charm-typing-skeleton.md)

An opt-in Charm screen runs a real English word-count test from generation through correction, completion, and a basic result. Required predecessors: inspected checkout.

### [002: Reproducible measurements on Results](002-metrics-results.md)

A completed Charm test shows one deterministic metric definition with clear Retry and Next behavior. Required predecessors: 1.

### [003: Completed results survive restart](003-durable-local-history.md)

Completed Charm tests appear in a durable local History list without retaining private input text. Required predecessors: 2.

### [004: Review and practice measured weaknesses](004-weakness-practice.md)

After a real test, a user can review measured weaknesses, complete a short targeted drill, and compare practiced items. Required predecessors: 3.

### [005: Launch the remembered test immediately](005-remembered-test-modes.md)

Bare tt starts a fresh saved test, defaulting to English 1k for 30 seconds, with intentional test configuration saved on Start. Required predecessors: 4.

### [006: Reach every implemented workflow by keyboard](006-keyboard-command-routing.md)

A searchable command palette and contextual quick pickers provide a consistent route to every implemented screen. Required predecessors: 5.

### [007: Choose word packs and independent modifiers](007-word-catalog-modifiers.md)

Users can select every valid existing word pack and generate predictable punctuation, number, and capitalization variations. Required predecessors: 6.

### [008: Browse authored passages without repeats](008-dialogue-catalog-queue.md)

Users can browse passage metadata and complete authored text using a persistent shuffled queue. Required predecessors: 7.

### [009: Ship a reviewed 100-passage catalog](009-reviewed-dialogue-content.md)

A user can choose at least 100 reviewed, redistributable passages across genres and lengths. Required predecessors: 8.

### [010: Explain a completed session](010-detailed-analysis.md)

Detailed Analysis shows timing, character outcomes, words and pairs with honest unavailable states for historical/private data. Required predecessors: 8, 4.

### [011: Control local history and privacy](011-history-data-controls.md)

Users can filter history, export retained data, inspect storage and confirm deletion without losing unrelated data. Required predecessors: 10.

### [012: Compare progress and personal records](012-comparable-progress-records.md)

Progress shows comparable trends and reproducible personal records from surviving eligible regular history. Required predecessors: 11.

### [013: Use accessible native themes and Focus](013-native-themes-focus.md)

Users can preview native/existing themes and enable a focused, accessible layout at every supported size. Required predecessors: 12.

### [014: Enable optional nonblocking feedback](014-optional-audio-feedback.md)

Users can enable distinct key/error/completion/PB feedback while audio failure never interrupts typing. Required predecessors: 13.

### [015: Run legacy custom input through Charm](015-legacy-charm-parity.md)

Existing file, stdin, resource and scripting invocations use the Charm application with preserved CLI contracts. Required predecessors: 14.

### [016: Check releases only with consent](016-consent-release-checks.md)

Users can opt into quiet asynchronous stable-release notifications or explicitly request a manual check. Required predecessors: 15.

### [017: Verify the v1 experience on supported terminals](017-platform-performance-acceptance.md)

The complete keyboard workflow is verified on Linux, macOS and Windows Terminal with measured performance and recoverable local data. Required predecessors: 9, 16.

## Dependency order and rationale

```text
001 Charm typing
  ->002 metrics/results
  ->003 durable history
  ->004 basic practice
  ->005 remembered modes/default
  ->006 keyboard palette
  ->007 word catalog/modifiers
  ->008 dialogue catalog/queue
       +->009 reviewed100-passage content ------------------+
       |                                                   |
       +->010 Analysis ->011 Data ->012 Progress            |
              ->013 Presentation ->014 Audio ->015 CLI     |
                  ->016 consent updates -------------------+
                                                           |
                                                           v
                                                      017 v1 gates
```

Session/input semantics precede measurement so each screen shares a clock and error model. Measurements precede history so private retention and comparable fields have one format. History enables practice early; this brings the complete typing-to-results-to-practice path into the first four phases. Saved modes precede palette/modifiers to prevent remembered settings from changing legacy invocations. Catalog/queue precedes official content and historical metadata. Analysis precedes Data and comparable progress so deletion/export reuse the same projections. Legacy cutover waits for presentation/audio/source parity. Update checks wait for output isolation. Final platform acceptance waits for both the reviewed content lane and all runtime capabilities; changing this order would permit an incomplete catalog, schema-dependent consumers, or incorrect compatibility/records claims.

## Parallel work opportunities

Lane A:001 ->002 ->003 ->004 ->005 ->006 ->007 ->008 ->010 ->011 ->012 ->013 ->014 ->015 ->016, sequential because these phases share session/root-model contracts.

Lane B:009 after 008, in a separate worktree from 010-016. Content/ledger files and dialogue catalog tests are disjoint from the runtime lane. Both lanes touch README/manual, so defer or isolate those documentation hunks and regenerate packed/manual outputs only at integration.017 waits for both lanes. This describes future implementation opportunities; no sub-agents were spawned for this planning task.

## Release gate ownership

| PRD gate | Owning plans | Required evidence |
| --- | --- | --- |
| A-01 | 005,016 | Fresh default timing/no-network and post-result consent |
| A-02 | 005-008 | Remembered configuration, cancel/preview and fresh launch |
| A-03 | 004-008 | Presets/bounds/quotes/timed refill/practice |
| A-04 | 001,006,013 | Clock-preserving overlay/resize journeys |
| A-05 | 001-004,012 | Error replay, timeout, retry identity and record count |
| A-06 | 006,010-016 | Keyboard route/focus audit for all screens |
| A-07 | 004,010,011 | Practice evidence/private/sparse scenarios |
| A-08 | 011,012 | History filters, comparable trends and PB deletion |
| A-09 | 003,011 | Export/deletion schemas and private sentinel checks |
| A-10 | 003,005,008,011,015 | Fault/unknown-version/journal/rollback preservation |
| A-11 | 007-009 | Reviewed 100 catalog, queues, attribution and legacy formats |
| A-12 | 001,010,013,014 | Size/capability/Unicode/Focus/motion/audio matrix |
| A-13 | 005,015 | All legacy CLI/source/output fixtures and documented differences |
| A-14 | 016 | Consent/due/request/no-install transport fixtures |
| A-15 | 017 | Native platform/performance evidence and canonical checks |

Plan 017 runs every gate. Requirement IDs in each phase define incremental unit/integration/journey coverage; release acceptance requires all applicable coverage, not a sample of gates. Proposed test counts are minimum new fixture counts, not tests claimed to exist. Shared-state rollback is stated per phase. No historical version is bumped or CHANGELOG release entry fabricated during planning.

## Scope intentionally outside v1

| PRD direction | Boundary and next engineering spec |
| --- | --- |
| 5k/10k, Hinglish and code/snippet packs | Language/token rules, rights and manifests; preserve current legacy custom input in 015. |
| Dedicated code/custom TUI | Source-browsing/privacy/line-layout spec; current files/stdin remain supported. |
| Advanced adaptive/symbol/pair training | Evidence/ranking/baseline spec after 004/010. |
| Expert/Master, blind/no-correction, thresholds | Explicit failure-status/eligibility and rules spec; v1 advertises Normal only. |
| Layouts/keyboard/heatmaps | Logical-versus-physical mapping and versioned analytics spec. |
| Presets and --preset | Schema/precedence/import/replace/rollback spec after 005. |
| Funbox | Each family's transformations/compatibility/privacy/PB spec. |
| Tape/tags/mouse/custom keybindings/more media | Interaction/capability/persistence specs; Focus is already v1. |
| History import/portable resources | Version/security/migration/validation spec; exports exist in 011. |
| Shell completions | Generate from actual implemented options after 015. |
| Harness integration, HARNESS-01 through HARNESS-06 | Post-v1 opt-in host/protocol/interruption/privacy spec; no core agent monitoring or executable plugins. |

These are bounded deferrals from approved PRD scope, not implementation-ready promises. The PRD deliberately requires dedicated specs for them; this v1 schedule does not invent unapproved host protocols, preset schemas, or money/security-sensitive contracts.

## Handoff and verification

Read the selected phase in full, inspect its file table and predecessor interfaces, then implement only that phase. Each document includes scope, concrete schemas/interfaces, failure handling, tests, rollout/rollback and numbered acceptance criteria with matching completion checklist. Complete predecessor interfaces are named; current-state claims have inspected path:line evidence. New files have no invented current line numbers.

Planning validation: local `make verify` and `make smoke` passed with `GOCACHE=/private/tmp/terminal-typer-go-cache`; these establish a working baseline, not the PRD's unimplemented capabilities. Documentation validation checks all 17 plan links, local source citations, requirement-ID coverage, required sections and acceptance/checklist equality. Source code, source assets and public docs are unchanged by this planning task. Native reference benchmarks and content reviews are required implementation evidence and are not yet performed.
