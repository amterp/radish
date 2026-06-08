package radish

import "github.com/amterp/color"

// Theme holds the styles applied while rendering. Each field is an amterp/color
// Color (or nil for "no styling"). Because color.Sprint returns plain text when
// color is disabled (color.NoColor / NO_COLOR), themed output collapses to clean
// plain text automatically - which is what makes rendered frames stable in tests
// with no special handling.
type Theme struct {
	Title      *color.Color // prompt / title line
	Cursor     *color.Color // the "> " pointer on the active row
	Selected   *color.Color // the active row's label
	Normal     *color.Color // inactive rows (nil = plain)
	Filter     *color.Color // the typed filter line
	ScrollHint *color.Color // "↑/↓ N more" and "(no matches)"
}

// DefaultTheme is a tasteful default: bold title, green cursor/selection, faint
// filter and scroll hints.
func DefaultTheme() *Theme {
	return &Theme{
		Title:      color.New(color.Bold),
		Cursor:     color.New(color.FgGreen, color.Bold),
		Selected:   color.New(color.FgGreen),
		Normal:     nil,
		Filter:     color.New(color.Faint),
		ScrollHint: color.New(color.Faint),
	}
}

// styled applies c to s, treating a nil Color as plain passthrough.
func styled(c *color.Color, s string) string {
	if c == nil {
		return s
	}
	return c.Sprint(s)
}
