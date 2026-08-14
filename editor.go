package radish

import (
	"strings"

	runewidth "github.com/mattn/go-runewidth"
)

// EditorOutcome says how an edit ended. Result{Canceled} only carries one bit,
// which cannot distinguish "throw this line away and prompt me again" from "I am
// done giving you input" - a distinction a REPL loop lives or dies by.
type EditorOutcome int

// EditorEOF is deliberately the zero value: Run also ends on a source EOF, which
// sets no outcome at all, and "the input stream ended" is the truthful reading of
// that. Making Submitted the zero value would have an abandoned edit claim its
// half-typed buffer was submitted.
const (
	EditorEOF       EditorOutcome = iota // Ctrl-D on an empty buffer, or input ran out
	EditorSubmitted                      // Enter on a complete buffer
	EditorDiscarded                      // Ctrl-C: abandon the buffer, prompt again
)

// EditorModel is a multi-line text buffer with a real cursor: the input half of
// a REPL. It is a pure Model like the prompts, and deliberately much smaller than
// a readline - no completion, no highlighting, no reverse search.
//
// Two hooks keep it language-agnostic. IsComplete decides whether Enter submits
// or opens a new line, and Indent supplies the leading whitespace for that new
// line. Both default to single-line behavior, so an EditorModel with no hooks is
// just a text field that happens to know about the cursor.
//
// It implements Cursorer, so it gets the terminal's own cursor rather than the
// glyph the prompts draw - a glyph would shift the text as it moved, which is
// tolerable in a one-line field and not in a code buffer.
type EditorModel struct {
	prompt     string
	contPrompt string
	theme      *Theme
	keymap     KeyMap
	width      int
	maxHeight  int

	lines [][]rune // logical lines, no trailing newlines
	row   int      // cursor's logical line
	col   int      // cursor's rune index within lines[row]

	history   []string
	histIdx   int      // == len(history) when not browsing history
	histDraft [][]rune // the buffer stashed when browsing began

	isComplete func(string) bool
	indent     func(string) string

	pasting bool
	outcome EditorOutcome
}

// NewEditor returns an empty editor with the default theme and keymap.
func NewEditor() *EditorModel {
	return &EditorModel{
		theme:  DefaultTheme(),
		keymap: DefaultKeyMap(),
		lines:  [][]rune{{}},
	}
}

// Prompt sets the prefix rendered before the first line (e.g. "> ").
func (m *EditorModel) Prompt(s string) *EditorModel { m.prompt = s; return m }

// ContPrompt sets the prefix for every line after the first (e.g. "... ").
// Defaults to spaces matching the main prompt's width, so text stays aligned.
func (m *EditorModel) ContPrompt(s string) *EditorModel { m.contPrompt = s; return m }

// Theme overrides the styling. A nil argument is ignored.
func (m *EditorModel) Theme(t *Theme) *EditorModel {
	if t != nil {
		m.theme = t
	}
	return m
}

// KeyMap overrides the key bindings.
func (m *EditorModel) KeyMap(k KeyMap) *EditorModel { m.keymap = k; return m }

// Width sets the terminal width the buffer wraps to. Zero disables wrapping.
func (m *EditorModel) Width(n int) *EditorModel {
	if n >= 0 {
		m.width = n
	}
	return m
}

// MaxHeight caps how many rows the editor draws, scrolling within the buffer to
// keep the cursor visible. Zero means no cap. Set it to the terminal height: the
// inline renderer walks back up over the rows it drew, so a block taller than the
// screen scrolls part of itself out of reach and corrupts the next redraw.
func (m *EditorModel) MaxHeight(n int) *EditorModel {
	if n >= 0 {
		m.maxHeight = n
	}
	return m
}

// History supplies previously submitted entries, oldest first, for Up/Down
// recall. The editor never mutates the slice.
func (m *EditorModel) History(entries []string) *EditorModel {
	m.history = entries
	m.histIdx = len(entries)
	return m
}

// IsComplete decides what Enter does: submit when fn reports the buffer is a
// complete unit, otherwise open a new line. A nil fn (the default) always
// submits, giving single-line behavior.
func (m *EditorModel) IsComplete(fn func(string) bool) *EditorModel {
	m.isComplete = fn
	return m
}

