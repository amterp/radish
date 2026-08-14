package radish

import (
	"strings"
	"testing"
)

// feed drives a sequence of events through the editor, returning the final Cmd.
func feed(m *EditorModel, evs ...Event) Cmd {
	var cmd Cmd
	for _, e := range evs {
		_, cmd = m.Update(e)
	}
	return cmd
}

// typeRunes lives in input_test.go.

func TestEditorTypingAndSubmit(t *testing.T) {
	m := NewEditor()
	cmd := feed(m, append(typeRunes("x = 1"), KeyEvent(KeyEnter))...)
	if cmd != CmdSubmit {
		t.Fatalf("cmd = %v, want CmdSubmit", cmd)
	}
	if got, ok := m.Value(); !ok || got != "x = 1" {
		t.Errorf("Value() = %q, %v; want \"x = 1\", true", got, ok)
	}
	if m.Outcome() != EditorSubmitted {
		t.Errorf("Outcome() = %v, want EditorSubmitted", m.Outcome())
	}
}

// With an IsComplete hook, Enter opens a new line instead of submitting - the
// whole point of the hook.
func TestEditorEnterContinuesWhenIncomplete(t *testing.T) {
	m := NewEditor().
		IsComplete(func(s string) bool { return !strings.HasSuffix(s, ":") }).
		Indent(func(string) string { return "    " })

	cmd := feed(m, append(typeRunes("if x:"), KeyEvent(KeyEnter))...)
	if cmd != CmdNone {
		t.Fatalf("cmd = %v, want CmdNone (should continue, not submit)", cmd)
	}
	feed(m, typeRunes("pass")...)
	if got := m.Text(); got != "if x:\n    pass" {
		t.Errorf("Text() = %q, want %q", got, "if x:\n    pass")
	}
}

// A blank line closes an open block even though IsComplete still says no. An
// indentation-scoped language has no block terminator, so without this the
// buffer could never be closed.
func TestEditorBlankLineAlwaysSubmits(t *testing.T) {
	m := NewEditor().IsComplete(func(string) bool { return false })
	feed(m, append(typeRunes("if x:"), KeyEvent(KeyEnter))...)
	feed(m, append(typeRunes("pass"), KeyEvent(KeyEnter))...)

	// Now sitting on a fresh blank line: Enter submits regardless of IsComplete.
	if cmd := feed(m, KeyEvent(KeyEnter)); cmd != CmdSubmit {
		t.Fatalf("cmd = %v, want CmdSubmit on blank line", cmd)
	}
}

func TestEditorBackspaceJoinsLines(t *testing.T) {
	m := NewEditor().IsComplete(func(string) bool { return false })
	feed(m, append(typeRunes("ab"), KeyEvent(KeyEnter))...)
	feed(m, typeRunes("cd")...)
	if got := m.Text(); got != "ab\ncd" {
		t.Fatalf("setup Text() = %q", got)
	}

	feed(m, KeyEvent(KeyHome), KeyEvent(KeyBackspace))
	if got := m.Text(); got != "abcd" {
		t.Errorf("Text() = %q, want %q", got, "abcd")
	}
	if m.row != 0 || m.col != 2 {
		t.Errorf("cursor = (%d,%d), want (0,2) at the join", m.row, m.col)
	}
}

func TestEditorDeleteForwardPullsNextLineUp(t *testing.T) {
	m := NewEditor().IsComplete(func(string) bool { return false })
	feed(m, append(typeRunes("ab"), KeyEvent(KeyEnter))...)
	feed(m, typeRunes("cd")...)
	feed(m, KeyEvent(KeyUp), KeyEvent(KeyEnd), KeyEvent(KeyDelete))
	if got := m.Text(); got != "abcd" {
		t.Errorf("Text() = %q, want %q", got, "abcd")
	}
}

// Ctrl-D is end-of-input only on an empty buffer; with text it deletes forward.
// Ending the session because the cursor happened to sit at the end would be a
// nasty surprise.
func TestEditorCtrlDMeaningDependsOnBuffer(t *testing.T) {
	empty := NewEditor()
	if cmd := feed(empty, KeyEvent(KeyCtrlD)); cmd != CmdCancel {
		t.Fatalf("cmd = %v, want CmdCancel on empty buffer", cmd)
	}
	if empty.Outcome() != EditorEOF {
		t.Errorf("Outcome() = %v, want EditorEOF", empty.Outcome())
	}

	full := NewEditor()
	feed(full, append(typeRunes("abc"), KeyEvent(KeyHome))...)
	if cmd := feed(full, KeyEvent(KeyCtrlD)); cmd != CmdNone {
		t.Fatalf("cmd = %v, want CmdNone with text present", cmd)
	}
	if got := full.Text(); got != "bc" {
		t.Errorf("Text() = %q, want %q", got, "bc")
	}
}

