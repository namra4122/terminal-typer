# Runtime settings window

- **Lifecycle:** implemented compatibility record. The six-row Settings modal and saved controls exist in `src/settings.go` and `src/typer.go`; inspect current source for exact behavior.
- **User outcome:** a typist can save live typing controls for future `tt` sessions without restarting, while every documented CLI flag keeps its existing syntax and overrides the saved value only for the invocation that supplied it.

## Overview

### Phase 1 — Live typing controls

Add a modal, persistent Settings window to the active typing screen. `Ctrl-P` pauses the test, changes saved preferences, and resumes the exact same test; explicit CLI flags override the matching saved preference for only the current process and never rewrite it. Command-line flags continue to own test selection, scripting, input, and resource-loading behavior.

```text
Phase 1: Live typing controls
```

**Sequencing rationale:** This is one bounded vertical slice: it adds the complete user-visible workflow, adapts the existing `tcell` event loop, preserves timer correctness, and documents the new key. Splitting the overlay from applying its settings would leave either a non-functional screen or settings that cannot be reached. No follow-on phase is required for the requested capability.

**Parallelization:** None. The event-loop changes, settings state, tests, and documentation share one runtime contract and should land in one PR.

## Phase 1 — Live typing controls

### Product

#### Phase goal

Unlock in-session adjustment of live typing controls through a Settings window opened with `Ctrl-P`, without removing or redefining any CLI flag.

#### User story / job to be done

As a typist already in a test, I can press `Ctrl-P`, save a behavior or presentation preference for future runs, close the window, and continue the same test without losing typed characters or consuming timed-test seconds. If I deliberately supply a conflicting CLI flag, that one run follows the flag without overwriting my saved preference.

#### Assumptions and decisions

- Settings changed through `Ctrl-P` persist in `$XDG_DATA_HOME/tt/settings.json` or `~/.local/share/tt/settings.json`; the directory remains mode `0700` and the file mode is `0600`. Existing `.db` file-progress and `.errors` mistake stores retain their formats and purposes.
- Startup precedence is fixed: built-in defaults, then valid persisted settings, then only **explicitly supplied** live-control flags. `flag.Visit` determines explicit flags, so omitted flags do not overwrite a saved preference. `-showwpm=false` is explicit and overrides only the current session.
- The live-control flag groups are `-showwpm`, `-noskip`, `-nobackspace`, `-blockcursor`, `-bold`, and the combined highlighting group `-nohighlight`/`-highlight1`/`-highlight2`. Any explicitly supplied highlighting flag overrides the stored highlighting mode for that process; existing `-nohighlight`, then `-highlight1`, then `-highlight2` precedence applies.
- An explicitly flagged row is shown in Settings as `CLI override` and its active value cannot be changed for this process. Space/Enter still changes and saves that row's persistent value for a future run; the row immediately displays both its unchanged active value and its changed `Saved:` value. A row without a CLI override changes both the active and saved value.
- Settings are committed only when `Ctrl-P` or Escape closes the modal. The modal edits a draft plus a set of changed rows; it merges only changed rows into the persisted value. This prevents an unrelated setting change from copying a one-session CLI override into `settings.json`.
- If the atomic settings write fails, the modal remains open, the active settings remain unchanged, and a visible error is rendered in the modal. The user may retry after correcting the filesystem or press `Ctrl-C` to exit. The close keys do not silently discard a changed draft after a write failure.
- Invalid, unreadable, or unsupported-version settings files are ignored without modification, startup uses defaults plus CLI overrides, and `tt` prints one warning to stderr before entering the TUI. No new dependency or generated asset source changes are needed. The project remains Go 1.17 with `tcell` v1.4.0 (`go.mod:1-9`).

#### In scope

1. Add a versioned, persistent runtime-settings document at `settings.json` inside the existing local data directory and load it after `-list`/`-v` early exits but before a TUI is created.
2. Build the active runtime settings from defaults, persisted values, and explicitly supplied live-control flags in that exact precedence order.
3. Add a centered `Settings` modal to the active typing screen, opened and closed by `Ctrl-P`.
4. Offer six settings rows, in this fixed order:
   1. `Show WPM` — `On` / `Off`.
   2. `Skip word on Space` — `On` / `Off`.
   3. `Allow Backspace` — `On` / `Off`.
   4. `Cursor style` — `Bar` / `Block`.
   5. `Typed text weight` — `Normal` / `Bold`.
   6. `Word highlighting` — `Current + next` / `Current only` / `Next only` / `Off`.
