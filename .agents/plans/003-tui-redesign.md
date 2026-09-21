# TUI Visual Redesign Spec

## Goal

Refactor the existing Go TUI so it has a polished, modern terminal UI similar to well-designed applications such as ATAC, lazygit, k9s, and other Charm/Bubble Tea-based applications.

The goal is **not to change business logic or keyboard behavior unless necessary**. The main focus is:

- visual hierarchy
- seamless borders
- consistent colors
- reusable style primitives
- focused/selected states
- clean spacing
- terminal responsiveness
- consistent rendering across the whole app

The final result should feel like a cohesive application, not a collection of individually rendered text blocks.

---

# 1. Tech stack

Use the Charm ecosystem if the project is not already using an equivalent TUI framework.

Preferred:

```go
github.com/charmbracelet/bubbletea
github.com/charmbracelet/lipgloss
github.com/charmbracelet/bubbles
```

Responsibilities:

```text
Bubble Tea
  → application state
  → keyboard events
  → terminal resize events
  → update loop

Lip Gloss
  → colors
  → borders
  → padding
  → alignment
  → width/height
  → rendering styles

Bubbles
  → viewport / textarea / list / table widgets where useful
```

Do not manually embed raw ANSI escape codes throughout the codebase unless absolutely necessary.

---

# 2. Design principles

The UI should follow these principles.

### Minimal but structured

Avoid excessive decoration.

Do not put borders around every piece of information.

Use borders primarily for:

- top-level panes
- major sections
- modal/dialog boundaries
- focused editing areas

Use whitespace and typography/color for smaller grouping.

---

### Strong hierarchy

There should be visually distinct levels:

```text
Application title
Section title
Control label
Control value
Muted/help text
Active/focused item
Status/footer information
```

Do not render all text with the same foreground color and weight.

---

### Consistent alignment

Settings and properties should be rendered as two-column rows.

Bad:

```text
Show WPM: On
Cursor style: Bar
Typed text weight: Bold
```

Preferred:

```text
Show WPM                              ● On
Cursor style                           Bar
Typed text weight                      Bold
Word highlighting            Current + next
```

Values should align to the right edge of their available region where possible.

---

# 3. Theme system

Create one centralized theme definition.

Do not hardcode colors throughout individual components.

Suggested structure:

```go
type Theme struct {
    Background lipgloss.Color
    Text       lipgloss.Color
    Muted      lipgloss.Color
    Subtle     lipgloss.Color
    Border     lipgloss.Color
    BorderFocus lipgloss.Color
    Accent     lipgloss.Color
    Success    lipgloss.Color
    Warning    lipgloss.Color
    Error      lipgloss.Color
    Info       lipgloss.Color
    SelectedBG lipgloss.Color
}
```

Example palette:

```go
var DefaultTheme = Theme{
    Background:  lipgloss.Color("#11111B"),
    Text:        lipgloss.Color("#CDD6F4"),
    Muted:       lipgloss.Color("#7F849C"),
    Subtle:      lipgloss.Color("#585B70"),
    Border:      lipgloss.Color("#585B70"),
    BorderFocus: lipgloss.Color("#89B4FA"),
    Accent:      lipgloss.Color("#F9E2AF"),
    Success:     lipgloss.Color("#A6E3A1"),
    Warning:     lipgloss.Color("#F9E2AF"),
    Error:       lipgloss.Color("#F38BA8"),
    Info:        lipgloss.Color("#89DCEB"),
    SelectedBG:  lipgloss.Color("#313244"),
}
```

Exact colors may be adjusted, but preserve the semantic distinction.

---

# 4. Reusable style primitives

Create reusable style variables/functions rather than styling inline everywhere.

Example:

```go
type Styles struct {
    AppTitle     lipgloss.Style
    SectionTitle lipgloss.Style
    Text         lipgloss.Style
    Muted        lipgloss.Style
    Border       lipgloss.Style
    FocusBorder  lipgloss.Style
    SelectedRow  lipgloss.Style
    Key          lipgloss.Style
    FooterText   lipgloss.Style
    Success      lipgloss.Style
    Warning      lipgloss.Style
    Error        lipgloss.Style
}
```