func TestEditorCtrlCDiscards(t *testing.T) {
	m := NewEditor()
	feed(m, typeRunes("half-typed")...)
	if cmd := feed(m, KeyEvent(KeyCtrlC)); cmd != CmdCancel {
		t.Fatalf("cmd = %v, want CmdCancel", cmd)
	}
	if m.Outcome() != EditorDiscarded {
		t.Errorf("Outcome() = %v, want EditorDiscarded", m.Outcome())
	}
	if _, ok := m.Value(); ok {
		t.Error("Value() ok = true after discard, want false")
	}
}

func TestEditorKillAndWordDelete(t *testing.T) {
	killToEnd := NewEditor()
	feed(killToEnd, append(typeRunes("hello world"), KeyEvent(KeyHome))...)
	feed(killToEnd, KeyEvent(KeyRight), KeyEvent(KeyRight), KeyEvent(KeyRight), KeyEvent(KeyRight), KeyEvent(KeyRight))
	feed(killToEnd, KeyEvent(KeyCtrlK))
	if got := killToEnd.Text(); got != "hello" {
		t.Errorf("ctrl-k Text() = %q, want %q", got, "hello")
	}

	wordBack := NewEditor()
	feed(wordBack, typeRunes("hello world")...)
	feed(wordBack, KeyEvent(KeyCtrlW))
	if got := wordBack.Text(); got != "hello " {
		t.Errorf("ctrl-w Text() = %q, want %q", got, "hello ")
	}

	killLine := NewEditor()
	feed(killLine, typeRunes("throw away")...)
	feed(killLine, KeyEvent(KeyCtrlU))
	if got := killLine.Text(); got != "" {
		t.Errorf("ctrl-u Text() = %q, want empty", got)
	}
}

func TestEditorCtrlAAndCtrlEMoveToLineEnds(t *testing.T) {
	m := NewEditor()
	feed(m, typeRunes("abc")...)
	feed(m, KeyEvent(KeyCtrlA))
	if m.col != 0 {
		t.Errorf("after ctrl-a col = %d, want 0", m.col)
	}
	feed(m, KeyEvent(KeyCtrlE))
	if m.col != 3 {
		t.Errorf("after ctrl-e col = %d, want 3", m.col)
	}
}

// --- history ---

func TestEditorHistoryRecall(t *testing.T) {
	m := NewEditor().History([]string{"first", "second"})

	feed(m, KeyEvent(KeyUp))
	if got := m.Text(); got != "second" {
		t.Errorf("one up = %q, want %q", got, "second")
	}
	feed(m, KeyEvent(KeyUp))
	if got := m.Text(); got != "first" {
		t.Errorf("two up = %q, want %q", got, "first")
	}
	feed(m, KeyEvent(KeyUp)) // already at the oldest: stays put
	if got := m.Text(); got != "first" {
		t.Errorf("past oldest = %q, want %q", got, "first")
	}
	feed(m, KeyEvent(KeyDown))
	if got := m.Text(); got != "second" {
		t.Errorf("back down = %q, want %q", got, "second")
	}
}

// Walking into history must not lose what you were part-way through typing.
func TestEditorHistoryRestoresDraft(t *testing.T) {
	m := NewEditor().History([]string{"old"})
	feed(m, typeRunes("draft")...)

	feed(m, KeyEvent(KeyUp))
	if got := m.Text(); got != "old" {
		t.Fatalf("up = %q, want %q", got, "old")
	}
	feed(m, KeyEvent(KeyDown))
	if got := m.Text(); got != "draft" {
		t.Errorf("down = %q, want the stashed draft %q", got, "draft")
	}
}

// Within a multi-line buffer Up/Down move between lines; only at the edges do
// they reach for history.
func TestEditorUpMovesWithinBufferBeforeHistory(t *testing.T) {
	m := NewEditor().
		History([]string{"old"}).
		IsComplete(func(string) bool { return false })
	feed(m, append(typeRunes("one"), KeyEvent(KeyEnter))...)
	feed(m, typeRunes("two")...)

	feed(m, KeyEvent(KeyUp)) // row 1 -> row 0, no history
	if got := m.Text(); got != "one\ntwo" {
		t.Fatalf("Text() = %q, want the buffer untouched", got)
	}
	if m.row != 0 {
		t.Fatalf("row = %d, want 0", m.row)
	}
	feed(m, KeyEvent(KeyUp)) // now at the top: history
	if got := m.Text(); got != "old" {
		t.Errorf("Text() = %q, want %q", got, "old")
	}
}

// --- paste ---

