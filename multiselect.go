package radish

import (
	"strconv"
	"strings"
)

// MultiSelectModel is a multi-select prompt: a title, a list of options with live
// type-to-filter and a moving cursor (all shared with Select via the embedded
// list), plus a per-option selection toggle and optional min/max bounds on how many
// may be chosen. It is a pure Model, driven identically in production and in tests.
//
// Build one with the chained setters (NewMultiSelect().Title(...).Options(...)),
// then hand it to Run. After Run, read the chosen labels with Selected.
//
// Tab or Space toggles the row under the cursor. With a Max set, toggling a new
// option on is blocked once Max are selected (deselect is always allowed). With a
// Min set, Enter does nothing until at least Min are selected.
type MultiSelectModel struct {
	title     string
	hint      string
	theme     *Theme
	keymap    KeyMap
	summarize func(selected []string) string

	list

	selected  map[int]bool // keyed by option index
	preselect []string     // labels to mark selected; reapplied when options change
	min       int
	max       int // 0 = unlimited

	canceled bool
}

// NewMultiSelect returns a MultiSelectModel with sane defaults (fuzzy matcher,
// default theme and keymap, 10 visible rows, no min/max). Customize via the setters.
func NewMultiSelect() *MultiSelectModel {
	return &MultiSelectModel{
		theme:    DefaultTheme(),
		keymap:   DefaultKeyMap(),
		list:     list{matcher: DefaultMatcher, height: defaultHeight},
		selected: map[int]bool{},
	}
}

// Title sets the line shown above the options (the question being asked).
func (m *MultiSelectModel) Title(s string) *MultiSelectModel { m.title = s; return m }

// Hint sets an interaction hint rendered as a faint footer line, e.g.
// "space to toggle, enter to confirm" for first-time users. Empty = no line.
func (m *MultiSelectModel) Hint(s string) *MultiSelectModel { m.hint = s; return m }

// Options sets the selectable options (in display order).
func (m *MultiSelectModel) Options(opts ...string) *MultiSelectModel {
	m.options = opts
	m.refilter()
	m.applyPreselect()
	return m
}

// Matcher overrides the filter-matching function. A nil argument is ignored.
func (m *MultiSelectModel) Matcher(fn Matcher) *MultiSelectModel {
	if fn != nil {
		m.matcher = fn
	}
	m.refilter()
	return m
}

// Theme overrides the styling. A nil argument is ignored.
func (m *MultiSelectModel) Theme(t *Theme) *MultiSelectModel {
	if t != nil {
		m.theme = t
	}
	return m
}

// KeyMap overrides the key bindings.
func (m *MultiSelectModel) KeyMap(k KeyMap) *MultiSelectModel { m.keymap = k; return m }

// Height sets how many option rows are visible at once (the viewport size).
// Non-positive values are ignored.
func (m *MultiSelectModel) Height(n int) *MultiSelectModel {
	if n > 0 {
		m.height = n
	}
	return m
}

// Width caps each option row to this terminal width (with an ellipsis). Zero
// disables truncation.
func (m *MultiSelectModel) Width(n int) *MultiSelectModel {
	if n >= 0 {
		m.width = n
	}
	return m
}

// Min sets the minimum number of options that must be selected before Enter
// submits. Negative values are ignored.
func (m *MultiSelectModel) Min(n int) *MultiSelectModel {
	if n >= 0 {
		m.min = n
	}
	return m
}

// Max sets the maximum number of options that may be selected (0 = unlimited).
// Negative values are ignored.
func (m *MultiSelectModel) Max(n int) *MultiSelectModel {
	if n >= 0 {
		m.max = n
	}
	return m
}

// Preselect marks the options with the given labels as already selected, so the
// prompt opens with them checked (e.g. reflecting defaults). Labels that match
// no option are ignored. Order-independent with Options: labels are remembered
// and reapplied whenever the options change.
func (m *MultiSelectModel) Preselect(labels ...string) *MultiSelectModel {
	m.preselect = append(m.preselect, labels...)
	m.applyPreselect()
	return m
}

func (m *MultiSelectModel) applyPreselect() {
	for _, label := range m.preselect {
		for i, opt := range m.options {
			if opt == label {
				m.selected[i] = true
			}
		}
	}
}