// Indent supplies the leading whitespace for a newly opened line, given the
// buffer text above it. A nil fn (the default) starts new lines at column zero.
// It is not consulted while pasting, since pasted text carries its own indent.
func (m *EditorModel) Indent(fn func(string) string) *EditorModel {
	m.indent = fn
	return m
}

// Text returns the buffer contents, lines joined with newlines.
func (m *EditorModel) Text() string {
	parts := make([]string, len(m.lines))
	for i, l := range m.lines {
		parts[i] = string(l)
	}
	return strings.Join(parts, "\n")
}

// Value returns the buffer and true, or ("", false) if the edit did not end in a
// submit. It mirrors InputModel.Value so callers can treat the two alike.
func (m *EditorModel) Value() (string, bool) {
	if m.outcome != EditorSubmitted {
		return "", false
	}
	return m.Text(), true
}

// Outcome reports how the edit ended.
func (m *EditorModel) Outcome() EditorOutcome { return m.outcome }

// Update advances the model in response to one event. It implements Model.
func (m *EditorModel) Update(e Event) (Model, Cmd) {
	// Inside a bracketed paste every event is content: Enter is a newline, not a
	// submit, and chords are not commands. This is the whole reason paste markers
	// are decoded.
	if m.pasting {
		switch {
		case e.Type == KeyPasteEnd:
			m.pasting = false
		case e.Type == KeyEnter:
			m.openLine("")
		case e.Type == KeyRune:
			m.insert(e.Rune)
		case e.Type == KeyTab:
			m.insert(' ')
			m.insert(' ')
			m.insert(' ')
			m.insert(' ')
		}
		return m, CmdNone
	}

	switch {
	case e.Type == KeyPasteStart:
		m.pasting = true

	case m.keymap.matches(e, m.keymap.EndOfInput):
		// Ctrl-D only means end-of-input on an empty buffer; with text present it
		// is the readline "delete forward", which is far less startling than
		// ending the session because the cursor happened to be at the end.
		if m.isEmpty() {
			m.outcome = EditorEOF
			return m, CmdCancel
		}
		m.deleteForward()

	case m.keymap.matches(e, m.keymap.Discard):
		m.outcome = EditorDiscarded
		return m, CmdCancel

	case m.keymap.matches(e, m.keymap.Submit):
		if m.complete() {
			m.outcome = EditorSubmitted
			return m, CmdSubmit
		}
		m.openLine(m.indentFor())

	case e.Type == KeyRune:
		m.insert(e.Rune)
	case e.Type == KeyBackspace:
		m.backspace()
	case e.Type == KeyDelete:
		m.deleteForward()

	case m.keymap.matches(e, m.keymap.Up):
		m.moveUp()
	case m.keymap.matches(e, m.keymap.Down):
		m.moveDown()
	case m.keymap.matches(e, m.keymap.Left):
		m.moveLeft()
	case m.keymap.matches(e, m.keymap.Right):
		m.moveRight()
	case m.keymap.matches(e, m.keymap.Home):
		m.col = 0
	case m.keymap.matches(e, m.keymap.End):
		m.col = len(m.lines[m.row])

	case m.keymap.matches(e, m.keymap.KillLine):
		m.lines[m.row] = nil
		m.col = 0
	case m.keymap.matches(e, m.keymap.KillToEnd):
		m.lines[m.row] = m.lines[m.row][:m.col]
	case m.keymap.matches(e, m.keymap.DeleteWordBack):
		m.deleteWordBack()
	}
	return m, CmdNone
}

// complete asks the caller whether Enter should submit. A blank line always
// ends accumulation: an indentation-scoped language has no block terminator, and
// without this the editor could hold a buffer no keystroke can close.
func (m *EditorModel) complete() bool {
	if m.isComplete == nil {
		return true
	}
	if len(m.lines) > 1 && len(strings.TrimSpace(string(m.lines[len(m.lines)-1]))) == 0 {
		return true
	}
	return m.isComplete(m.Text())
}

func (m *EditorModel) indentFor() string {
	if m.indent == nil {
		return ""
	}
	return m.indent(m.Text())
}

func (m *EditorModel) isEmpty() bool {
	return len(m.lines) == 1 && len(m.lines[0]) == 0
}

// --- editing ---