// Inside a bracketed paste, Enter is content. Without this every pasted
// multi-line snippet submits on its first newline.
func TestEditorPasteInsertsNewlinesInsteadOfSubmitting(t *testing.T) {
	m := NewEditor() // no IsComplete: Enter would otherwise always submit
	evs := []Event{KeyEvent(KeyPasteStart)}
	evs = append(evs, typeRunes("a")...)
	evs = append(evs, KeyEvent(KeyEnter))
	evs = append(evs, typeRunes("b")...)
	evs = append(evs, KeyEvent(KeyPasteEnd))

	if cmd := feed(m, evs...); cmd != CmdNone {
		t.Fatalf("cmd = %v, want CmdNone during paste", cmd)
	}
	if got := m.Text(); got != "a\nb" {
		t.Errorf("Text() = %q, want %q", got, "a\nb")
	}
	// Paste over: Enter submits again.
	if cmd := feed(m, KeyEvent(KeyEnter)); cmd != CmdSubmit {
		t.Errorf("cmd after paste = %v, want CmdSubmit", cmd)
	}
}

// --- layout ---

func TestEditorLayoutCursorPosition(t *testing.T) {
	m := NewEditor().Prompt("> ")
	feed(m, typeRunes("abc")...)

	rows, row, col := m.layout()
	if len(rows) != 1 || rows[0] != "> abc" {
		t.Fatalf("rows = %q, want [\"> abc\"]", rows)
	}
	if row != 0 || col != 5 {
		t.Errorf("cursor = (%d,%d), want (0,5) after the prompt and text", row, col)
	}

	feed(m, KeyEvent(KeyHome))
	if _, row, col = m.layout(); row != 0 || col != 2 {
		t.Errorf("cursor at home = (%d,%d), want (0,2) just after the prompt", row, col)
	}
}

func TestEditorLayoutMultiLineUsesContinuationPrompt(t *testing.T) {
	m := NewEditor().
		Prompt("> ").
		ContPrompt(". ").
		IsComplete(func(string) bool { return false })
	feed(m, append(typeRunes("a"), KeyEvent(KeyEnter))...)
	feed(m, typeRunes("b")...)

	rows, row, col := m.layout()
	want := []string{"> a", ". b"}
	if len(rows) != 2 || rows[0] != want[0] || rows[1] != want[1] {
		t.Fatalf("rows = %q, want %q", rows, want)
	}
	if row != 1 || col != 3 {
		t.Errorf("cursor = (%d,%d), want (1,3)", row, col)
	}
}

// A logical line longer than the terminal wraps into several frame rows, each
// exactly one visual row, and the cursor follows it across the break.
func TestEditorLayoutWrapsLongLine(t *testing.T) {
	// width 10, prompt "> " (2), so 10-2-1 = 7 columns of text per row.
	m := NewEditor().Prompt("> ").Width(10)
	feed(m, typeRunes("abcdefghij")...) // 10 runes

	rows, row, col := m.layout()
	if len(rows) != 2 {
		t.Fatalf("rows = %q, want 2 rows", rows)
	}
	if rows[0] != "> abcdefg" {
		t.Errorf("rows[0] = %q, want %q", rows[0], "> abcdefg")
	}
	if rows[1] != "  hij" {
		t.Errorf("rows[1] = %q, want %q (hanging indent)", rows[1], "  hij")
	}
	if row != 1 || col != 5 {
		t.Errorf("cursor = (%d,%d), want (1,5) at the end of the wrapped tail", row, col)
	}
}

// No frame row may reach the full terminal width: a row filled to exactly the
// width triggers the terminal's own auto-wrap, which desyncs the inline
// renderer's redraw accounting.
func TestEditorLayoutRowsStayUnderWidth(t *testing.T) {
	const width = 12
	m := NewEditor().Prompt(">>> ").Width(width)
	feed(m, typeRunes(strings.Repeat("x", 100))...)

	rows, _, _ := m.layout()
	for i, r := range rows {
		if w := len([]rune(r)); w >= width {
			t.Errorf("row %d has width %d, want < %d: %q", i, w, width, r)
		}
	}
}

func TestEditorLayoutScrollsToKeepCursorVisible(t *testing.T) {
	m := NewEditor().
		Prompt("> ").
		MaxHeight(2).
		IsComplete(func(string) bool { return false })
	for _, s := range []string{"one", "two", "three"} {
		feed(m, typeRunes(s)...)
		feed(m, KeyEvent(KeyEnter))
	}

	// Buffer is "one", "two", "three", "" - four rows, windowed to the last two.
	rows, row, _ := m.layout()
	if len(rows) != 2 {
		t.Fatalf("rows = %q, want 2 (capped by MaxHeight)", rows)
	}
	if row != 1 {
		t.Errorf("cursor row = %d, want 1 (last visible row)", row)
	}
	if !strings.Contains(rows[0], "three") {
		t.Errorf("rows[0] = %q, want the window scrolled down to \"three\"", rows[0])
	}
}
