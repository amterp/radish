package radish

import (
	"strconv"
	"strings"
)

// SelectModel is a single-select prompt: a title, a list of options, live
// type-to-filter, and a moving cursor. It is a pure Model - all state, logic, and
// rendering, no I/O - so it is driven identically by the production terminal and
// by scripted test events.
//
// Build one with the chained setters (NewSelect().Title(...).Options(...)), then
// hand it to Run. After Run, read the choice with Selected. The option set, filter,
// cursor, and viewport live in the embedded list, shared with MultiSelect.
type SelectModel struct {
	title     string
	theme     *Theme
	keymap    KeyMap
	summarize func(selected string) string

	list

	canceled bool
}

// NewSelect returns a SelectModel with sane defaults (fuzzy matcher, default
// theme and keymap, 10 visible rows). Customize via the chained setters.
func NewSelect() *SelectModel {
	return &SelectModel{
		theme:  DefaultTheme(),
		keymap: DefaultKeyMap(),
		list:   list{matcher: DefaultMatcher, height: defaultHeight},
	}
}

// Title sets the line shown above the options (the question being asked).
func (m *SelectModel) Title(s string) *SelectModel { m.title = s; return m }

// Options sets the selectable options (in display order).
func (m *SelectModel) Options(opts ...string) *SelectModel {
	m.options = opts
	m.refilter()
	return m
}

// Matcher overrides the filter-matching function. A nil argument is ignored.
func (m *SelectModel) Matcher(fn Matcher) *SelectModel {
	if fn != nil {
		m.matcher = fn
	}
	m.refilter()
	return m
}

// Theme overrides the styling. A nil argument is ignored.
func (m *SelectModel) Theme(t *Theme) *SelectModel {
	if t != nil {
		m.theme = t
	}
	return m
}

// KeyMap overrides the key bindings.
func (m *SelectModel) KeyMap(k KeyMap) *SelectModel { m.keymap = k; return m }

// SummaryFunc overrides the collapsed line shown after submit: fn receives the
// selected option and its result replaces the default summary entirely (an
// empty result collapses to nothing). Cancel still collapses to nothing.
func (m *SelectModel) SummaryFunc(fn func(selected string) string) *SelectModel {
	m.summarize = fn
	return m
}

// Height sets how many option rows are visible at once (the viewport size).
// Non-positive values are ignored.
func (m *SelectModel) Height(n int) *SelectModel {
	if n > 0 {
		m.height = n
	}
	return m
}

// Width caps each option row to this terminal width (with an ellipsis). Zero
// disables truncation.
func (m *SelectModel) Width(n int) *SelectModel {
	if n >= 0 {
		m.width = n
	}
	return m
}

// Selected returns the chosen option and true, or ("", false) if the prompt was
// canceled or nothing matched. Whether a run was canceled is reported by
// Result.Canceled from Run; this reports the value.
func (m *SelectModel) Selected() (string, bool) {
	if m.canceled || m.cursor < 0 || m.cursor >= len(m.matched) {
		return "", false
	}
	return m.options[m.matched[m.cursor]], true
}

// Update advances the model in response to one event. It implements Model.
func (m *SelectModel) Update(e Event) (Model, Cmd) {
	switch {
	case m.keymap.matches(e, m.keymap.Cancel):
		m.canceled = true
		return m, CmdCancel
	case m.keymap.matches(e, m.keymap.Submit):
		if len(m.matched) == 0 {
			return m, CmdNone // nothing to submit
		}
		return m, CmdSubmit
	case m.keymap.matches(e, m.keymap.Up):
		m.moveCursor(-1)
	case m.keymap.matches(e, m.keymap.Down):
		m.moveCursor(1)
	case m.keymap.matches(e, m.keymap.PageUp):
		m.moveCursor(-m.height)
	case m.keymap.matches(e, m.keymap.PageDown):
		m.moveCursor(m.height)
	case m.keymap.matches(e, m.keymap.Home):
		m.cursor = 0
		m.clampViewport()
	case m.keymap.matches(e, m.keymap.End):
		m.cursor = max(0, len(m.matched)-1)
		m.clampViewport()
	case m.keymap.matches(e, m.keymap.ClearFilter):
		if m.filter != "" {
			m.filter = ""
			m.refilter()
		}
	case e.Type == KeyBackspace:
		if m.filter != "" {
			r := []rune(m.filter)
			m.filter = string(r[:len(r)-1])
			m.refilter()
		}
	case e.Type == KeyRune:
		m.filter += string(e.Rune)
		m.refilter()
	}
	return m, CmdNone
}

// View renders the current frame: title, optional filter line, the visible window
// of options with the cursor row marked, and scroll hints. No trailing newline.
// Line count is fully data-driven so the renderer's redraw accounting stays exact.
func (m *SelectModel) View() string {
	var lines []string

	if m.title != "" {
		lines = append(lines, styled(m.theme.Title, m.fit(m.title, 0)))
	}
	if m.filter != "" {
		lines = append(lines, styled(m.theme.Filter, m.fit("/"+m.filter, 0)))
	}

	if len(m.matched) == 0 {
		lines = append(lines, styled(m.theme.ScrollHint, m.fit("  (no matches - backspace to widen)", 0)))
		return strings.Join(lines, "\n")
	}

	if m.offset > 0 {
		lines = append(lines, styled(m.theme.ScrollHint, m.fit("  ↑ "+strconv.Itoa(m.offset)+" more", 0)))
	}

	start, end := m.visibleRange()
	for i := start; i < end; i++ {
		label := m.fit(m.options[m.matched[i]], 2) // reserve 2 cols for the "> " / "  " prefix
		if i == m.cursor {
			lines = append(lines, styled(m.theme.Cursor, "> ")+styled(m.theme.Selected, label))
		} else {
			lines = append(lines, "  "+styled(m.theme.Normal, label))
		}
	}

	if remaining := len(m.matched) - end; remaining > 0 {
		lines = append(lines, styled(m.theme.ScrollHint, m.fit("  ↓ "+strconv.Itoa(remaining)+" more", 0)))
	}

	return strings.Join(lines, "\n")
}

// Summary is the collapsed line shown after submit: prompt plus chosen value.
// It returns "" on cancel so nothing is left behind. Implements Summarizer.
func (m *SelectModel) Summary() string {
	if m.canceled {
		return ""
	}
	sel, ok := m.Selected()
	if !ok {
		return ""
	}
	if m.summarize != nil {
		return m.summarize(sel)
	}
	title := m.title
	if title == "" {
		title = "Selected"
	}
	return styled(m.theme.Title, title) + " " + styled(m.theme.Selected, sel)
}
