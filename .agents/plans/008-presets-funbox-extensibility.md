# Phase 5 - Presets, Funbox, and extensibility

- Lifecycle: planning outline. Write one small implementation spec per persistent format or modifier family.
- Goal: make a preferred setup repeatable and allow safe local content and challenge extensions.
- User outcome: load a named preset and use optional test modifiers without manually rebuilding a configuration.

## Current starting point

- `src/settings.go:24-36` stores six versioned live controls; `src/tt.go:434-464` owns CLI flags and `src/tt.go:487-511` resolves explicit settings overrides.
- `src/db.go:9-55` establishes local state paths. `src/tt.go:240-267` and `src/tt.go:474-483` load and list resources. `Makefile:39-43` generates embedded assets from source directories.
- Phases 1-4 supply mode, content, metrics, layout, training, and UI configuration for presets and modifiers.

## Spec candidates

1. Define a versioned preset schema containing mode, language or pack, theme, difficulty, duration or word count, text modifiers, sounds, layout, live widgets, and UI mode. Specify precedence among built-in defaults, saved settings, selected preset, and explicit CLI flags.
1. Ship a small set of named built-in presets for ordinary English, focused timed typing, code practice, and layout practice. Add local user presets with create, rename, replace, delete, and validation flows.
1. Add `--preset NAME` loading while retaining existing short flags. Add interactive preset switching through the Phase 3 command palette, showing which settings will change before a running test restarts.
1. Add local preset import and export with version checks, conflict handling, and no hidden path or user-content leakage.
1. Define a Funbox modifier interface over generated test content, session behavior, and PB eligibility. Specify deterministic ordering, compatibility rules, display labels, and reasons a combination is rejected.
1. Implement initial Funbox candidates in bounded groups: memory, random case, reverse, no-space, symbols, ROT13, disappearing text, weakspot, speed ramp, and sudden death. Validate combinations against quote, code, custom, and practice modes instead of applying transformations blindly.
1. Define versioned local resource formats for language packs, quotes, themes, logical layouts, sounds, and declarative Funbox configurations where safe. Validate size, encoding, and schema before loading. Do not execute untrusted code as a resource format.
1. Add user-installed content discovery and clearer `tt -list` output while preserving existing resource names and lookup paths. Document the local config directory and the process for creating community resources.
1. Add explicit settings and data migrations and an export/import route for portable local configuration. Keep user text, progress, and mistakes on the user's machine.
1. Add bash, zsh, and fish completion scripts generated or tested against actual CLI options. Update the manual and README as public commands land.

## Completion signals

- A user can save a configuration, launch it with `tt --preset NAME`, and switch to it interactively; explicit CLI flags still win for that invocation.
- Every shipped Funbox declares compatible modes and PB eligibility, and rejected combinations produce a clear error before a test starts.
- User-installed resources load from documented local paths, invalid resources fail visibly, and migration preserves existing settings and history.
- Shell completions, README, and manual match the executable. `make verify` and `make smoke` pass.

## Boundaries and dependencies

- Requires stable configuration from Phases 1, 3, and 4 and result eligibility from Phase 2.
- Multiplayer, public leaderboards, accounts, cloud sync, hosted APIs, and executable plugins are outside this local-only plan. A later proposal for any of them must explicitly revisit the privacy and network contract.
- Do not release or publish artifacts as part of implementing this plan without explicit human approval.
