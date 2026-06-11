package radish

import "github.com/amterp/color"

// Theme holds the styles applied while rendering. Each field is an amterp/color
// Color (or nil for "no styling"). Because color.Sprint returns plain text when
// color is disabled (color.NoColor / NO_COLOR), themed output collapses to clean
// plain text automatically - which is what makes rendered frames stable in tests
// with no special handling.
//
// One Theme serves the whole prompt family; a component uses only the fields it
// renders and ignores the rest (e.g. Placeholder is Input-only; Filter/ScrollHint
// are used by the list-based prompts).
type Theme struct {
	Title       *color.Color // prompt / title line
	Cursor      *color.Color // the "> " pointer on the active row, or the text cursor
	Selected    *color.Color // the active row's label
	Normal      *color.Color // inactive rows / typed text (nil = plain)
	Filter      *color.Color // the typed filter line
	Placeholder *color.Color // an input's ghost/hint text shown while empty
	ScrollHint  *color.Color // "↑/↓ N more" and "(no matches)"
	Error       *color.Color // an input's validation error line
}

// DefaultTheme is a tasteful default: bold title, green cursor/selection, faint
// filter, placeholder, and scroll hints, red validation errors.
func DefaultTheme() *Theme {
	return &Theme{
		Title:       color.New(color.Bold),
		Cursor:      color.New(color.FgGreen, color.Bold),
		Selected:    color.New(color.FgGreen),
		Normal:      nil,
		Filter:      color.New(color.Faint),
		Placeholder: color.New(color.Faint),
		ScrollHint:  color.New(color.Faint),
		Error:       color.New(color.FgRed),
	}
}

// styled applies c to s, treating a nil Color as plain passthrough.
func styled(c *color.Color, s string) string {
	if c == nil {
		return s
	}
	return c.Sprint(s)
}
