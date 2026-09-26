# Terminal Typer product requirements

- Document type: final product requirements document (PRD).
- Lifecycle: approved product scope, awaiting implementation planning. This document does not claim that its target features are implemented.
- Approval: the product owner confirmed the complete interview synthesis after decisions Q1-Q107.
- Date: 2026-09-26.
- Product: Terminal Typer. Command and compact wordmark: `tt`.
- Milestone: v1 product experience. This is a planning label, not a promise of semantic version `1.0.0`.
- Audience: product owner, designers, contributors, and implementation agents.

## 1. Product promise and user

Terminal Typer helps terminal-heavy developers improve their typing through short, focused tests, understandable performance analysis, entertaining content, and targeted follow-up practice. Users can practice while a coding agent develops a feature, while waiting for another task, or as a standalone training habit.

Marketing line: "Improve your typing while your agent builds."

Product descriptor: "A local-first typing trainer for developers."

The core application runs independently of coding agents. Future harness integrations may offer a compact typing test during agent work. They are outside v1.

The primary job is to launch quickly, complete a useful test, understand weaknesses, and begin relevant practice without leaving the terminal or using a mouse. The interface should fit sessions lasting about 30 seconds to 5 minutes while supporting longer deliberate tests.

### 1.1 Product principles

| ID   | Principle                     | Consequence                                                                                         |
| ---- | ----------------------------- | --------------------------------------------------------------------------------------------------- |
| P-01 | Reliable daily usefulness     | A complete test-to-practice loop takes priority over a broad feature catalog.                       |
| P-02 | Immediate entry               | Launch into a ready-to-type test with saved preferences or sensible defaults.                       |
| P-03 | Calm focus                    | The prompt dominates the active screen. Metrics and controls remain quiet.                          |
| P-04 | Explainable improvement       | Results explain errors and practice selection rather than presenting scores alone.                  |
| P-05 | Local ownership               | Typing content, settings, mistakes, progress, and history stay on the user's machine.               |
| P-06 | Compatibility and data safety | Existing commands and resources remain usable, with the approved changes documented explicitly.     |
| P-07 | Keyboard completeness         | Every v1 workflow is usable through discoverable keyboard controls.                                 |
| P-08 | Restrained motivation         | Personal bests and practice progress motivate users without currencies, levels, or streak pressure. |
| P-09 | Maintainable delivery         | Reuse the existing application and ship small, working increments after product planning.           |

When requirements compete, prioritize reliable daily usefulness, compatibility and data safety, clear maintainable implementation, contributor friendliness, then feature breadth and adoption metrics.

### 1.2 Success criteria

Success means a developer can launch a test, configure it, finish it, understand the result, inspect local progress, and practice an identified weakness entirely from the keyboard. The application should remain useful without network access and should not delay or lose typing input.

Assess success through scenario testing, opt-in user feedback, and the performance requirements in Section 12. User counts and contribution activity are secondary signals. Do not add telemetry to measure these goals.

## 2. Authority and verified starting point

This PRD owns the approved target product scope. [Plans 004-008](README.md) remain historical roadmap outlines and sources of candidate ideas. Where their v1 scope differs from this PRD, this PRD wins. Completed records retain their original meaning. [Plan 003](003-tui-redesign.md) remains a historical visual reference and does not require a framework migration.

`README.md` and `man.md` continue to describe supported executable behavior. The PRD changes no runtime behavior by itself. Update public documentation when implementation lands, not when a requirement is written.

The following baseline was inspected in the checkout when this PRD was written:

| Existing capability                                                | Evidence                                                                        | Product implication                                                                                            |
| ------------------------------------------------------------------ | ------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------- |
| Go 1.17, `tcell`, optional `beep` audio                            | `go.mod:1-10`                                                                   | Use the current stack for implementation planning. A migration needs a separate rationale and scoped decision. |
| Word, quote, stdin, and file source selection                      | `src/test.go:63-89`, `src/tt.go:530-554`                                        | Preserve legacy source precedence and input contracts.                                                         |
| Unified configuration and generated-test metadata                  | `src/test.go:35-48`, `src/test.go:92-149`, [plan 010](010-typing-test-model.md) | Reuse this first foundation slice. It does not implement new modes, event history, or eligibility policy.      |
| Six persistent live settings and explicit CLI overrides            | `src/settings.go:24-45`, `src/tt.go:485-510`                                    | Retain the controls and their active-versus-saved semantics.                                                   |
| Settings pauses timing                                             | `src/typer.go:418-433`                                                          | Extend pause accounting to the new application-owned interfaces.                                               |
| Basic WPM, CPM, accuracy, timestamp, and mistakes                  | `src/tt.go:27-33`, `src/tt.go:632-643`                                          | Rich results and persistent history are new work.                                                              |
| JSON array and CSV records at process exit                         | `src/tt.go:63-91`                                                               | Preserve machine-readable output and its field/column contracts.                                               |
| Local file progress, mistakes, and settings paths                  | `src/db.go:14-33`                                                               | Preserve existing stores and plan migrations explicitly.                                                       |
| Explicit-path, local-directory, then embedded resource lookup      | `src/util.go:201-221`                                                           | Retain resource names, formats, and lookup order.                                                              |
| Resize currently exits the active run and restarts it              | `src/typer.go:407-409`, `src/tt.go:649-653`                                     | Stable prompt copies exist, but full state-preserving resize still needs implementation.                       |
| Build, verification, smoke, generated assets, and release commands | `Makefile`                                                                      | Reuse canonical commands and edit source assets rather than generated `src/packed.go`.                         |

Existing source assets include English 200/1000 word lists, other language lists, one English quote collection, a large theme collection, and a key sound. Their existence does not waive the content review requirements for official v1 bundles.

## 3. V1 scope and later product direction

### 3.1 Required v1 experience

| Area           | Required scope                                                                                                                                |
| -------------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| Modes          | Timed words, fixed word count, quotes/dialogue, and basic targeted practice.                                                                  |
| Content        | All valid existing bundled word packs, English as primary/default, and at least 100 reviewed dialogue passages.                               |
| Word modifiers | Independent punctuation, numbers, and capitalization controls.                                                                                |
| Interaction    | Immediate start, saved configuration, command palette, full configuration, quick pickers, live settings, help, Focus mode, and confirmations. |
| Presentation   | Native `tt-dark` and `tt-light`, searchable existing themes, optional sound, responsive layouts, and accessible fallbacks.                    |
| Results        | Effective/raw WPM, CPM, accuracy, consistency, errors, character outcomes, timing, weaknesses, graphs, and eligibility.                       |
| Progress       | History, trends, personal records, filters, separate practice history, export, deletion, and visible storage information.                     |
| Compatibility  | Existing flags, custom word/quote resources, files, stdin, raw/multi behavior, output formats, and lookup paths.                              |
| Persistence    | Safe configuration/history writes, existing-data migration, indefinite retention, and recoverable corruption handling.                        |
| Updates        | Consent-based asynchronous stable-release checks and notifications.                                                                           |
| Platforms      | Linux, macOS, and modern Windows Terminal, with verification before advertising support.                                                      |

