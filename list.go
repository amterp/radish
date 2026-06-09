package radish

import (
	"sort"

	runewidth "github.com/mattn/go-runewidth"
)

const defaultHeight = 10

// list is the shared option-list core: the filterable set of options, the live
// filter and its matched subset, the cursor, and the scrolling viewport, plus the
// math to move the cursor, keep it on screen, and width-truncate a line. The
// list-style prompts (Select, MultiSelect) embed it and compose their own
// Update/View over this state, so the navigation/filtering logic lives in exactly
// one place.
type list struct {
	options []string
	matcher Matcher
	height  int
	width   int // 0 = no truncation

	filter  string
	matched []int // indices into options that pass the matcher, in display order
	cursor  int   // index into matched
	offset  int   // viewport top: index into matched
}

// currentIndex returns the option index under the cursor (an index into options),
// and false when nothing is matched.
func (l *list) currentIndex() (int, bool) {
	if l.cursor < 0 || l.cursor >= len(l.matched) {
		return 0, false
	}
	return l.matched[l.cursor], true
}

// visibleRange is the half-open [start, end) slice of matched currently on screen.
func (l *list) visibleRange() (start, end int) {
	return l.offset, min(l.offset+l.height, len(l.matched))
}

func (l *list) moveCursor(delta int) {
	if len(l.matched) == 0 {
		return
	}
	l.cursor = min(max(l.cursor+delta, 0), len(l.matched)-1)
	l.clampViewport()
}

func (l *list) clampViewport() {
	if l.cursor < l.offset {
		l.offset = l.cursor
	}
	if l.cursor >= l.offset+l.height {
		l.offset = l.cursor - l.height + 1
	}
	if l.offset < 0 {
		l.offset = 0
	}
}

// refilter recomputes the matched set against the current filter, stably ordered
// by (rank, original index), and resets the cursor/viewport to the top.
func (l *list) refilter() {
	type scored struct{ idx, rank int }
	var s []scored
	for i, opt := range l.options {
		if ok, rank := l.matcher(l.filter, opt); ok {
			s = append(s, scored{i, rank})
		}
	}
	sort.SliceStable(s, func(a, b int) bool { return s[a].rank < s[b].rank })

	l.matched = l.matched[:0]
	for _, sc := range s {
		l.matched = append(l.matched, sc.idx)
	}
	l.cursor = 0
	l.offset = 0
}

// fit truncates plain text s to the list's width (0 = unlimited), reserving
// `reserve` columns for a fixed prefix. See fitWidth for the why.
func (l *list) fit(s string, reserve int) string {
	return fitWidth(s, l.width, reserve)
}

// fitWidth truncates plain text s to width (0 = unlimited), reserving `reserve`
// columns for a fixed prefix so the styled line never exceeds the terminal width.
// Truncating the plain text *before* styling keeps it correct under color: ANSI
// codes are zero-width on screen but would otherwise confuse a width-aware
// truncator. Bounding every line this way keeps each frame line to one visual row,
// which the inline renderer relies on for correct redraws.
func fitWidth(s string, width, reserve int) string {
	if width <= 0 {
		return s
	}
	return runewidth.Truncate(s, max(1, width-reserve), "…")
}
