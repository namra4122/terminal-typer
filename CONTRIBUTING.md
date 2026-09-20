# Contributing

Read [AGENTS.md](AGENTS.md) before changing the repository. It defines compatibility, privacy, generated-asset, and approval constraints. Scoped maintenance decisions are retained in [`.agents/plans/`](.agents/plans/).

## Local workflow

1. Install Go 1.17 or newer and run `go mod download`.
2. Run `make build` to create `bin/tt`.
3. Run `make verify` and `make smoke` before opening a change.

`make assets` regenerates `src/packed.go` from the source asset directories and generates `tt.1.gz` from `man.md`; it requires Python 3 and Pandoc. Include regenerated artifacts only when their source assets or manual change.

## Changes

Keep documented CLI behavior compatible unless a scoped plan explicitly approves a change. Update `README.md` and `man.md` together when user-facing behavior changes. Do not release artifacts, deploy, or add user-data transmission without explicit human approval.