func (m *EditorModel) insert(r rune) {
	line := m.lines[m.row]
	out := make([]rune, 0, len(line)+1)
	out = append(out, line[:m.col]...)
	out = append(out, r)
	out = append(out, line[m.col:]...)
	m.lines[m.row] = out
	m.col++
}

// openLine splits the current line at the cursor and starts the new one with the
// given indent, carrying any text right of the cursor along with it.
func (m *EditorModel) openLine(indent string) {
	line := m.lines[m.row]
	head := append([]rune{}, line[:m.col]...)
	tail := append([]rune(indent), line[m.col:]...)

	m.lines[m.row] = head
	rest := append([][]rune{tail}, m.lines[m.row+1:]...)
	m.lines = append(m.lines[:m.row+1], rest...)
	m.row++
	m.col = len([]rune(indent))
}

func (m *EditorModel) backspace() {
	if m.col > 0 {
		line := m.lines[m.row]
		m.lines[m.row] = append(line[:m.col-1], line[m.col:]...)
		m.col--
		return
	}
	if m.row == 0 {
		return
	}
	// At column zero: join this line onto the end of the previous one.
	prev := m.lines[m.row-1]
	m.col = len(prev)
	m.lines[m.row-1] = append(prev, m.lines[m.row]...)
	m.lines = append(m.lines[:m.row], m.lines[m.row+1:]...)
	m.row--
}

func (m *EditorModel) deleteForward() {
	line := m.lines[m.row]
	if m.col < len(line) {
		m.lines[m.row] = append(line[:m.col], line[m.col+1:]...)
		return
	}
	if m.row == len(m.lines)-1 {
		return
	}
	// At end of line: pull the next line up onto this one.
	m.lines[m.row] = append(line, m.lines[m.row+1]...)
	m.lines = append(m.lines[:m.row+1], m.lines[m.row+2:]...)
}

func (m *EditorModel) deleteWordBack() {
	line := m.lines[m.row]
	i := m.col
	for i > 0 && line[i-1] == ' ' {
		i--
	}
	for i > 0 && line[i-1] != ' ' {
		i--
	}
	m.lines[m.row] = append(line[:i], line[m.col:]...)
	m.col = i
}

// --- movement and history ---

func (m *EditorModel) moveLeft() {
	if m.col > 0 {
		m.col--
		return
	}
	if m.row > 0 {
		m.row--
		m.col = len(m.lines[m.row])
	}
}

func (m *EditorModel) moveRight() {
	if m.col < len(m.lines[m.row]) {
		m.col++
		return
	}
	if m.row < len(m.lines)-1 {
		m.row++
		m.col = 0
	}
}

// moveUp walks to the previous line, or recalls the previous history entry when
// already on the first line - the bash/zsh/python behavior, where history recall
// only kicks in once you cannot go up within the buffer.
func (m *EditorModel) moveUp() {
	if m.row > 0 {
		m.row--
		m.clampCol()
		return
	}
	m.historyBack()
}

func (m *EditorModel) moveDown() {
	if m.row < len(m.lines)-1 {
		m.row++
		m.clampCol()
		return
	}
	m.historyForward()
}

func (m *EditorModel) historyBack() {
	if m.histIdx == 0 || len(m.history) == 0 {
		return
	}
	if m.histIdx == len(m.history) {
		m.histDraft = m.snapshot() // stash what the user was typing
	}
	m.histIdx--
	m.load(splitLines(m.history[m.histIdx]))
}

func (m *EditorModel) historyForward() {
	if m.histIdx >= len(m.history) {
		return
	}
	m.histIdx++
	if m.histIdx == len(m.history) {
		m.load(m.histDraft) // walked back off the end: restore the stashed draft
		return
	}
	m.load(splitLines(m.history[m.histIdx]))
}

// load replaces the buffer and parks the cursor at the very end, which is where
// you want it after recalling an entry you are about to edit or re-run.
func (m *EditorModel) load(lines [][]rune) {
	if len(lines) == 0 {
		lines = [][]rune{{}}
	}
	m.lines = lines
	m.row = len(lines) - 1
	m.col = len(lines[m.row])
}

func (m *EditorModel) snapshot() [][]rune {
	out := make([][]rune, len(m.lines))
	for i, l := range m.lines {
		out[i] = append([]rune{}, l...)
	}
	return out
}

