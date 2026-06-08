package radish

import (
	"sort"
	"strconv"
	"strings"

	runewidth "github.com/mattn/go-runewidth"
)

const defaultHeight = 10

// SelectModel is a single-select prompt: a title, a list of options, live
// type-to-filter, and a moving cursor. It is a pure Model - all state, logic, and
// rendering, no I/O - so it is driven identically by the production terminal and
// by scripted test events.
//
// Build one with the chained setters (NewSelect().Prompt(...).Options(...)), then
// hand it to Run. After Run, read the choice with Selected.
type SelectModel struct {
	prompt  string
	options []string
	matcher Matcher
	theme   *Theme
	keymap  KeyMap
	height  int
	width   int // 0 = no truncation

	filter  string
	matched []int // indices into options that pass the matcher, in display order
	cursor  int   // index into matched
	offset  int   // viewport top: index into matched

	done     bool
	canceled bool
}

// NewSelect returns a SelectModel with sane defaults (fuzzy matcher, default
// theme and keymap, 10 visible rows). Customize via the chained setters.
func NewSelect() *SelectModel {
	return &SelectModel{
		matcher: DefaultMatcher,
		theme:   DefaultTheme(),
		keymap:  DefaultKeyMap(),
		height:  defaultHeight,
	}
}

// Prompt sets the title line shown above the options.
func (m *SelectModel) Prompt(s string) *SelectModel { m.prompt = s; return m }

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
// canceled or nothing matched.
func (m *SelectModel) Selected() (string, bool) {
	if m.canceled || m.cursor < 0 || m.cursor >= len(m.matched) {
		return "", false
	}
	return m.options[m.matched[m.cursor]], true
}

// Canceled reports whether the prompt was aborted (Esc/Ctrl-C/Ctrl-D or EOF).
func (m *SelectModel) Canceled() bool { return m.canceled }

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
		m.done = true
		return m, CmdSubmit
	case m.keymap.matches(e, m.keymap.Up):
		m.moveCursor(-1)
	case m.keymap.matches(e, m.keymap.Down):
		m.moveCursor(1)
	case m.keymap.matches(e, m.keymap.Home):
		m.cursor = 0
		m.clampViewport()
	case m.keymap.matches(e, m.keymap.End):
		m.cursor = max(0, len(m.matched)-1)
		m.clampViewport()
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

	if m.prompt != "" {
		lines = append(lines, styled(m.theme.Title, m.prompt))
	}
	if m.filter != "" {
		lines = append(lines, styled(m.theme.Filter, "/"+m.filter))
	}

	if len(m.matched) == 0 {
		lines = append(lines, styled(m.theme.ScrollHint, "  (no matches)"))
		return strings.Join(lines, "\n")
	}

	if m.offset > 0 {
		lines = append(lines, styled(m.theme.ScrollHint, "  ↑ "+strconv.Itoa(m.offset)+" more"))
	}

	end := min(m.offset+m.height, len(m.matched))
	for i := m.offset; i < end; i++ {
		label := m.truncate(m.options[m.matched[i]])
		if i == m.cursor {
			lines = append(lines, styled(m.theme.Cursor, "> ")+styled(m.theme.Selected, label))
		} else {
			lines = append(lines, "  "+styled(m.theme.Normal, label))
		}
	}

	if remaining := len(m.matched) - end; remaining > 0 {
		lines = append(lines, styled(m.theme.ScrollHint, "  ↓ "+strconv.Itoa(remaining)+" more"))
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
	title := m.prompt
	if title == "" {
		title = "Selected"
	}
	return styled(m.theme.Title, title) + " " + styled(m.theme.Selected, sel)
}

func (m *SelectModel) moveCursor(delta int) {
	if len(m.matched) == 0 {
		return
	}
	m.cursor = min(max(m.cursor+delta, 0), len(m.matched)-1)
	m.clampViewport()
}

func (m *SelectModel) clampViewport() {
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+m.height {
		m.offset = m.cursor - m.height + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

// refilter recomputes the matched set against the current filter, stably ordered
// by (rank, original index), and resets the cursor/viewport to the top.
func (m *SelectModel) refilter() {
	type scored struct{ idx, rank int }
	var s []scored
	for i, opt := range m.options {
		if ok, rank := m.matcher(m.filter, opt); ok {
			s = append(s, scored{i, rank})
		}
	}
	sort.SliceStable(s, func(a, b int) bool { return s[a].rank < s[b].rank })

	m.matched = m.matched[:0]
	for _, sc := range s {
		m.matched = append(m.matched, sc.idx)
	}
	m.cursor = 0
	m.offset = 0
}

func (m *SelectModel) truncate(s string) string {
	if m.width <= 0 {
		return s
	}
	avail := max(1, m.width-2) // leave room for the "> " / "  " prefix
	return runewidth.Truncate(s, avail, "…")
}