5. Support `Up`/`Down` to select a row and `Space` or `Enter` to toggle a Boolean or advance the two-/four-value choice. Selection wraps from last to first and first to last.
6. Save only rows changed through the modal. For non-overridden rows, apply the saved value to the active typer after close. For CLI-overridden rows, retain the CLI value until process exit.
7. Preserve the exact typed buffer, current cursor position, generated test, report history, and navigation index while the modal is open.
8. Pause elapsed-time accounting while the modal is open. This applies to both the countdown from `-t` and the duration used for live WPM and final report statistics.
9. Redraw the typing screen after the modal closes and apply eligible selected controls before accepting the next typing key.
10. Keep `Ctrl-C` as an immediate program exit from both the typing screen and the Settings modal.
11. Update public key/configuration documentation in `README.md` and `man.md`, then regenerate `tt.1.gz` with `make assets`.

#### Out of scope

- Editing test content or source after startup: `-words`, `-quotes`, positional file input, stdin, `-n`, `-g`, `-start`, `-raw`, and `-multi` remain CLI-only.
- Editing timer duration, report/output behavior, one-shot behavior, sound resources, color theme, resource lists, or version behavior in the modal: `-t`, `-noreport`, `-oneshot`, `-csv`, `-json`, `-sound`, `-error-sound`, `-theme`, `-notheme`, `-list`, and `-v` remain CLI-only.
- A settings modal on the report screen, `-list`, `-v`, or noninteractive output paths. The modal exists only while `typer.start` is processing an active test.
- Mouse input, scrolling, configurable key bindings, a GUI process, network activity, telemetry, or new dependencies.

#### Acceptance criteria

1. Existing invocations using every currently registered flag parse and retain their current behavior; the flag declarations in `src/tt.go:329-359` remain available under the same names and defaults.
2. On startup, omitted live-control flags preserve the corresponding valid `settings.json` value; an explicitly supplied live-control flag overrides only that setting for the active process and does not modify `settings.json`.
3. During an active test, `Ctrl-P` displays a centered modal headed `Settings` with exactly the six rows and control legend specified above; `Ctrl-P` or `Escape` closes it after a successful save.
4. `Up`/`Down` wrap through the six rows; `Space` and `Enter` alter only the selected row; typing runes and Left/Right do not change test input or test navigation while the modal is open.
5. Closing Settings atomically writes only its changed rows to a version-1, mode-`0600` settings file. A write error remains visibly actionable in the modal and changes neither the file nor active settings.
6. A changed non-overridden row affects the next redraw/input exactly as its CLI equivalent does. A CLI-overridden row retains its CLI behavior for the session while its new Saved value is used by the next invocation that omits that flag.
7. Opening and closing the modal preserves the active test text and all characters typed before `Ctrl-P`; completing the test reports those same characters in the existing result format.
8. For a timed test, the wall-clock interval between opening and closing the modal contributes 0 seconds to the timer, live WPM duration, and reported test duration.
9. `Ctrl-C` exits with the existing failure status from the modal.
10. A terminal smaller than 52 columns or 14 rows shows `Terminal too small for settings (need 52x14)` and only accepts `Ctrl-P`, `Escape`, `Ctrl-C`, or resize; it neither mutates settings nor test input.
11. An unreadable, invalid, or unknown-version settings file produces one stderr warning, falls back to defaults plus explicitly supplied flags, and is unchanged.
12. `README.md` and `man.md` document `Ctrl-P`, persistence, CLI precedence, and modal navigation; regenerated `tt.1.gz` matches `man.md`.
13. `make verify` and `make smoke` pass after the implementation.

#### UX flow

1. The user starts `tt`. The application loads defaults, then `settings.json`, then overlays only explicitly supplied live-control flags.
2. At any point on the typing screen, the user presses `Ctrl-P`. If timing has started, timing pauses before the modal is rendered; otherwise the first typed rune still starts timing after close.
3. The application clears the screen and draws a 52-column, 14-row centered modal with `Settings`, six label/value rows, a `>` marker on the selected row, and `Up/Down select · Space/Enter change · Esc/Ctrl-P save & close` as the footer. CLI-overridden rows append `CLI override; Saved: <value>`.
4. The user uses Up/Down and Space/Enter. The modal redraws the draft value. Typing keys are ignored rather than added to the test.
5. The user presses Escape or Ctrl-P. If no row changed, the application closes immediately. Otherwise it atomically saves only changed values; success redraws the original test with applicable styles/control behavior and resumes its event loop. Write failure leaves the modal open with its error.
6. `Ctrl-C` exits through the existing termination path. A resize redraws the modal at the new size; if it falls below 52x14 the compact size message replaces the controls until the screen becomes large enough or the user closes/exits.