### 3.2 Post-v1 capabilities

| Capability          | Intended direction                                                                                                                                                                          |
| ------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Content expansion   | Curated English 5k/10k, Roman-script Hinglish, and realistic Go, Python, JavaScript, TypeScript, Rust, Bash, SQL, JSON, and YAML snippet packs.                                             |
| Code and custom TUI | Dedicated configuration and browsing workflows. Existing file/stdin/custom-resource CLI behavior remains available in v1.                                                                   |
| Advanced training   | Longer-term weak-word, symbol, character-pair, and transition analysis with transparent practice recommendations.                                                                           |
| Difficulty          | Expert/Master, stop-on-error, no-correction/blind controls, and minimum performance thresholds with visible rules and explicit result statuses.                                             |
| Layouts             | QWERTY, Colemak, Colemak-DH, Dvorak, Workman, custom logical layouts, keyboard visualization, heatmaps, and layout-specific analytics.                                                      |
| Presets             | Built-in/user configurations, create/rename/replace/delete, `--preset`, preview, import/export, and explicit precedence.                                                                    |
| Funbox              | Bounded families such as memory, random case, reverse, no-space, symbols, ROT13, disappearing text, weakspot, speed ramp, and sudden death. Each declares compatibility and PB eligibility. |
| UI expansion        | Tape mode, custom tags, mouse support, and additional sound/theme capabilities.                                                                                                             |
| Portability         | History import and versioned portable configuration/resources with migration and validation rules.                                                                                          |
| Shell support       | Bash, zsh, and fish completions based on actual executable options.                                                                                                                         |
| Agent harnesses     | Opt-in local extensions that offer a compact test during agent thinking or waiting. See Section 13.                                                                                         |

These are future product directions, not promised v1 deliverables or implementation specs. Versioned formats for new local language, quote, theme, layout, sound, and declarative modifier resources need dedicated specs. Resource formats must not execute untrusted code.

### 3.3 Product non-goals

- Accounts, cloud sync, public leaderboards, multiplayer races, and hosted typing APIs.
- Telemetry, usage tracking, automatic crash reporting, and transmission of typing content.
- In-application update downloads, installation, executable replacement, or automatic restart.
- Pomodoro timers, break management, or agent-task monitoring in the core v1 application.
- Terminal font-family, font-size, image-background, or hardware-key remapping controls.
- Executable plugins loaded by the typing core.
- V1 custom keybindings, tags, presets, history import, beta/nightly update channels, and mouse support.

## 4. Launch, configuration, and compatibility

### 4.1 Launch and remembered state

| ID     | Requirement                                                                                                                                                                                                                         |
| ------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| CFG-01 | A new installation launches directly into a ready English 1k, 30-second test using `tt-dark`, Normal rules, sounds off, word modifiers off, and a focused default layout. The timer has not started.                                |
| CFG-02 | A returning user receives a fresh test using the last configuration they started through the TUI and their saved presentation preferences. Never automatically restore partially typed text.                                        |
| CFG-03 | Configuration includes mode, duration/count, content pack, supported modifiers, and presentation/typing preferences. Future difficulty and layout values must not be silently inferred from absent v1 features.                     |
| CFG-04 | Save test-defining changes when the user starts a test with them. Browsing and previewing leave remembered configuration unchanged.                                                                                                 |
| CFG-05 | Save runtime preferences when Settings closes successfully. A failed save leaves the interface open with an actionable error and preserves the previous active/saved state.                                                         |
| CFG-06 | Resolve values in this order: built-in defaults, saved configuration, selected preset when implemented, explicit CLI flags/input. Explicit CLI values affect only the current invocation unless deliberately saved through the TUI. |
| CFG-07 | Retain existing live-setting overrides: show both the active CLI value and saved future value. Changing an overridden saved control does not change its active CLI value.                                                           |
| CFG-08 | A first-run hint identifies configuration, Settings, and Help without introducing an onboarding wizard. Dismiss or mute the relevant hint after the interface has been discovered.                                                  |
| CFG-09 | Changing mode, pack, length, or modifiers after input requires a new-test confirmation. Appearance changes may apply immediately without restarting. Cancel returns to the same test.                                               |
| CFG-10 | Offer reset-current-section, reset-test-configuration, reset-appearance, and reset-all-settings. Confirm reset-all. Settings resets never delete history.                                                                           |
| CFG-11 | Applying settings or starting a session does not save incidental CLI paths or piped content into remembered startup configuration. Existing file-progress behavior remains separate.                                                |

Full configuration groups controls under Test, Content, Typing, Display, Sound, Data, and Help. The v1 Test group offers Normal rules only. Post-v1 features appear only when implemented; unavailable actions are not advertised as working controls.

The existing live controls remain available with these initial defaults. Migrated or saved values take precedence over initial defaults under CFG-06.

| Live control       | Choices                                         | Initial default |
| ------------------ | ----------------------------------------------- | --------------- |
| Show WPM           | On / Off                                        | Off             |
| Skip word on Space | On / Off                                        | On              |
| Allow Backspace    | On / Off                                        | On              |
| Cursor style       | Bar / Block                                     | Bar             |
| Typed text weight  | Normal / Bold                                   | Normal          |
| Word highlighting  | Current + next / Current only / Next only / Off | Current + next  |

New live-error and Focus controls follow the same saved-preference rules. Keep time/progress visible outside Focus according to mode. Settings retains Up/Down selection, Space/Enter change, and Escape/Ctrl-P save-and-close behavior.

### 4.2 Approved changes and preserved CLI contracts

The owner approved these future behavior changes:

- First-run bare `tt` changes from 50 generated words to English 1k for 30 seconds. Saved TUI configuration controls subsequent bare launches.
- A meaningful in-progress test uses a short double-Escape safeguard before restarting.
- Resize retains session state instead of restarting the test.
- Rich metrics use a documented common model. Any changed meaning of an existing exported metric must be documented and tested as part of the new release.
- Network access is permitted for explicit or consent-enabled release checks only.

