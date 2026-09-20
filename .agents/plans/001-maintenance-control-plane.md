# Maintenance control plane

- **Lifecycle:** completed implementation record; retain for future maintenance.
- **User outcome:** contributors and coding agents can safely maintain the existing `tt` terminal typing-test CLI with one authoritative build, verification, and smoke path.

## Scope

Establish repository guidance, canonical Make targets, clean-checkout CI, and contributor-facing maintenance instructions. Repair the source-install dependency on the generated manual.

**Non-goals:** redesigning the CLI, changing its public flags or output, modernizing the Go baseline, adding telemetry or hosted services, or changing release artifacts.

## Current-state evidence

- `go.mod` declares module `tt`, Go 1.17, `tcell`, `beep`, and `go-isatty`.
- `src/tt.go` implements the terminal CLI; `src/db.go` keeps local state under XDG data or the user home directory.
- `themes/`, `words/`, `quotes/`, and `sounds/` are packed into generated `src/packed.go` by `make assets`.
- Before this plan, `Makefile` had only build/install/asset/release targets, no automated verification or CI; its `install` target assumed `tt.1.gz` existed although a clean checkout lacks it.

## Architecture and invariants

`src/` owns runtime behavior. Source assets own embedded content; `src/packed.go` is generated. `man.md` owns the generated `tt.1.gz` manual. The Makefile owns executable commands; documentation names targets rather than duplicating recipes.

Compatibility invariant: retain documented CLI flags, stdin/file handling, resource lookup paths, and CSV/JSON outputs. Privacy invariant: user content, progress, and mistakes remain local; no telemetry or network transmission.

## Vertical phases

1. **Executable maintenance contract:** add canonical formatting, static-analysis, test, build, and CLI resource-list smoke targets; make source installation build both binary and manual.
2. **Reproducible enforcement:** run the same verification and smoke targets in GitHub Actions using the Go version declared by `go.mod`.
3. **Discoverable operations:** add stable agent and contributor instructions that point to the Makefile and identify generated assets.

## Failure modes and rollback

Missing `pandoc` prevents manual generation and source installation; build, verification, and smoke remain independent of it. If CI support for Go 1.17 becomes unavailable, update the Go baseline only through a compatibility-scoped plan. Revert this control-plane change as a unit if its commands prove incompatible with a supported checkout.

## Verification and smoke

```sh
make verify
make smoke
```

`make smoke` builds `bin/tt` and executes `tt -list words`, demonstrating that the real executable starts without terminal interaction and reads embedded assets.

## Completion checklist

- [x] Canonical commands exist in `Makefile`.
- [x] `install` has declared binary and manual prerequisites.
- [x] CI invokes the canonical verification and smoke targets from a clean checkout.
- [x] Stable agent and contributor guidance points to authoritative sources.
- [x] `make verify` and `make smoke` observed after the changes.
