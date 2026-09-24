package main

import (
	"strings"

	"github.com/gdamore/tcell"
	"github.com/mattn/go-runewidth"
)

// Border describes the Unicode box-drawing cells used by a panel. Join cells
// are included so a parent layout can build shared separators without joining
// two independent boxes and producing doubled borders.
type Border struct {
	Top         rune
	Bottom      rune
	Left        rune
	Right       rune
	TopLeft     rune
	TopRight    rune
	BottomLeft  rune
	BottomRight rune
	LeftJoin    rune
	RightJoin   rune
	TopJoin     rune
	BottomJoin  rune
	Cross       rune
}

// NormalBorder returns the thin square border used for ordinary panes.
func NormalBorder() Border {
	return Border{
		Top:         '─',
		Bottom:      '─',
		Left:        '│',
		Right:       '│',
		TopLeft:     '┌',
		TopRight:    '┐',
		BottomLeft:  '└',
		BottomRight: '┘',
		LeftJoin:    '├',
		RightJoin:   '┤',
		TopJoin:     '┬',
		BottomJoin:  '┴',
		Cross:       '┼',
	}
}

// RoundedBorder returns the rounded variant for outer dialogs and modals.
// Its joins remain square so it can still be used with parent separators.
func RoundedBorder() Border {
	border := NormalBorder()
	border.TopLeft = '╭'
	border.TopRight = '╮'
	border.BottomLeft = '╰'
	border.BottomRight = '╯'
	return border
}

// TopLine renders the top edge for a box with the requested total width.
func (b Border) TopLine(width int) string {
	return borderLine(b.TopLeft, b.Top, b.TopRight, width)
}

// BottomLine renders the bottom edge for a box with the requested total width.
func (b Border) BottomLine(width int) string {
	return borderLine(b.BottomLeft, b.Bottom, b.BottomRight, width)
}

// Render returns the complete border as one string per row. Width and height
// are measured in terminal cells, not bytes or runes.
func (b Border) Render(width, height int) []string {
	if width <= 0 || height <= 0 {
		return nil
	}

	lines := make([]string, height)
	if height == 1 {
		lines[0] = b.TopLine(width)
		return lines
	}

	lines[0] = b.TopLine(width)
	if width == 1 {
		for y := 1; y < height-1; y++ {
			lines[y] = string(b.Left)
		}
	} else {
		middle := string(b.Left) + strings.Repeat(" ", width-2) + string(b.Right)
		for y := 1; y < height-1; y++ {
			lines[y] = middle
		}
	}
	lines[height-1] = b.BottomLine(width)
	return lines
}

func borderLine(left, fill, right rune, width int) string {
	if width <= 0 {
		return ""
	}
	if width == 1 {
		return string(left)
	}
	return string(left) + strings.Repeat(string(fill), width-2) + string(right)
}

// Rect is a terminal-cell rectangle. Width and Height are total dimensions,
// including any border cells.
type Rect struct {
	X      int
	Y      int
	Width  int
	Height int
}

// Inset returns the rectangle inside padding cells on every side. A rectangle
// cannot have negative dimensions.
func (r Rect) Inset(padding int) Rect {
	if padding < 0 {
		padding = 0
	}
	width := r.Width - padding*2
	height := r.Height - padding*2
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	return Rect{X: r.X + padding, Y: r.Y + padding, Width: width, Height: height}
}

// DrawBorder draws a border with tcell-native cell writes. Writes outside the
// screen are ignored, which makes this safe during resize transitions.
func DrawBorder(screen tcell.Screen, rect Rect, border Border, style tcell.Style) {
	if screen == nil || rect.Width <= 0 || rect.Height <= 0 {
		return
	}
	for row, line := range border.Render(rect.Width, rect.Height) {
		DrawText(screen, rect.X, rect.Y+row, line, style)
	}
}

// DrawBox is an alias that reads naturally at call sites rendering a panel.
func DrawBox(screen tcell.Screen, rect Rect, border Border, style tcell.Style) {
	DrawBorder(screen, rect, border, style)
}