These changes take effect only when implemented and documented for the target release. They do not authorize an immediate release or a framework change.

| ID     | Compatibility requirement                                                                                                                                                                                                                                              |
| ------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| CLI-01 | Keep every registered flag's syntax and documented role. Keep `-n` as words per group, `-g` as groups, `-t` as timeout, and `-start` as file-progress selection. The TUI's 500-word custom limit does not retroactively restrict supported legacy invocations.         |
| CLI-02 | Preserve source precedence: explicit `-words`, explicit `-quotes`, non-TTY stdin, positional file, then bundled `1000en` when no saved/new mode selection applies. Explicit legacy inputs remain authoritative over remembered TUI selections.                         |
| CLI-03 | Preserve `-raw` over `-multi`, paragraph handling, resource input through `-`, source exhaustion, and saved file progress.                                                                                                                                             |
| CLI-04 | Preserve source selection, report suppression, one-shot operation, listing, version/help, theme flags, sound flags, and existing keyboard navigation where they are not covered by an approved change.                                                                 |
| CLI-05 | Keep JSON fields and existing CSV record/column order usable. Introduce richer output additively or through an explicitly versioned richer format. Keep decoration, update notices, consent prompts, and colors out of machine-readable stdout.                        |
| CLI-06 | Preserve explicit-path, user-directory, system-directory, then embedded-resource lookup and existing resource identifiers. Show source origin in selectors where useful.                                                                                               |
| CLI-07 | Use a compatibility matrix with concrete invocation fixtures before implementing remembered mode selection. It must demonstrate legacy word-count requests without inheriting an unintended timer and quote/file/stdin requests without inheriting unwanted modifiers. |

Structured output is a supported scripting surface. Existing JSON/CSV flags do not by themselves imply that the current typing input is headless. Any new unattended/headless mode needs its own spec.

## 5. Modes, content, and practice

### 5.1 Test modes

| ID      | Mode                  | Configuration                                                              | Completion rule                                                                                                                                                    |
| ------- | --------------------- | -------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| MODE-01 | Timed words           | 15, 30, 60, or 120 seconds; custom integer 5-3600 seconds. Default 30.     | Supply enough content to last the selected active duration. Stop at expiry, including mid-word. Exhausting an initial word buffer must not end a timed test early. |
| MODE-02 | Word count            | 10, 25, 50, or 100 words; custom integer 1-500 words.                      | Generate exactly the selected count and finish at the end of that prompt. Hide legacy group mechanics in the TUI.                                                  |
| MODE-03 | Quote/dialogue        | Select pack and passage metadata filters supported by its catalog.         | Complete the authored passage. The default word-test timer does not truncate it. Measure active elapsed time.                                                      |
| MODE-04 | Targeted practice     | Short completion-based set of approximately 25 words.                      | Finish the drill and show item-specific improvement. Exclude ordinary PB eligibility.                                                                              |
| MODE-05 | Legacy custom sources | Existing files, stdin, custom word lists, and quote resources through CLI. | Preserve their documented generation, exhaustion, progress, and raw/multi rules.                                                                                   |

Reject invalid lengths with an inline explanation before starting. Punctuation, numbers, and capitalization are independent toggles for generated word tests. Preserve quote/dialogue authored text and user-supplied file/stdin text. Define exact modifier-generation rules in a scoped spec with examples for each combination and language support.

### 5.2 Content catalog

| ID         | Requirement                                                                                                                                                                                                                                                                         |
| ---------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| CONTENT-01 | Expose all valid existing bundled word packs with understandable display names and language/variant labels. English 1k maps to the existing `1000en` resource. Keep existing resource identifiers usable.                                                                           |
| CONTENT-02 | V1 includes at least 100 individually reviewed dialogue passages across multiple genres and lengths. This is a release gate, not a count of unreviewed imported text.                                                                                                               |
| CONTENT-03 | Prefer source/genre browsing, with passage length and difficulty as metadata. Include collections such as comedy, drama, science fiction, fantasy, workplace scenes, and public-domain literature as the available content supports them.                                           |
| CONTENT-04 | Official content uses public-domain, permissively licensed, or original material with clear redistribution rights. Famous modern movie/TV dialogue is included only when rights permit it. Local user packs may supply other material.                                              |
| CONTENT-05 | Each official pack has an identifier, display name, language, type, license, source, attribution, and maintainer notes. Each passage has an identity, title/source, speaker when applicable, provenance, license, length, difficulty, and tags.                                     |
| CONTENT-06 | Official packs remain broadly workplace-safe. Mild authentic language is permitted. Exclude slurs, explicit sexual content, graphic violence, harassment, and shock-driven material.                                                                                                |
| CONTENT-07 | Prevent adjacent duplicate generated words. Quotes/dialogue use a shuffled queue that avoids repetition until the pack is exhausted. Preserve enough queue/recent-use state to avoid immediately repeating recent passages after restart. Explicit Retry bypasses this restriction. |
| CONTENT-08 | Show a muted collection/category during typing. Show full source, title, speaker, attribution, and provenance on Results and Analysis. Include required legal attribution in distributed manifests/resources where applicable.                                                      |
| CONTENT-09 | Validate content before use and report the affected resource and remedy for errors. New manifests must not make existing valid local word/quote formats unusable.                                                                                                                   |

Dialogue is fun through authored content and variety. V1 does not add game levels, currencies, public competition, or forced engagement loops.

### 5.3 Basic targeted practice

| ID          | Requirement                                                                                                                                                                             |
| ----------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| PRACTICE-01 | Results offers `Practice weaknesses` when enough suitable error/word-timing data exists. Training never starts automatically.                                                           |
| PRACTICE-02 | Choose errors and slow words from the just-completed test plus a bounded recent-history window. Explain selection and the available sample, including when history contributes nothing. |
| PRACTICE-03 | Repeat weak items enough to reinforce them and mix neutral words to reduce rote memorization. Preserve required punctuation/context for a quoted weak item.                             |
| PRACTICE-04 | Review shows selected items and reasons before starting. Allow repeat or dismiss, and explain insufficient-data states without inventing weaknesses.                                    |
| PRACTICE-05 | Completion compares practiced items with their recent baseline using accuracy, errors, and relative speed. Show sample limits. Offer `Practice again` and `Return to regular test`.     |
| PRACTICE-06 | Store practice separately from ordinary tests. Exclude practice from ordinary trends and PBs by default. Do not present its WPM as comparable to a normal random test.                  |
| PRACTICE-07 | Read only permitted stored text. Without explicit content-retention consent, private file/stdin input contributes no word fragments to practice.                                        |