### Engineering

#### What already exists

- `main` defines/parses flags in `src/tt.go:296-362` and has `-list`/`-v` early exits at `src/tt.go:364-379`.
- `typer` owns live controls and its event loop in `src/typer.go:36-85,175-435`.
- `src/db.go:13-53` already owns the private local-data directory and JSON state files. This phase extends that local-only store.
- `tcell` v1.4.0 provides `KeyCtrlP` and `NewSimulationScreen` (`key.go:396-400`, `simulation.go:24-68`).

#### Components touched

- `src/db.go`: add `RUNTIME_SETTINGS_DB` alongside current local-state paths.
- New `src/settings.go`: schema/defaults, explicit-flag overlay, strict loading, atomic saving, menu draft/dirty tracking, and modal UI.
- `src/tt.go`: load settings after `-list`/`-v`, collect visited flags, merge settings, and initialize the typer.
- `src/typer.go`: retain colors, apply effective settings, dispatch `Ctrl-P`, pause timing, commit the modal draft, and redraw.
- New `src/settings_test.go`: persistence, precedence, error, and simulation-screen coverage.
- `README.md`, `man.md`, `tt.1.gz`: document persistence, precedence, and controls.

#### Data model

No existing file migrates. The new `settings.json` document is:

```json
{
  "version": 1,
  "settings": {
    "showWPM": false,
    "skipWord": true,
    "allowBackspace": true,
    "blockCursor": false,
    "boldTypedText": false,
    "highlight": "current-and-next"
  }
}
```

```go
type highlightMode string
const (
    highlightCurrentAndNext highlightMode = "current-and-next"
    highlightCurrentOnly highlightMode = "current-only"
    highlightNextOnly highlightMode = "next-only"
    highlightOff highlightMode = "off"
)
type runtimeSettings struct {
    ShowWPM bool `json:"showWPM"`
    SkipWord bool `json:"skipWord"`
    AllowBackspace bool `json:"allowBackspace"`
    BlockCursor bool `json:"blockCursor"`
    BoldTypedText bool `json:"boldTypedText"`
    Highlight highlightMode `json:"highlight"`
}
type persistedSettings struct {
    Version int `json:"version"`
    Settings runtimeSettings `json:"settings"`
}
type settingsOverrides struct {
    ShowWPM, SkipWord, AllowBackspace bool
    BlockCursor, BoldTypedText bool
    Highlight bool
}
```

- Only version `1`, all six fields, and the four declared highlight strings are valid. Missing/invalid fields, an unreadable file, or an unknown version produces the specified warning and leaves the file untouched.
- Missing `settings.json` is normal: defaults apply and the file is created only after a successful modal change.
- `settingsOverrides` is built only with `flag.Visit`; it records explicitly supplied groups. Any visited highlight flag marks that combined group. `runtimeSettings` is the persisted baseline; the typer receives a separate effective copy after applying overrides.
- The modal tracks dirty rows. It reloads `settings.json` immediately before close, merges only dirty rows, and atomically saves with a mode-`0600` temp file in the same directory followed by `os.Rename`. Save error removes the temp file and returns an error; it never calls the existing panic-on-write `writeValue`.

For strict presence validation, `loadPersistedSettings` must unmarshal into a private wire struct whose `version`, six Boolean settings, and `highlight` fields are pointers. It rejects the file when any pointer is nil before converting to `persistedSettings`; `false` is valid because its pointer is non-nil. Unknown JSON object keys are ignored for forward compatibility.

#### API contracts

| Input | Context | Result |
| --- | --- | --- |
| `Ctrl-P` | Typing | Pause and open Settings. |
| `Ctrl-P` / `Escape` | Unchanged modal | Close and resume. |
| `Ctrl-P` / `Escape` | Changed modal | Save then close only on success. |
| `Up` / `Down` | Settings | Select previous / next row, wrapping. |
| `Space` / `Enter` | Settings | Change draft; overridden rows change only Saved value. |
| `Ctrl-C` | Settings | Existing `TyperSigInt` exit path. |
| Resize | Settings | Redraw and remain open. |

```go
func loadPersistedSettings(path string) (runtimeSettings, error)
func savePersistedSettings(path string, settings runtimeSettings) error
func collectSettingsOverrides(visited map[string]bool) settingsOverrides
func effectiveRuntimeSettings(saved runtimeSettings, overrides settingsOverrides, flags flagValues) runtimeSettings
func showSettings(screen tcell.Screen, saved *runtimeSettings, overrides settingsOverrides, flags flagValues) (committed bool, interrupted bool)
func (t *typer) applyRuntimeSettings()
```

