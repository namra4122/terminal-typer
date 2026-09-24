# Phase 3 - TUI experience and visual polish

- Lifecycle: planning outline. Reuse the visual principles in `003-tui-redesign.md`; treat that existing plan as design input, not as a feature already implemented.
- Goal: make the typing, results, settings, history, and statistics workflows easy to control from the keyboard.
- User outcome: launch `tt` without flags and configure a test through a coherent terminal interface.

## Current starting point

- `src/layout.go:13-268` provides border, cell-width, truncation, and `tcell` drawing helpers. `src/theme.go:10-69` centralizes palette and style roles.
- `src/settings.go:24-36` persists six live controls, and `src/typer.go:418-438` opens the settings modal on `Ctrl-P`. `src/tt.go:94-229` renders a results report.
- `src/tt.go:240-267` loads existing local or built-in themes. `src/typer.go:145-155` plays key and error sounds. The phase 2 result, history, and graph models supply the new views.

## Spec candidates

1. Define screen and modal navigation for typing, results, settings, command palette, history, and statistics. Preserve Escape restart and `Ctrl-P` settings behavior for existing users; choose a distinct palette binding and show it in the UI and docs.
1. Add focus mode that hides optional chrome while preserving the prompt, caret, essential timer or completion state, and exit controls. Add tape mode that keeps the active text centered or in a fixed reading area while advancing.
1. Add selectable live widgets for WPM, raw WPM, accuracy, errors, time, progress, and burst. Store visibility choices with the existing runtime settings using an explicit migration. Offer compact and full arrangements that fit current terminal dimensions.
1. Define customizable header, status, and footer content using a small set of supported fields. Keep controls discoverable even when optional bars are hidden.
1. Extend caret choices and test text rendering, including current, next, correct, and error states. Prototype smooth caret only where terminal refresh behavior is predictable; always provide an immediate-motion fallback.
1. Extend the existing theme schema and ship curated dark and light themes: Catppuccin, Gruvbox, Tokyo Night, Nord, Dracula, Rose Pine, and at least one light option. Specify automatic light/dark selection only where terminal capabilities expose a reliable signal; otherwise offer explicit selection.
1. Extend optional sounds from key and error to test success and PB events. Ensure silent terminals and unavailable audio devices remain usable.
1. Add a searchable command palette for settings, mode, language, theme, modifiers, history, statistics, restart, and eventually preset commands. Use fuzzy matching with visible labels and keyboard navigation; wire preset actions after Phase 5 adds presets.
1. Make all screens responsive to terminal size, Unicode width, and resize. Provide a readable compact state instead of clipping, and keep an active session's timing and text intact when possible.
1. Add restrained completion feedback in the terminal with a reduced-motion or off option. Font family, font size, and image backgrounds belong to the terminal emulator, so document terminal-controlled styling rather than claiming the app can set them portably.

## Completion signals

- A new user can start a test, change mode and language, finish it, inspect history, and return to typing using only the keyboard.
- Focus and tape modes remain usable at supported terminal sizes. Small terminals show an actionable fallback rather than overlapping content.
- Existing keys and settings precedence remain compatible, and no sound or animation blocks input. `make verify` and `make smoke` pass.

## Boundaries and dependencies

- Requires Phase 1's mode and content configuration and Phase 2's metrics and local views.
- `003-tui-redesign.md` already specifies the palette, borders, and layout style. Review its implementation status before copying or revising any of its work.
- Phase 5 supplies real preset commands; before then the palette exposes only implemented actions.