The practice spec must pin the recent-history window, minimum evidence, selection ranking, neutral-word ratio, timing baseline, and dismissal behavior before implementation. These are bounded algorithm contracts beneath the approved product behavior.

## 6. Session lifecycle and keyboard interaction

### 6.1 Timing, correction, and restart

| ID         | Requirement                                                                                                                                                                                                                                    |
| ---------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| SESSION-01 | The first accepted typing character starts active time. Setup, navigation, and control keys before that character do not start timing.                                                                                                         |
| SESSION-02 | Settings, command palette, Help, configuration, and application-owned attention/confirmation dialogs pause an active test. Closing without replacement resumes the same input and prompt.                                                      |
| SESSION-03 | Idle time after typing begins counts. Do not infer pauses from inactivity or terminal focus loss.                                                                                                                                              |
| SESSION-04 | Timed expiry ends the test at its selected active duration. The timer display, live metrics, result calculations, and graphs share the same active-time accounting.                                                                            |
| SESSION-05 | Normal rules show errors immediately and permit Backspace and word deletion. Space may advance an incorrect word unless the skip control is disabled. Track corrected and remaining errors separately.                                         |
| SESSION-06 | Control keys and overlays never become typing content. The live error count must identify its definition consistently with result error totals.                                                                                                |
| SESSION-07 | Escape restarts immediately before meaningful input. Once input/time crosses the defined safeguard threshold, show `Esc again to restart` and require a second Escape within a short interval. A missed interval returns to the same test.     |
| SESSION-08 | Restart/Retry repeats the exact generated prompt in memory. Next generates a fresh prompt using the same configuration. A restarted attempt cannot create a duplicate completed result.                                                        |
| SESSION-09 | Preserve generated text, typed characters, correction state, timer, navigation position, and pause accounting through supported resizes. Returning from a modal preserves its prior context.                                                   |
| SESSION-10 | Completion transitions to Results in about 150-250 ms when terminal refresh allows it, or immediately with reduced motion. The transition never blocks input processing. Prevent a buffered completion key from accidentally skipping Results. |
| SESSION-11 | V1 exposes only Normal difficulty. If a later version restores strict settings, show the rules before input begins and distinguish failed, aborted, and completed sessions.                                                                    |

Pin safeguard thresholds, pause-state transitions, input classification, and the too-small-terminal timing policy in the session spec. Resize below the usable minimum must preserve recoverable session state and make the timing policy visible and deterministic.

### 6.2 Keyboard defaults

| Key                                | Typing                                          | Other contexts                                                                                          |
| ---------------------------------- | ----------------------------------------------- | ------------------------------------------------------------------------------------------------------- |
| `Ctrl-K`                           | Open command palette and pause.                 | Open command palette.                                                                                   |
| `Ctrl-P`                           | Open existing live-settings overlay and pause.  | Expose saved runtime preferences where relevant without redefining active-test behavior.                |
| `Escape`                           | Restart with the agreed safeguard.              | Close overlay, cancel, or return to the previous screen. Existing Settings close/save behavior remains. |
| `Ctrl-C`                           | Exit.                                           | Exit.                                                                                                   |
| `Ctrl-L`                           | Redraw/synchronize terminal.                    | Redraw/synchronize terminal.                                                                            |
| `?`                                | Type `?` when entering prompt text.             | Open contextual Help outside text-entry/search fields.                                                  |
| Arrows                             | Preserve compatible typing/test navigation.     | Move selection.                                                                                         |
| `j` / `k`                          | Type those characters.                          | Optional list navigation when a text/search field does not have focus.                                  |
| `Enter`                            | Follow content/input rules.                     | Activate selection.                                                                                     |
| `/`                                | Type `/`.                                       | Focus search in a searchable list when not editing a field.                                             |
| Backspace / `Ctrl-W`               | Correct a character/delete a word when enabled. | Edit a focused text field where applicable.                                                             |
| `Ctrl-Backspace` / `Alt-Backspace` | Delete a word where the terminal reports it.    | Follow text-field editing behavior.                                                                     |

Help is always available through the command palette, including during a test where `?` must remain literal content. Search fields consume text as text; letter-navigation shortcuts must not intercept a query. Provide an action label and a discoverable control on every screen.

## 7. Screens and journeys

### 7.1 Screen map

```text
Launch -> Typing -> Results -> Next / Retry -> Typing
                     |
                     +-> Practice review -> Practice -> Practice result
                     +-> Detailed analysis
                     +-> Progress [History | Trends | Records]
Ctrl-K -> Command palette -> Quick picker / Configure / Progress / Help
Ctrl-P -> Live settings -> Same paused typing session
```

Navigation back from read-only screens does not create, record, or abandon a test. Starting a replacement test from any route follows the same configuration and confirmation rules.

### 7.2 Screen responsibilities

| ID    | Screen or overlay      | Required content and behavior                                                                                                                                                                                                                                                   |
| ----- | ---------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| UI-01 | Typing                 | Small `tt` label, mode/length/pack context, quiet centered prompt, caret, time or word progress, optional WPM and error count, minimal keyboard hints. No decorative prompt box.                                                                                                |
| UI-02 | Results                | Effective WPM dominates. Accuracy and consistency are secondary. Include raw WPM, errors, timeline, eligible PB comparison, weaknesses, and test details. Default action is Next test, alongside Retry, Practice weaknesses, and Analysis.                                      |
| UI-03 | Detailed analysis      | WPM/raw-WPM and error timelines, correct/incorrect/extra/missed counts, corrected/uncorrected errors, slowest/fastest words, weak words/pairs, configuration, eligibility, active/pause durations, and full attribution. Explain unavailable fields for private or sparse data. |
| UI-04 | Progress: History      | Chronological regular/practice views, period summary, PB indicators, selection details, open Analysis, confirmed individual deletion, and filters by time, mode, length, pack, difficulty, and eligibility.                                                                     |
| UI-05 | Progress: Trends       | WPM, accuracy, consistency, test volume, recent PBs, and frequent weak words/pairs. State time range and sample size. Default comparisons use comparable regular sessions.                                                                                                      |
| UI-06 | Progress: Records      | PBs grouped by eligible configuration, record details, and the improvement amount. Updating/deleting history keeps this view consistent.                                                                                                                                        |
| UI-07 | Practice review/result | Explain selection, start/cancel, repeat/dismiss, item-level comparison, and return to regular testing.                                                                                                                                                                          |
| UI-08 | Full configuration     | Test, Content, Typing, Display, Sound, Data, and Help groups. Start applies and remembers test configuration. Cancel preserves it.                                                                                                                                              |
| UI-09 | Command palette        | Search labels/aliases and show reachable implemented actions. Route simple changes to quick pickers and complete setup to Configure test.                                                                                                                                       |
| UI-10 | Quick pickers          | Select mode, length, pack, modifiers, or theme while retaining context. Respect preview, apply, and confirmation rules.                                                                                                                                                         |
| UI-11 | Live settings          | Retain the six existing controls, add implemented live presentation controls through explicit schema migration, and display CLI override versus saved preference. Never merge this into full test configuration.                                                                |
| UI-12 | Help                   | Contextual controls, full keyboard map, CLI equivalents, input/privacy limits, and escape/return route.                                                                                                                                                                         |
| UI-13 | Confirmation/error     | Say what will change or what failed, whether data changed, and the next available action. Keep typing state recoverable when cancelling.                                                                                                                                        |