Construct them once from the active theme.

Example:

```go
func NewStyles(theme Theme) Styles
```

---

# 5. Border system

Use Unicode box-drawing characters.

Use:

```text
─ │
┌ ┐
└ ┘
├ ┤
┬ ┴
┼
```

Rounded borders are acceptable for outer dialogs:

```text
╭ ╮
╰ ╯
```

Example:

```go
lipgloss.NormalBorder()
```

or:

```go
lipgloss.RoundedBorder()
```

---

# 6. Seamless panel joins

Important requirement:

Avoid rendering adjacent panes as completely independent boxes when they visually belong to the same layout.

Avoid this:

```text
┌──────────────┐┌──────────────────────┐
│ Left         ││ Right                │
│              ││                      │
└──────────────┘└──────────────────────┘
```

Prefer:

```text
┌──────────────┬───────────────────────┐
│ Left         │ Right                 │
│              │                       │
└──────────────┴───────────────────────┘
```

For complex layouts, the parent layout should control shared borders/separators.

If Lip Gloss composition alone creates doubled borders, explicitly suppress overlapping borders or create shared separators at the parent level.

---

# 7. Settings screen target

Redesign the Settings screen toward this structure:

```text
╭──────────────────────────────────────────────────────────╮
│ Settings                                                 │
│ Configure typing behavior and appearance                 │
│                                                          │
│ GENERAL                                                  │
│                                                          │
│  › Show WPM                                      ● On    │
│    Skip word on Space                            ● On    │
│    Allow Backspace                               ● On    │
│                                                          │
│ APPEARANCE                                               │
│                                                          │
│    Cursor style                                   Bar    │
│    Typed text weight                              Bold   │
│    Word highlighting                    Current + next   │
│                                                          │
├──────────────────────────────────────────────────────────┤
│ ↑↓ Navigate   Space Toggle   Enter Change   Esc Close    │
╰──────────────────────────────────────────────────────────╯
```

---

# 8. Selected row behavior

Selection must be visually obvious.

Do not rely only on:

```text
>
```

The active row should combine:

- left indicator
- accent color
- bold label
- subtle selected background

Example visual:

```text
  › Show WPM                                      ● On
```

Recommended selected style:

```go
lipgloss.NewStyle().
    Background(theme.SelectedBG).
    Foreground(theme.Text).
    Bold(true)
```

The `›` indicator should use the accent color.

The selected background should span the full content width of the row, not only the text.

---

# 9. Boolean values

Render booleans semantically.

Preferred:

```text
● On
○ Off
```

Use:

```text
On  → Success color
Off → Muted color
```

Example:

```go
func renderBool(value bool) string {
    if value {
        return successStyle.Render("● On")
    }

    return mutedStyle.Render("○ Off")
}
```

Do not use loud red for `Off`, because disabled is not necessarily an error.

---

# 10. Enum/settings values

For values such as:

```text
Bar
Bold
Current + next
```

Use a subtle value style.

Selected/focused value can use Accent.

Normal value can use Text or Muted depending on importance.

Example:

```text
Cursor style                              Bar
Typed text weight                         Bold
Word highlighting               Current + next
```

---

# 11. Section titles

Section titles should be visually distinct but not overly loud.

Example:

```text
GENERAL
APPEARANCE
```

Recommended style:

```go
lipgloss.NewStyle().
    Foreground(theme.Muted).
    Bold(true)
```

Optionally use uppercase.

Do not put large borders around each section.

---

# 12. Header/title

Use a modern title layout.

Preferred:

```text
Settings
Configure typing behavior and appearance
```

rather than only:

```text
              Settings
```

Title:

- bold
- primary text color

Subtitle:

- muted
- smaller visual weight

Example:

```go
title := styles.AppTitle.Render("Settings")
subtitle := styles.Muted.Render(
    "Configure typing behavior and appearance",
)
```

---

# 13. Footer/status bar

The footer should be a dedicated region.

Add a separator:

```text
├──────────────────────────────────────────────────────────┤
│ ↑↓ Navigate   Space Toggle   Enter Change   Esc Close    │
╰──────────────────────────────────────────────────────────╯
```