`flagValues` is a private struct containing only parsed live-control flags. `showSettings` must not mutate test text, generate a test, initialize audio, call `os.Exit`, or save an unchanged draft.

#### Key flows

1. Parse flags unchanged; preserve `-list`/`-v` exits. Load defaults then `settings.json`; print exactly `tt: ignoring runtime settings: <error>` to stderr for a non-missing load error. Overlay only explicit live-control flags.
2. Open Settings before `startTime` initialization. It edits a saved-settings draft and dirty-row set. A CLI-overridden row displays `CLI override; Saved: <value>`; its current process value does not change.
3. On close with changes, reload disk state, merge only dirty fields, atomically save, recompute effective settings from saved plus overrides, apply it, and add the full modal interval to a non-zero `startTime`. On save failure, keep the modal/draft open and leave active settings unchanged.
4. Applying settings maps the first three controls to existing typer fields, writes `\033[2 q` (Block) or `\033[5 q` (Bar), and rebuilds bold/highlight styles from original colors. Test exit resets the cursor with `\033[2 q`.
5. The test's text, typed runes, index, time budget, attribution, tests, and result history remain unchanged. Runes/arrows are consumed by the modal.

#### Dependencies on prior phases

None. This is the only phase.

#### Non-functional requirements

- No new modules, network, telemetry, subprocesses, or environment variables.
- `settings.json` is mode `0600`; replacement is atomic within the existing data directory. A successful save exposes no partial JSON.
- Modal creates no goroutine; full UI requires 52x14; modal time is excluded exactly, with deterministic test time.
- Two processes use last-successful-close-wins for the same row. Reload-before-merge preserves disjoint row changes.

#### File reference table

| File | Change |
| --- | --- |
| `src/db.go:10-53` | Add `RUNTIME_SETTINGS_DB`; preserve state formats. |
| `src/settings.go` | Add persistence, precedence, atomic I/O, draft UI. |
| `src/typer.go:36-85,175-435` | Apply settings and preserve input/timer behavior. |
| `src/tt.go:296-453` | Collect explicit flags, load/merge/warn, initialize typer. |
| `src/settings_test.go` | Add persistence and UI integration tests. |
| `README.md:58-90` | Document key, path, precedence, navigation. |
| `man.md:53-100,203-209` | Document matching behavior. |
| `tt.1.gz` | Regenerate through `make assets`; never hand edit. |

#### Failure modes

| Codepath | Failure | Handling / visibility | Coverage |
| --- | --- | --- | --- |
| Load | Corrupt, unsupported, unreadable config. | One stderr warning; defaults plus flags; file unchanged. | Unit tests each class. |
| Merge | Omitted flag overwrites saved value or flag leaks to disk. | `flag.Visit` mask plus dirty merge. | Unit precedence matrix, including explicit false/highlights. |
| Save | Permission/disk/rename error. | Clean temp; visible modal error; no close or active change. | Injected I/O failure test. |
| Concurrent save | Disjoint edits overwrite each other. | Reload-before-merge. | Two-copy integration test. |
| Modal/timer | Input leaks or pause counts. | Consume input; adjust start time. | Simulation + deterministic clock tests. |
| Style/small screen | Bad styles/cursor or draw overflow. | Original-color rebuild; minimum-size no-mutation path. | Unit + 51x13 simulation + terminal smoke. |

No silent, untested, or unhandled failure is permitted.

#### Test plan

| Layer | What | Count |
| --- | --- | ---: |
| Unit | Validation, missing file, version, atomic-save cleanup. | +6 |
| Unit | Defaults/persisted/explicit flag precedence, false and highlights. | +5 |
| Unit | Dirty merge, live mapping, deterministic pause. | +5 |
| Integration | Simulation navigation, isolation, size, Ctrl-C. | +4 |
| Integration | Persist/restart/conflicting flag/non-mutation. | +2 |
| E2E | Save/restart/flag override; timed pause. | 2 scenarios |

#### Rollout

- No feature flag. `settings.json` is created only after a successful Settings change.
- Users without it retain existing defaults. All CLI/input/resource/output contracts remain unchanged.
- Rollback reverts the PR; old binaries ignore the harmless local file. Never delete preferences automatically.

#### Definition of done

- [ ] Criteria 1–2: flags retain contracts; explicit flags override only this process.
- [ ] Criteria 3–6: menu, atomic persistence, override display, and application work as specified.
- [ ] Criteria 7–10: test state, timer, Ctrl-C, and small screens are proven.
- [ ] Criteria 11–13: invalid config warning, docs/manual regeneration, `make verify`, and `make smoke` pass.