Live metrics in v1 are exactly time remaining or word progress, WPM, and error count. Default to time/progress only. WPM and error count can be enabled in a compact arrangement. Raw WPM, accuracy, and other diagnostics appear after completion.

The palette exposes Change mode, Change duration/count, Select content, supported modifiers, Settings, Switch theme, Results/Analysis when available, History, Statistics, Records, Practice when available, Restart, Next test, Help, Export history, manual update check, and Quit. `History`, `Statistics`, and `Records` open the corresponding Progress tab. Search accepts useful aliases such as `stats` and `colors`.

### 7.3 Required user journeys

1. First use: launch, see the ready English 1k 30-second test and quiet hints, type, finish, view Results, then answer the update-consent prompt without losing the result.
1. Repeated testing: choose Next for new text with the same configuration or Retry for identical text, complete, and return to Results without setup.
1. Configuration: open palette, select a quick change or Configure test, preview/browse, apply, confirm abandonment if needed, and start a new test. Later launch remembers intentional choices.
1. Live preference: open Settings mid-test, edit, save, and resume with active time unchanged by the overlay. An explicit CLI override stays effective.
1. Improvement: inspect weaknesses, review why items were selected, complete practice, compare item performance, and return to a regular test.
1. Progress: browse/filter history, open a result, inspect comparable trends or records, and return through keyboard navigation.
1. Data ownership: inspect data path/usage, export JSON or CSV to a selected local destination, or confirm deletion. Failed writes/deletions preserve existing data and show a remedy.
1. Offline/error: complete tests with the network unavailable or history unreadable. Show unavailable-data states without replacing damaged files.
1. Legacy input: launch a documented word/quote/file/stdin invocation and complete it with expected source, timeout, output, and progress behavior.

## 8. Visual language and accessibility

Design input: the owner's [PlanetScale Design Analysis conversation](chatgpt-conversation://6ab685d1-6cb4-83ee-8fc7-7b4b99c59d68) and Monkeytype's calm typing focus. Treat these as inspiration, not a claim of affiliation or an exact website reproduction.

| ID        | Requirement                                                                                                                                                                                                                                                                               |
| --------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| DESIGN-01 | Native themes are `tt-dark` and `tt-light`. Dark is the default. No shipped theme name, product copy, or marketing implies PlanetScale endorsement.                                                                                                                                       |
| DESIGN-02 | Build native themes around near-black, warm near-white, a neutral gray ladder, and a restrained orange accent. Reference seed colors from the design research are `#1a1a1a`, `#fafafa`, and `#f35815`. Final semantic roles need contrast-tested theme tokens.                            |
| DESIGN-03 | Use spacing, alignment, grid relationships, and quiet rules for hierarchy. Use shared panel boundaries in dense screens. Keep the typing prompt spacious and largely borderless.                                                                                                          |
| DESIGN-04 | Orange indicates caret, current selection, progress, and rare emphasis such as PBs. Borders and ordinary headings remain neutral.                                                                                                                                                         |
| DESIGN-05 | Pending text is muted, correct text uses primary foreground, incorrect text uses red plus underline/inverse indication, and extra text has a distinct marker or background. Missed text uses warning treatment in analysis. Focus/selection has a shape/contrast change as well as color. |
| DESIGN-06 | Keep the compact brand `tt`. Avoid large ASCII logos, emoji-dependent controls, gradients, shadows, ornamental icons, and floating-card layouts.                                                                                                                                          |
| DESIGN-07 | Use concise technical and quietly encouraging copy. Prefer `New personal best: 4 WPM faster` to exaggerated praise.                                                                                                                                                                       |
| DESIGN-08 | Present a curated theme group first, including native dark/light, terminal defaults, Gruvbox, Dracula, Nord, and suitable accessible choices. Search the complete existing collection. Preview before saving.                                                                             |
| DESIGN-09 | Provide Focus mode that hides optional chrome while preserving prompt, caret, essential time/progress, and discoverable exit/help controls.                                                                                                                                               |
| DESIGN-10 | Sounds are off initially. Key, error, completion, and PB feedback are optional and individually understandable. Unavailable audio cannot prevent a test.                                                                                                                                  |
| DESIGN-11 | Completion feedback is brief and restrained. Reduced-motion/off switches immediately; sound and animation never block typing or navigation.                                                                                                                                               |

The product cannot select a terminal font or reproduce a sans/mono website typography pairing. Document font, size, ligature, background-image, and screen-reader behavior as emulator-controlled capabilities. Use terminal-compatible spacing and restrained weight for hierarchy.

### 8.1 Layout and capability tiers

| Terminal dimensions/capability                                                    | Required behavior                                                                       |
| --------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------- |
| Width at least 80 and height at least 24                                          | Full layout.                                                                            |
| Width at least 52 and height at least 14, with either full-layout threshold unmet | Compact layout.                                                                         |
| Width below 52 or height below 14                                                 | Actionable minimum-size message, exit/redraw controls, and preserved recoverable state. |
| Truecolor                                                                         | Intended native theme colors.                                                           |
| 256/basic colors or disabled color                                                | Readable mapped/fallback colors and non-color state indicators.                         |
| Broken/unavailable box drawing                                                    | ASCII structural fallback.                                                              |

As space decreases, remove optional live metrics, secondary footer hints, extra spacing/rules, then secondary metadata. Replace charts with text summaries. Preserve the prompt, caret, essential timer/progress, active selection, primary result, and exit/help route at every usable size.