Key names should be visually distinct.

Example:

```text
↑↓ Navigate
Space Toggle
Enter Change
Esc Close
```

Render key tokens using Accent or stronger text.

Render action descriptions using Muted.

Example helper:

```go
func keyHint(key, description string) string {
    return keyStyle.Render(key) +
        " " +
        footerTextStyle.Render(description)
}
```

---

# 14. Focused borders

If multiple panes exist, focused panes should differ from unfocused panes.

Example:

```text
Focused:
border = Accent / BorderFocus

Inactive:
border = Border
```

Do not completely change background colors between panes.

The border difference should be subtle.

---

# 15. Tabs

Tabs should not look like desktop-style boxed buttons.

Render them inline:

```text
Params │ Auth │ Headers (6) │ Body (JSON)
```

Inactive tabs:

```text
Muted
```

Active tab:

```text
Accent + Bold
```

Example:

```go
func renderTab(label string, active bool) string
```

Do not give every tab a border unless the app design clearly requires it.

---

# 16. Status and semantic colors

Use semantic colors consistently.

Example:

```text
Success  → green
Warning  → yellow
Error    → red
Info     → cyan/blue
Accent   → yellow/blue depending on theme
```

For HTTP methods, if relevant:

```text
GET     → green
POST    → yellow
PUT     → blue
PATCH   → cyan
DELETE  → red
```

Use these sparingly.

---

# 17. Responsive layout

Handle terminal resize events.

In Bubble Tea:

```go
case tea.WindowSizeMsg:
    m.width = msg.Width
    m.height = msg.Height
```

The UI must adapt to available terminal dimensions.

Do not assume a fixed terminal size.

---

## Minimum width behavior

At smaller widths:

1. preserve label readability
2. truncate values if needed
3. reduce padding
4. hide optional subtitle/help text before breaking layout
5. never allow borders to wrap incorrectly

Example:

```go
available := width - horizontalPadding - borderWidth
```

Use `lipgloss.Width()` when calculating rendered strings because ANSI sequences do not have visible width.

Do not use raw `len()` for styled strings.

---

# 18. Truncation

Create a safe truncation helper for long values.

Example behavior:

```text
Current + next + future…
```

Never allow a setting value to overflow through the right border.

Prefer terminal-cell-aware truncation.

If using Lip Gloss utilities or runewidth-aware functions, use those instead of byte length.

---

# 19. Spacing

Maintain consistent spacing.

Suggested layout constants:

```go
const (
    HorizontalPadding = 2
    VerticalPadding   = 1
    SectionGap        = 1
)
```

Avoid random spaces embedded into strings.

Centralize spacing rules where practical.

---

# 20. Component architecture

Separate rendering into small functions.

Example:

```go
func (m Model) View() string {
    return renderSettingsScreen(m)
}
```

Then:

```go
func renderSettingsScreen(m Model) string
func renderHeader(m Model) string
func renderSection(title string, rows []SettingRow) string
func renderSettingRow(row SettingRow, selected bool, width int) string
func renderFooter(m Model) string
```

Do not create one huge `View()` function with styling mixed everywhere.

---

# 21. Setting row model

Prefer a declarative structure.

Example:

```go
type SettingRow struct {
    Label string
    Value string
    Kind  SettingKind
}
```

Possible:

```go
type SettingKind int

const (
    SettingBoolean SettingKind = iota
    SettingEnum
    SettingText
)
```

Then rendering can be generic.

---

# 22. Layout calculations

For each row:

```text
| padding | indicator | label | flexible space | value | padding |
```

Pseudo logic:

```go
indicatorWidth := 2
valueWidth := lipgloss.Width(renderedValue)

availableLabelWidth :=
    contentWidth -
    indicatorWidth -
    valueWidth -
    1
```

Then generate spacing so values align.

Do not hardcode arbitrary spacing like:

```go
"Show WPM                    On"
```

Calculate it from width.

---

# 23. Example render function

Codex can use something approximately like:

```go
func renderSettingRow(
    label string,
    value string,
    selected bool,
    width int,
) string {
    indicator := "  "

    if selected {
        indicator = styles.Accent.Render("› ")
    }

    renderedLabel := styles.Text.Render(label)
    renderedValue := styles.Muted.Render(value)

    used :=
        lipgloss.Width(indicator) +
        lipgloss.Width(renderedLabel) +
        lipgloss.Width(renderedValue)

    gap := width - used

    if gap < 1 {
        gap = 1
    }

    row :=
        indicator +
        renderedLabel +
        strings.Repeat(" ", gap) +
        renderedValue

    if selected {
        row = styles.SelectedRow.
            Width(width).
            Render(row)
    }

    return row
}
```

Adjust as needed for ANSI width correctness.

---

# 24. Avoid common TUI problems

Do not introduce these issues.

### Double borders

Avoid:

```text
┌──────┐┌──────┐
```

when panels should share a separator.

---

### Too much color

Avoid:

```text
green label
blue value
purple border
yellow heading
red selection
```

Use mostly neutral colors with one accent.

Rule of thumb:

```text
~80–90% neutral
~10–20% semantic/accent
```

---

### Excessive bold text

Only use bold for:

- title
- section title if needed
- selected row
- important status

---

### ASCII borders

Do not use:

```text
--------------------
|                  |
--------------------
```

Use Unicode box-drawing characters.

---

### Hardcoded terminal dimensions

Do not assume:

```go
width := 80
height := 24
```

Read terminal size from `tea.WindowSizeMsg`.

---

### Raw string length

Do not use:

```go
len(styledString)
```

for layout math.

ANSI sequences distort byte length.

Use:

```go
lipgloss.Width(...)
```

or another cell-width-aware method.

---

# 25. Optional improvement: shared layout primitives

If multiple screens use panels, create reusable primitives such as:

```go
type Pane struct {
    Title   string
    Width   int
    Height  int
    Focused bool
}
```

or render functions:

```go
func RenderPane(
    title string,
    content string,
    width int,
    height int,
    focused bool,
) string
```

Also consider:

```go
func RenderTabs(...)
func RenderFooter(...)
func RenderSettingRow(...)
func RenderStatus(...)
```

The goal is for all screens to automatically inherit the same visual language.

---

# 26. Implementation priority

Implement in this order.

1. Add centralized `Theme`
2. Add centralized `Styles`
3. Convert ASCII borders to Unicode/Lip Gloss borders
4. Implement responsive width/height handling
5. Refactor Settings rendering into reusable rows
6. Add aligned label/value columns
7. Add selection background + accent indicator
8. Add semantic boolean rendering
9. Add dedicated footer
10. Refactor remaining screens to use shared styles
11. Fix overlapping/double borders between panes
12. Add polish for narrow terminals

---

# 27. Acceptance criteria

The implementation is considered complete when:

- no major screen uses ASCII `-` / `|` borders
- major adjacent panes have seamless/shared-looking separators
- colors come from one centralized theme
- focused and selected components are immediately distinguishable
- selected rows use more than just a `>` marker
- boolean settings visually distinguish `On` and `Off`
- values align consistently across rows
- footer keyboard hints are styled consistently
- window resize does not break the UI
- long text cannot overflow borders
- rendered width calculations are ANSI-aware
- no component individually invents random colors
- the visual language is consistent across screens
- business logic and existing keyboard behavior remain intact

---

# 28. Desired aesthetic

Use the attached ATAC screenshot only as **visual inspiration**, not as something to reproduce pixel-for-pixel.

Target characteristics:

```text
dark background
thin muted borders
seamless pane separators
subtle accent color
semantic status colors
dense but readable spacing
minimal decoration
strong active/focus state
aligned values
compact footer
```

The desired result should feel closer to:

```text
ATAC
lazygit
k9s
modern Charm-based TUIs
```

than to a traditional ncurses configuration dialog.

---

## One more thing

> Before editing code, inspect the current rendering architecture and identify whether borders are drawn by each component independently or by a shared parent layout. Preserve existing state/update logic wherever possible. Refactor rendering/style concerns separately instead of rewriting working application behavior.
