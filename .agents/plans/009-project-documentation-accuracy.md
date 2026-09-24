# Project documentation accuracy

- Lifecycle: completed documentation record for the current checkout.
- Goal: let readers distinguish verified behavior in the source tree, historical release notes, and proposed roadmap work.

## Scope

- Update `AGENTS.md` and `CONTRIBUTING.md` with explicit source-of-truth and plan-status guidance.
- Update `README.md` with current checkout behavior and a link to the five-phase roadmap.
- Correct stale options and examples in `man.md`, then regenerate `tt.1.gz` with `make assets`.
- Label the older TUI design draft and runtime-settings plan by their present status. Keep historical release notes as history.

## Evidence and compatibility

- The executable uses Go 1.17, `tcell`, and `beep` (`go.mod:1-9`, `src/tt.go:19-20`); it has a six-row persistent settings modal (`src/settings.go:24-36`, `src/typer.go:418-438`).
- Current output contains WPM, CPM, accuracy, timestamp, and mistakes (`src/tt.go:27-33`, `src/tt.go:63-91`). It does not yet contain the richer metrics or history described in roadmap plans `004`-`008`.
- Existing input selection, flags, and resource lookup remain unchanged (`src/tt.go:434-548`, `src/util.go:201-225`). This plan changes documentation and generated manual only.

## Completion checks

- [x] A reader can identify what exists now, what the released `0.4.2` history says, and what is proposed.
- [x] The manual and README describe the same public behavior and valid examples.
- [x] `make assets`, `make verify`, and `make smoke` passed; generated changes were limited to `tt.1.gz` and inspected before completion.