Mandatory accessibility requirements include Unicode cell-width-aware layout, long-text handling, high-contrast native themes, color-independent information, optional sound, reduced motion, text controls, and timing pauses during Help/application accessibility overlays. Mouse and configurable keybindings are deferred.

## 9. Measurement, results, and personal bests

### 9.1 Common metric model

| ID        | Requirement                                                                                                                                                                                                                                                                       |
| --------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| METRIC-01 | Effective WPM represents correct-character speed. Raw WPM represents typed-character activity. Accuracy represents correct input relative to attempted input. Use familiar five-character word units and disclose the exact formula/version.                                      |
| METRIC-02 | Preserve CPM and distinguish correct, incorrect, extra, and missed character outcomes. Track input correction history separately from final retained outcomes.                                                                                                                    |
| METRIC-03 | Report consistency, active duration, pause duration, corrected/uncorrected errors, per-word speed/timing, and WPM/raw-WPM/error series. Deep per-key heatmaps and physical-key inference are deferred.                                                                            |
| METRIC-04 | Derive live, final, graph, history, export, and record values from one metric contract. A replay of the same recorded session yields identical measurements.                                                                                                                      |
| METRIC-05 | Define zero/near-zero duration, skipped words, corrected input, unfinished words, timeouts, extra/missed characters, aborted tests, rounding, and Unicode units before implementation. Display unavailable measurements explicitly rather than NaN, infinity, or invented values. |
| METRIC-06 | Show sample size and time range for trends and practice baselines. Do not imply reliable progress from sparse or incomparable samples.                                                                                                                                            |

The current in-memory result can show word-level diagnostics while the session text is available. Historical analysis shows only retained permitted data or resolvable catalog metadata. If a fastest-word label or exact-prompt view cannot be recovered without retaining prohibited text, show that detail as unavailable rather than enlarging the default content-retention policy.

"Monkeytype-familiar" describes user expectations, not verified formula parity with an external service. The dedicated measurement spec must finalize consistency, counting units, timing windows, error-series definitions, burst behavior, and examples. No coding agent may infer these independently for each screen.

### 9.2 Result identity and eligibility

Rich results include session identity, metric-definition version, mode, pack, configured duration/count, modifiers, Normal difficulty, timestamp, active/pause duration, outcome, retry/practice status, character/error totals, timing samples, weakness summary where permitted, and eligibility with reasons. Include source attribution for quote/dialogue results. Future layout/tag fields follow their own versioned contracts.

| ID    | Requirement                                                                                                                                                                                     |
| ----- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| PB-01 | Group ordinary records by mode, configured length, content pack, difficulty, and result-affecting modifiers. A 15-second word test does not compete with a quote or punctuation-heavy dialogue. |
| PB-02 | A normal timed test ending at its limit is a valid result, including an unfinished final word under the documented metric rules. New ordinary tests can be eligible.                            |
| PB-03 | Exclude exact-prompt retries, practice, aborted tests, private/custom inputs, and unapproved challenge modifiers from ordinary PBs. Display the exclusion reason.                               |
| PB-04 | A new PB shows a restrained orange marker and improvement amount. Optional PB sound follows the user's preference and never blocks the result.                                                  |
| PB-05 | Records are reproducible from retained eligible history. Define ties, quote/dialogue length comparison, deleted-record effects, and pack/version changes in the eligibility spec.               |

## 10. Local history, privacy, and recovery

### 10.1 Recording and content retention

| ID      | Requirement                                                                                                                                                                                                                                                                         |
| ------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| DATA-01 | Store completed and timed-expired tests automatically. Future strict failures use explicit failure status. Do not record abandoned tests before meaningful input. Longer abandoned attempts, if retained under the session spec, are marked aborted and cannot affect ordinary PBs. |
| DATA-02 | Retain history indefinitely by default, show local storage usage, and offer explicit deletion. Do not silently expire records.                                                                                                                                                      |
| DATA-03 | Separate practice history and normal history/trends. Preserve retry status and PB exclusions.                                                                                                                                                                                       |
| DATA-04 | Never store complete prompts or complete typed responses by default. Retain pack identity and a content-free opaque/digest identity for matching. Such identity is not an anonymization guarantee, particularly for guessable public passages.                                      |
| DATA-05 | Generated/approved-pack tests may retain mistake/slow-word fragments needed for practice. Private files/stdin retain aggregate metrics and a source label, without text fragments or mistakes unless the user explicitly enables local retention.                                   |
| DATA-06 | Keep all typing data local. Local exports are explicit user actions to selected destinations and contain no incidental private paths beyond documented fields.                                                                                                                      |
| DATA-07 | Keep existing structured CLI mistake output compatible as an explicit invocation output. The private-text default applies to the new automatic rich-history store; preserve existing local mistake files during migration.                                                          |

The source-privacy spec must classify built-in packs, approved local packs, private custom lists, files, and stdin. Do not infer that a user resource is safe to retain merely because it looks like a word list. Keep stored aggregate history useful when text-derived diagnostics are unavailable.

### 10.2 Export, deletion, and safe writes

| ID      | Requirement                                                                                                                                                                                                                |
| ------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| DATA-08 | Export complete retained history as versioned JSON and a flat session summary as CSV. Export only stored/permitted fields. History export is separate from the existing per-invocation JSON/CSV format.                    |
| DATA-09 | Offer confirmed individual-result deletion, practice-history deletion, and all-history deletion. Refresh records/trends after deletion. Never combine reset settings with deletion.                                        |
| DATA-10 | Show the data-directory path and storage usage. Continue honoring the existing XDG/home path contract and document supported platform behavior.                                                                            |
| DATA-11 | Use versioned durable storage with safe write behavior and appropriate private file/directory permissions. Interrupted or failed writes do not erase prior valid data or freeze typing.                                    |
| DATA-12 | Corrupt/unreadable history remains intact. Typing continues with history unavailable and a message naming the path, whether data changed, and recovery/export guidance. Do not silently start a replacement store over it. |
| DATA-13 | Safely migrate existing six-control settings and preserve file progress and mistake stores. Start rich result history from its new schema; do not fabricate old tests from aggregate mistakes.                             |
| DATA-14 | Back up before irreversible migration. If configuration migration cannot be performed safely, preserve/back up the original, use defaults, and show the exact preserved path. Leave history untouched.                     |
| DATA-15 | Provide tested rollback instructions for persistent changes. Unknown-version data remains recoverable. Scope a later import/migration spec before adding history import.                                                   |

Configuration and history errors need separate handling: configuration may fall back to defaults after preserving its file; damaged history stays unavailable until a safe recovery action is chosen. A failed result save leaves the current result visible and explains whether it was stored.