// SummaryFunc overrides the collapsed line shown after submit: fn receives the
// selected labels (in option order) and its result replaces the default summary
// entirely (an empty result collapses to nothing). Cancel still collapses to
// nothing.
func (m *MultiSelectModel) SummaryFunc(fn func(selected []string) string) *MultiSelectModel {
	m.summarize = fn
	return m
}

// Selected returns the chosen option labels in original option order.
func (m *MultiSelectModel) Selected() []string {
	var out []string
	for i, opt := range m.options {
		if m.selected[i] {
			out = append(out, opt)
		}
	}
	return out
}

// Update advances the model in response to one event. It implements Model.
func (m *MultiSelectModel) Update(e Event) (Model, Cmd) {
	switch {
	case m.keymap.matches(e, m.keymap.Cancel):
		m.canceled = true
		return m, CmdCancel
	case m.keymap.matches(e, m.keymap.Submit):
		// Unlike Select (which submits the cursor row, so an empty match set has
		// nothing to submit), MultiSelect submits the accumulated selection, which
		// is independent of the current filter view - so Enter submits even while
		// filtered to no matches, as long as the minimum is met.
		if len(m.selected) >= m.min {
			return m, CmdSubmit
		}
		return m, CmdNone // under min: submit blocked
	case m.keymap.matches(e, m.keymap.Toggle) || (e.Type == KeyRune && e.Rune == ' '):
		m.toggle()
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

// toggle flips the selection of the option under the cursor, honoring the Max bound
// (selecting more than Max is blocked; deselecting is always allowed).
func (m *MultiSelectModel) toggle() {
	idx, ok := m.currentIndex()
	if !ok {
		return
	}
	if m.selected[idx] {
		delete(m.selected, idx)
		return
	}
	if m.max > 0 && len(m.selected) >= m.max {
		return
	}
	m.selected[idx] = true
}

// View renders the title, optional filter line, the visible window of options with
// checkboxes and the cursor row marked, scroll hints, and a min hint while under
// the minimum. No trailing newline; line count is fully data-driven.
func (m *MultiSelectModel) View() string {
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
		idx := m.matched[i]
		box := "[ ] "
		if m.selected[idx] {
			box = "[x] "
		}
		label := m.fit(m.options[idx], 6) // reserve 2 for "> "/"  " + 4 for "[x] "
		if i == m.cursor {
			lines = append(lines, styled(m.theme.Cursor, "> ")+styled(m.theme.Selected, box+label))
		} else {
			lines = append(lines, "  "+box+styled(m.theme.Normal, label))
		}
	}

	if remaining := len(m.matched) - end; remaining > 0 {
		lines = append(lines, styled(m.theme.ScrollHint, m.fit("  ↓ "+strconv.Itoa(remaining)+" more", 0)))
	}

	// A single status hint: how many more are needed below the minimum, or that
	// the maximum is reached (which explains why further toggles do nothing). The
	// "N more" phrasing stays informative even when the title already states the
	// bound, and it updates live as the selection changes.
	switch {
	case m.min > 0 && len(m.selected) < m.min:
		need := m.min - len(m.selected)
		lines = append(lines, styled(m.theme.ScrollHint, m.fit("  (select "+strconv.Itoa(need)+" more)", 0)))
	case m.max > 0 && len(m.selected) >= m.max:
		lines = append(lines, styled(m.theme.ScrollHint, m.fit("  (max "+strconv.Itoa(m.max)+" selected)", 0)))
	}

	if m.hint != "" {
		lines = append(lines, styled(m.theme.ScrollHint, m.fit("  "+m.hint, 0)))
	}

	return strings.Join(lines, "\n")
}

// Summary is the collapsed line shown after submit: title plus the chosen values.
// Returns "" on cancel. A SummaryFunc, when set, replaces the default rendering.
// Implements Summarizer.
func (m *MultiSelectModel) Summary() string {
	if m.canceled {
		return ""
	}
	if m.summarize != nil {
		return m.summarize(m.Selected())
	}
	title := m.title
	if title == "" {
		title = "Selected"
	}
	return styled(m.theme.Title, title) + " " + styled(m.theme.Selected, strings.Join(m.Selected(), ", "))
}
