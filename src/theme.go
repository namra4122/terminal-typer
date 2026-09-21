package main

import "github.com/gdamore/tcell"

// Theme contains the semantic colors shared by every terminal view.
//
// Components should choose a role from the active theme instead of introducing
// their own colors. Keeping the palette in tcell.Color also lets the same
// values be used by both text and screen-cell rendering.
type Theme struct {
	Background  tcell.Color
	Text        tcell.Color
	Muted       tcell.Color
	Subtle      tcell.Color
	Border      tcell.Color
	BorderFocus tcell.Color
	Accent      tcell.Color
	Success     tcell.Color
	Warning     tcell.Color
	Error       tcell.Color
	Info        tcell.Color
	SelectedBG  tcell.Color
}

// DefaultTheme is the application's default dark, low-contrast palette.
// Semantic colors are deliberately muted so that accent and status values carry
// visual meaning without turning the screen into a collection of bright colors.
var DefaultTheme = Theme{
	Background:  tcell.NewRGBColor(17, 17, 27),
	Text:        tcell.NewRGBColor(205, 214, 244),
	Muted:       tcell.NewRGBColor(127, 132, 156),
	Subtle:      tcell.NewRGBColor(88, 91, 112),
	Border:      tcell.NewRGBColor(88, 91, 112),
	BorderFocus: tcell.NewRGBColor(137, 180, 250),
	Accent:      tcell.NewRGBColor(249, 226, 175),
	Success:     tcell.NewRGBColor(166, 227, 161),
	Warning:     tcell.NewRGBColor(249, 226, 175),
	Error:       tcell.NewRGBColor(243, 139, 168),
	Info:        tcell.NewRGBColor(137, 220, 235),
	SelectedBG:  tcell.NewRGBColor(49, 50, 68),
}

// Styles is the set of reusable text styles derived from a Theme. Styles are
// values, so a screen can construct them once and use them without mutating a
// shared global style.
type Styles struct {
	AppTitle     tcell.Style
	Subtitle     tcell.Style
	SectionTitle tcell.Style
	Text         tcell.Style
	Value        tcell.Style
	Muted        tcell.Style
	Subtle       tcell.Style
	Border       tcell.Style
	FocusBorder  tcell.Style
	SelectedRow  tcell.Style
	Key          tcell.Style
	FooterText   tcell.Style
	Success      tcell.Style
	Warning      tcell.Style
	Error        tcell.Style
	Info         tcell.Style
	Indicator    tcell.Style
}

// NewStyles creates the complete style vocabulary for theme. The background is
// included in each style so a selected row can be filled without relying on a
// screen-wide default style.
func NewStyles(theme Theme) Styles {
	base := tcell.StyleDefault.
		Foreground(theme.Text).
		Background(theme.Background)

	return Styles{
		AppTitle:     base.Bold(true),
		Subtitle:     base.Foreground(theme.Muted),
		SectionTitle: base.Foreground(theme.Muted).Bold(true),
		Text:         base,
		Value:        base,
		Muted:        base.Foreground(theme.Muted),
		Subtle:       base.Foreground(theme.Subtle),
		Border:       base.Foreground(theme.Border),
		FocusBorder:  base.Foreground(theme.BorderFocus),
		SelectedRow:  base.Foreground(theme.Text).Background(theme.SelectedBG).Bold(true),
		Key:          base.Foreground(theme.Accent).Bold(true),
		FooterText:   base.Foreground(theme.Muted),
		Success:      base.Foreground(theme.Success),
		Warning:      base.Foreground(theme.Warning),
		Error:        base.Foreground(theme.Error),
		Info:         base.Foreground(theme.Info),
		Indicator:    base.Foreground(theme.Accent).Bold(true),
	}
}

// DefaultStyles is ready-to-use styling for callers that use DefaultTheme.
// Screens with a user-selected palette should call NewStyles with that theme.
var DefaultStyles = NewStyles(DefaultTheme)
