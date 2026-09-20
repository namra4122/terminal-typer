# Agent guide

## Sources of truth

- `README.md` defines public installation, usage, and supported behavior.
- `man.md` is the source for the generated `tt(1)` manual; regenerate it with `make assets` after changing it.
- `Makefile` owns executable build, verification, smoke, install, and release commands.
- `CHANGELOG.md` records released changes. `.agents/plans/` holds scoped implementation specifications.

## Product map

`tt` is a local, scriptable terminal typing-test CLI. `src/` contains the application; `src/packed.go` is generated from `themes/`, `words/`, `quotes/`, and `sounds/` by `make assets`. Do not edit `src/packed.go` directly. `scripts/` contains the asset-generation tools.

## Stable constraints

- Preserve documented flags, stdin/file input behavior, resource lookup, and CSV/JSON output unless an approved plan explicitly changes the contract.
- Keep user text, progress, and mistake data local. State is stored in `$XDG_DATA_HOME/tt` or `~/.local/share/tt`; do not add telemetry or network transmission.
- Keep the CLI self-contained: application code may use embedded resources and local user resources; generated assets flow from source assets into `src/packed.go`.
- Preserve the existing Go and Makefile stack. Introduce dependencies or CI changes only when required by scoped work.

## Change workflow

Create a numbered plan in `.agents/plans/` for cross-cutting, compatibility-affecting, generated-asset, dependency, or CI changes. Retain completed plans as implementation records.

Before reporting a change complete, run `make verify` and `make smoke`. Update `README.md` and `man.md` together for public behavior changes; update `CHANGELOG.md` for released changes.

Do not release, deploy, publish artifacts, or upload user data without explicit human approval.