## 11. Update checks and network exception

| ID        | Requirement                                                                                                                                                                                                                    |
| --------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| UPDATE-01 | No automatic release request occurs until the user gives informed consent. Show the one-time choice after the first completed test reaches Results, keeping the result accessible. No startup onboarding interruption.         |
| UPDATE-02 | Explain that checks send current version, OS, architecture, and unavoidable network metadata such as IP address. Offer Enable checks and Not now. Keep the decision in Settings > Data.                                        |
| UPDATE-03 | If the user quits before that result or dismisses without deciding, leave consent unset. Not now declines automatic checks until the user changes the setting.                                                                 |
| UPDATE-04 | When enabled, check stable releases at most once every 24 hours. Persist the check/attempt time and avoid repeated requests on failed startups. Run asynchronously without delaying startup/input.                             |
| UPDATE-05 | Provide a manual Check for updates command as an explicit user-requested network action, even when automatic checks are disabled.                                                                                              |
| UPDATE-06 | Send no installation identifier, usage statistics, settings, resource names, typing text, mistakes, results, or agent content. Document the chosen update endpoint and its request behavior in the update implementation spec. |
| UPDATE-07 | Automatic network failures are silent. Manual failures show a concise error. Offline operation remains complete.                                                                                                               |
| UPDATE-08 | A newer release produces a quiet notice with version and release link/install guidance. Never download, execute an installer, replace the binary, or restart.                                                                  |
| UPDATE-09 | Stable channel only in v1. No telemetry, crash reporting, content downloads, or other automatic network requests. Help/list/version and scripting paths do not trigger consent UI or release checks.                           |

The release endpoint, ownership, version comparison, request timeout, redirects, transport security, and notification dismissal rules must be pinned in its implementation spec. No update server or release infrastructure is implicitly authorized by this PRD.

## 12. Platform, performance, and release acceptance

### 12.1 Supported environments

Tier 1 targets are Linux, macOS, and modern Windows Terminal. Legacy Windows console behavior is unsupported. Verify terminal input, Unicode, resize, color fallback, audio failure, local paths, and restoration after exit on each advertised platform. Publish architecture-specific binaries only where builds and verification support the claim. Preserve source builds and the current Go/Makefile stack unless a separate approved technical decision changes them.

### 12.2 Measurable quality targets

| ID      | Target                                                                                 | Verification condition                                                                                                    |
| ------- | -------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------- |
| PERF-01 | Ready interactive screen within 250 ms.                                                | Typical supported reference machine, warm executable, healthy local configuration. Release check does not gate readiness. |
| PERF-02 | Keystroke visible within 50 ms.                                                        | Normal supported terminal conditions. Measure event-to-render behavior.                                                   |
| PERF-03 | No lost/reordered input at 200 WPM.                                                    | Deterministic sustained input, including corrections and normal redraw activity.                                          |
| PERF-04 | Resize redraw within 100 ms with state intact.                                         | Usable terminal dimensions and representative sessions.                                                                   |
| PERF-05 | Progress screen opens within 500 ms with 10,000 stored results.                        | Healthy store on a documented supported reference machine.                                                                |
| PERF-06 | Audio, graph rendering, persistence, animation, and update checks do not block typing. | Exercise slow/failing dependencies while measuring input processing.                                                      |

Before implementation acceptance, document hardware/OS/terminal, cold/warm conditions, sample sizes, and measurement methodology. Report percentile and worst-case behavior where relevant. Slow remote terminals/filesystems do not invalidate the target, but must degrade visibly without data loss. These are targets to test, not claims about the present checkout.

### 12.3 End-to-end acceptance gates

| Gate | Acceptance scenario                                                                                                                                                                                                                          | Requirements                                                                    |
| ---- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------- |
| A-01 | Fresh launch opens English 1k for 30 seconds. No timer or request runs before input/consent. Complete and reach Results.                                                                                                                     | CFG-01, CFG-08, MODE-01, SESSION-01, UPDATE-01                                  |
| A-02 | Change mode/content through the keyboard, start, exit, relaunch, and receive fresh content with remembered configuration. Cancel a preview and verify it was not remembered.                                                                 | CFG-02 through CFG-06, UI-08 through UI-10                                      |
| A-03 | Exercise all standard presets, custom bounds, and invalid values. Quotes finish by completion; timed words continue to the time limit.                                                                                                       | MODE-01 through MODE-04                                                         |
| A-04 | Open/close each pausing overlay during a test. Verify exact retained text, correction state, and active duration. Check idle time still counts.                                                                                              | SESSION-01 through SESSION-06                                                   |
| A-05 | Correct and skip known input errors, timeout mid-word, retry, and generate Next. Compare metric fixtures, eligibility, and record count.                                                                                                     | SESSION-05 through SESSION-08, METRIC-01 through METRIC-05, PB-01 through PB-03 |
| A-06 | Use all dedicated screens, palette aliases, pickers, help, confirmation, and keyboard controls without a mouse. Search literal shortcut letters/punctuation correctly.                                                                       | Section 6.2, UI-01 through UI-13                                                |
| A-07 | Complete a test with known weaknesses, review practice explanations, finish a drill, and compare relevant items. Sparse/private data shows an honest unavailable state.                                                                      | PRACTICE-01 through PRACTICE-07                                                 |
| A-08 | Inspect History, Trends, and Records with filters and samples. Delete a PB and verify records/trends recompute from surviving eligible results.                                                                                              | UI-04 through UI-06, PB-04, PB-05, DATA-09                                      |
| A-09 | Export JSON/CSV locally and verify version, summaries, and private-text exclusions. Exercise all deletion scopes and cancelled confirmations.                                                                                                | DATA-04 through DATA-10                                                         |
| A-10 | Interrupt/fail writes and supply malformed/unknown-version data. Preserve originals, keep typing usable, and provide visible recovery paths.                                                                                                 | DATA-11 through DATA-15, CFG-05                                                 |
| A-11 | Verify reviewed manifests, at least 100 dialogue passages, exact authored text, attribution, shuffled cycles, restart repetition policy, and valid local resource compatibility.                                                             | CONTENT-01 through CONTENT-09                                                   |
| A-12 | Exercise full/compact/too-small sizes, Unicode, ASCII/color fallbacks, Focus mode, reduced motion, and failed audio. Input and exit remain available.                                                                                        | SESSION-09, DESIGN-01 through DESIGN-11, Section 8.1                            |
| A-13 | Run representative old invocations, including raw/multi, file progress, custom resources, sound/theme flags, one-shot/report suppression, list/help/version, and JSON/CSV parsing. Document the approved default/metric/restart differences. | CLI-01 through CLI-07                                                           |
| A-14 | Deny/unset/enable update consent, repeat launches inside/outside 24 hours, fail the network, check manually, and verify notification-only behavior and transmitted fields.                                                                   | UPDATE-01 through UPDATE-09                                                     |
| A-15 | Verify platform and performance matrix, data migration/rollback, and repository maintenance checks before declaring the milestone complete.                                                                                                  | Section 12.1, PERF-01 through PERF-06, DATA-13 through DATA-15                  |