// DrawRule draws a horizontal Unicode separator. It is intentionally separate
// from DrawBorder because parent layouts own shared separators.
func DrawRule(screen tcell.Screen, x, y, width int, glyph rune, style tcell.Style) {
	if screen == nil || width <= 0 {
		return
	}
	DrawText(screen, x, y, strings.Repeat(string(glyph), width), style)
}

// CellWidth returns the maximum visible width of a string in terminal cells.
// Newline-separated strings are measured line by line, making the result useful
// for centered blocks and bounded layout calculations.
func CellWidth(text string) int {
	width := 0
	for _, line := range strings.Split(text, "\n") {
		if lineWidth := runewidth.DefaultCondition.StringWidth(line); lineWidth > width {
			width = lineWidth
		}
	}
	return width
}

// VisibleWidth is a descriptive alias for CellWidth.
func VisibleWidth(text string) int {
	return CellWidth(text)
}

// TruncateCells limits every line to width terminal cells and uses an ellipsis
// when content is removed. It never returns a line wider than width.
func TruncateCells(text string, width int) string {
	if width <= 0 {
		return ""
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if runewidth.DefaultCondition.StringWidth(line) <= width {
			continue
		}
		if width == 1 {
			lines[i] = "…"
			continue
		}
		lines[i] = runewidth.DefaultCondition.Truncate(line, width, "…")
	}
	return strings.Join(lines, "\n")
}

// TruncateToWidth is a descriptive alias for TruncateCells.
func TruncateToWidth(text string, width int) string {
	return TruncateCells(text, width)
}

// PadCells appends spaces until text reaches width terminal cells. Existing
// content is not truncated; use TruncateCells first when a fixed width is
// required.
func PadCells(text string, width int) string {
	padding := width - CellWidth(text)
	if padding <= 0 {
		return text
	}
	return text + strings.Repeat(" ", padding)
}

// AlignRightCells truncates text if necessary and pads it on the left to the
// requested terminal-cell width.
func AlignRightCells(text string, width int) string {
	text = TruncateCells(text, width)
	padding := width - CellWidth(text)
	if padding <= 0 {
		return text
	}
	return strings.Repeat(" ", padding) + text
}

// DrawText writes text without allowing wide or combining runes to corrupt
// neighboring cells. Newlines return to the original x coordinate. The return
// values are the next logical cell position, useful for composing rows.
func DrawText(screen tcell.Screen, x, y int, text string, style tcell.Style) (nextX, nextY int) {
	if screen == nil {
		return x, y
	}
	screenWidth, screenHeight := screen.Size()
	originX := x
	nextX, nextY = x, y
	lastX, lastY := 0, 0
	hasLast := false

	for _, r := range text {
		if r == '\n' {
			nextX = originX
			nextY++
			hasLast = false
			continue
		}

		cellWidth := runewidth.DefaultCondition.RuneWidth(r)
		if cellWidth == 0 {
			if hasLast && lastX >= 0 && lastX < screenWidth && lastY >= 0 && lastY < screenHeight {
				main, combining, existingStyle, _ := screen.GetContent(lastX, lastY)
				combining = append(combining, r)
				screen.SetContent(lastX, lastY, main, combining, existingStyle)
			}
			continue
		}

		if nextX >= 0 && nextX+cellWidth <= screenWidth && nextY >= 0 && nextY < screenHeight {
			screen.SetContent(nextX, nextY, r, nil, style)
			lastX, lastY = nextX, nextY
			hasLast = true
		} else {
			hasLast = false
		}
		nextX += cellWidth
	}
	return nextX, nextY
}

// DrawTextInRect clips text to a rectangle in terminal cells and suppresses
// lines that fall below its bottom edge.
func DrawTextInRect(screen tcell.Screen, rect Rect, text string, style tcell.Style) {
	if screen == nil || rect.Width <= 0 || rect.Height <= 0 {
		return
	}
	for row, line := range strings.Split(text, "\n") {
		if row >= rect.Height {
			break
		}
		DrawText(screen, rect.X, rect.Y+row, TruncateCells(line, rect.Width), style)
	}
}