func (m *EditorModel) clampCol() {
	if n := len(m.lines[m.row]); m.col > n {
		m.col = n
	}
}

func splitLines(s string) [][]rune {
	parts := strings.Split(s, "\n")
	out := make([][]rune, len(parts))
	for i, p := range parts {
		out[i] = []rune(p)
	}
	return out
}

// --- rendering ---

// View renders the wrapped buffer. It implements Model.
func (m *EditorModel) View() string {
	rows, _, _ := m.layout()
	return strings.Join(rows, "\n")
}

// Cursor reports where the terminal cursor belongs. It implements Cursorer.
func (m *EditorModel) Cursor() (int, int) {
	_, row, col := m.layout()
	return row, col
}

// Summary leaves the submitted input in the scrollback, prompts and all, so the
// session reads as a transcript. Implements Summarizer.
func (m *EditorModel) Summary() string {
	if m.outcome == EditorEOF {
		return ""
	}
	rows, _, _ := m.layout()
	return strings.Join(rows, "\n")
}

// layout wraps the buffer into frame rows and locates the cursor among them.
// Both come out of one pass so they cannot disagree - a cursor computed
// separately from the text it points into drifts the moment wrapping changes.
//
// Rows are wrapped to width-1 rather than width: a row filled to exactly the
// terminal width triggers the terminal's own auto-wrap, putting the cursor on a
// row the inline renderer did not count and corrupting every redraw after it.
func (m *EditorModel) layout() (rows []string, curRow, curCol int) {
	curRow, curCol = -1, 0

	for i, line := range m.lines {
		prefix := m.contPrompt
		if i == 0 {
			prefix = m.prompt
		} else if prefix == "" {
			prefix = strings.Repeat(" ", runewidth.StringWidth(m.prompt))
		}
		prefixW := runewidth.StringWidth(prefix)
		hang := strings.Repeat(" ", prefixW)

		avail := m.width - prefixW - 1
		if m.width <= 0 {
			avail = 0 // no wrapping
		}

		chunks, offsets := wrapRunes(line, avail)
		for c, chunk := range chunks {
			p := prefix
			if c > 0 {
				p = hang
			}
			if i == m.row && curRow < 0 {
				// The cursor belongs on this chunk if it falls inside it, or if
				// this is the last chunk (cursor at end of line).
				start := offsets[c]
				end := start + len(chunk)
				if (m.col >= start && m.col < end) || c == len(chunks)-1 {
					curRow = len(rows)
					curCol = prefixW + runewidth.StringWidth(string(chunk[:clamp(m.col-start, 0, len(chunk))]))
				}
			}
			rows = append(rows, styled(m.theme.Normal, p+string(chunk)))
		}
	}

	if curRow < 0 {
		curRow = len(rows) - 1
	}
	return m.scrollTo(rows, curRow, curCol)
}

// scrollTo trims rows to MaxHeight, keeping the cursor's row in view and
// reporting its position within the trimmed window.
func (m *EditorModel) scrollTo(rows []string, curRow, curCol int) ([]string, int, int) {
	if m.maxHeight <= 0 || len(rows) <= m.maxHeight {
		return rows, curRow, curCol
	}
	start := curRow - m.maxHeight + 1 // keep the cursor on the last visible row
	start = clamp(start, 0, len(rows)-m.maxHeight)
	return rows[start : start+m.maxHeight], curRow - start, curCol
}

// wrapRunes splits a line into chunks of at most avail display columns,
// returning each chunk's starting rune index alongside it. avail <= 0 means no
// wrapping. A line always yields at least one (possibly empty) chunk, so an
// empty line still renders its prompt.
func wrapRunes(line []rune, avail int) (chunks [][]rune, offsets []int) {
	if avail <= 0 {
		return [][]rune{line}, []int{0}
	}
	start, w := 0, 0
	for i, r := range line {
		rw := runewidth.RuneWidth(r)
		if w+rw > avail {
			chunks = append(chunks, line[start:i])
			offsets = append(offsets, start)
			start, w = i, 0
		}
		w += rw
	}
	chunks = append(chunks, line[start:])
	offsets = append(offsets, start)
	return chunks, offsets
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