These gates define the release bar. Each implementation spec must also map every applicable requirement ID to its unit, integration, or user-journey checks. The gate list does not waive a requirement that is not named explicitly in a particular row.

V1 is complete only when every required gate passes, the content catalog is reviewed, migrations/rollback are tested, and current public documentation agrees with the executable. Use `make verify` and `make smoke` for code changes. Update `README.md` and `man.md` together for public behavior changes and regenerate assets/manual through `make assets` when source assets or `man.md` change. Update release history when a release occurs. Release, publishing, and deployment remain separate human-authorized actions.

## 13. Future harness integration requirements

This direction captures the owner's intended extension experience. It is deferred and does not authorize core v1 agent monitoring or an executable plugin mechanism.

| ID         | Future requirement                                                                                                                                                                                                                         |
| ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| HARNESS-01 | A harness extension notices an agent thinking/waiting state and may offer a quick typing test. Support local saved settings and a compact typing surface suitable for the host.                                                            |
| HARNESS-02 | Ask for opt-in during plugin setup. Offer Always offer, Ask once, and Never offer preferences with clear scope. Prompt only after about 15 seconds of agent work, at most once per task, and avoid interrupting active user command entry. |
| HARNESS-03 | Acceptance opens a short timed dialog/test using user preferences. Declining leaves agent work unaffected. Host capability determines native dialog versus an external terminal surface and must be specified per integration.             |
| HARNESS-04 | When the agent completes or needs input, pause and show an attention decision with Return now and Finish test. Preserve the session and never discard it automatically.                                                                    |
| HARNESS-05 | The harness receives only open/closed mini-session state by default. Share typing results only after separate explicit opt-in if a future spec introduces it. Never automatically use task/code/prompt content as typing material.         |
| HARNESS-06 | Reuse stable session/configuration/result interfaces from the core. Define the local integration protocol, interruption semantics, consent, host limitations, and process boundaries in a post-v1 spec.                                    |

## 14. Planning handoff

The product interview is complete and the scope is approved. This PRD defines product outcomes and release acceptance. It deliberately does not select a history database, invent wire schemas, finalize metric algorithms, or impose an agent integration protocol. Those choices belong in the next engineering specs, with this document's requirements as their constraints.

Before developing each capability, create a numbered spec with the required behavior, data/interface contracts, terminal state transitions, migrations/rollback, compatibility fixtures, tests, and requirement IDs covered. Each implementation slice must leave a working demonstrable application. Reinspect the current checkout when writing those specs rather than treating this baseline as permanently current.

The first planning pass must resolve these engineering contracts:

| Contract                  | Required output before implementation                                                                                                                  |
| ------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Configuration/CLI mapping | Explicit legacy invocation matrix, new-mode resolution, saved settings migration, override behavior, and approved-change release notes.                |
| Session/input             | Event and correction semantics, active-time rules, safeguard thresholds, resize/minimum-size transitions, and repeat/new/abort identities.             |
| Metrics/eligibility       | Formulas and example fixtures, consistency/windows/rounding/Unicode rules, status classification, ties, quote length grouping, and retry/PB policy.    |
| History/privacy           | Versioned schema/store, permissions, retention fields, source classification, safe writes, export/deletion, corruption recovery, and rollback.         |
| Content/practice          | Manifest compatibility, licensed catalog, passage queue, modifier rules, practice window/ranking/sample limits, and review workflow.                   |
| TUI/design                | Concrete focus/keymap, screen states, theme roles/contrast, fallback behavior, and representative terminal renders.                                    |
| Update checks             | Project-owned release endpoint, transmitted request fields, secure/versioned check behavior, consent persistence, timeouts/caching, and failure tests. |
| Performance/platforms     | Reference environment matrix and repeatable benchmark/acceptance procedures.                                                                           |

Do not treat plans 004-008 as the implementation schedule unchanged. This PRD moves some former later-phase capabilities into v1 and defers other candidates. The next project plan must reflect this approved scope and include the full typing-to-results-to-practice path early.

### 14.1 Interview decision trace

| Interview decisions | Captured requirement areas                                                                                                                                                                  |
| ------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Q1-Q8               | User, promise, scope split, visual personality, motivation, local ownership, and success criteria.                                                                                          |
| Q9-Q18              | Harness positioning, saved fresh-test configuration, v1 modes, palette, content rights, practice, result priority, dark/light themes, live density, and Normal rules.                       |
| Q19-Q26             | CLI preservation, completion actions, dialogue organization, recent practice, history/statuses, PB categories, harness direction, and short-session focus.                                  |
| Q27-Q38             | English 1k 30-second default, hints, screens, keys, restart safeguard, dimensions, visual structure, sound/motion, and future harness consent/attention/privacy.                            |
| Q39-Q50             | First-character timing, durations, 1-500-word bound, passage completion, modifiers, correction/pause, practice result, retries, and attribution.                                            |
| Q51-Q59             | Common metrics, indefinite history, filters/trends, deferred tags, text privacy, PB feedback, export/deletion, and corruption recovery.                                                     |
| Q60-Q69             | Existing language catalog, deferred new languages, 100 reviewed dialogue passages, content tone/repetition, native-theme naming, color fallback, state colors, identity, and microcopy.     |
| Q70-Q80             | `tt-dark`/`tt-light`, configuration groups/save/precedence, new-test confirmations, resets, palette scope, keyboard-only access, accessibility, platforms, and migration failure.           |
| Q81-Q91             | Screen compositions, exactly three live metric types, Focus now/Tape later, Analysis, Progress tabs, configuration entry points, borders, errors, transition, and small-screen degradation. |
| Q92-Q101            | Scripting, update-only networking, external installation, performance, data/resource preservation, provenance, approved v1 boundary, milestone naming, and priority order.                  |
| Q102-Q107           | Update consent/frequency/payload/notice/failures/channel, post-test consent placement, and approval of the complete shared product definition.                                              |
