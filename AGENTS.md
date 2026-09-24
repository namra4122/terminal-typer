# Agent guide

## Sources of truth

- `README.md` defines public installation, usage, and supported behavior.
- `man.md` is the source for the generated `tt(1)` manual; regenerate it with `make assets` after changing it.
- `Makefile` owns executable build, verification, smoke, install, and release commands.
- `CHANGELOG.md` is historical release history, not a complete description of the current checkout. `.agents/plans/README.md` indexes completed records, a design draft, and proposed roadmap outlines; a plan is not evidence that its features exist.

## Product map

`tt` is a local, scriptable terminal typing-test CLI written in Go 1.17 with `tcell`. The current checkout has random-word, quote, file, and stdin input; a persistent six-control Settings modal; built-in and local themes and sounds; and final WPM, CPM, accuracy, and mistake reporting. `src/tt.go` selects input, flags, reporting, and output; `src/typer.go` runs the test; `src/settings.go` owns saved live controls; `src/db.go` owns local state paths; and `src/layout.go` and `src/theme.go` provide shared drawing primitives. `src/packed.go` is generated from `themes/`, `words/`, `quotes/`, and `sounds/` by `make assets`. Edit source assets rather than `src/packed.go`.

The five-phase roadmap in plans `004`-`008` proposes additional modes, language packs, analytics, a command palette, training, layouts, presets, and Funbox. Those features are not implemented merely because a plan names them. The older `003-tui-redesign.md` is a design reference with Charm examples; inspect the current `tcell` implementation before applying any part of it.

## Stable constraints

- Preserve documented flags, stdin/file input behavior, resource lookup, and CSV/JSON output unless an approved plan explicitly changes the contract.
- Keep user text, progress, and mistake data local. State is stored in `$XDG_DATA_HOME/tt` or `~/.local/share/tt`; do not add telemetry or network transmission.
- Keep the CLI self-contained: application code may use embedded resources and local user resources; generated assets flow from source assets into `src/packed.go`.
- Preserve the existing Go and Makefile stack. Introduce dependencies or CI changes only when required by scoped work.

## Change workflow

Inspect the relevant code and `git status` before a change. Create a numbered plan in `.agents/plans/` for cross-cutting, compatibility-affecting, generated-asset, dependency, or CI changes. Retain completed plans as implementation records and label proposals as proposals.

Before reporting a change complete, run `make verify` and `make smoke`. Update `README.md` and `man.md` together for public behavior changes; update `CHANGELOG.md` for released changes.

Do not release, deploy, publish artifacts, or upload user data without explicit human approval.
