package radish

import (
	"strings"

	runewidth "github.com/mattn/go-runewidth"
)

// EchoMode controls how an InputModel renders the text being typed.
type EchoMode int

const (
	EchoNormal EchoMode = iota // show the typed text
	EchoNone                   // show nothing (sudo-style secret entry)
	EchoMasked                 // show one '•' per rune
)

const cursorGlyph = "█"

// InputModel is a single-line text prompt: an optional title heading, an inline
// prompt prefix, and an editable value with a visible cursor. Like the other
// components it is a pure Model - all state and rendering, no I/O - so it runs
// identically in production and under scripted test events.
//
// Build one with the chained setters (NewInput().Prompt(...)), then hand it to Run.
// After Run, read the text with Value.
//
// EchoNone renders nothing as the user types (the terminal convention for
// passwords): every keystroke produces a byte-identical frame and the typed runes
// never appear in any rendered frame, only in the returned Value. In EchoNone the
// cursor cannot be moved (append and backspace only), matching tools like sudo.
type InputModel struct {
	title       string
	prompt      string
	placeholder string
	echo        EchoMode
	theme       *Theme
	keymap      KeyMap
	width       int // 0 = no truncation

	value  []rune
	cursor int // index into value

	canceled bool
}

// NewInput returns an InputModel with default theme and keymap and normal echo.
func NewInput() *InputModel {
	return &InputModel{
		echo:   EchoNormal,
		theme:  DefaultTheme(),
		keymap: DefaultKeyMap(),
	}
}

// Title sets an optional heading line shown above the input field.
func (m *InputModel) Title(s string) *InputModel { m.title = s; return m }

// Prompt sets the inline prefix rendered immediately before the editable field
// (e.g. "> " or "Confirm? [Y/n] > ").
func (m *InputModel) Prompt(s string) *InputModel { m.prompt = s; return m }

// Placeholder sets ghost/hint text shown (dimmed) while the field is empty. It is
// only shown in EchoNormal mode.
func (m *InputModel) Placeholder(s string) *InputModel { m.placeholder = s; return m }

// Echo sets how typed text is displayed (EchoNormal, EchoNone, EchoMasked).
func (m *InputModel) Echo(mode EchoMode) *InputModel { m.echo = mode; return m }

// Theme overrides the styling. A nil argument is ignored.
func (m *InputModel) Theme(t *Theme) *InputModel {
	if t != nil {
		m.theme = t
	}
	return m
}

// KeyMap overrides the key bindings.
func (m *InputModel) KeyMap(k KeyMap) *InputModel { m.keymap = k; return m }

// Width caps the rendered field to this terminal width (with an ellipsis). Zero
// disables truncation.
func (m *InputModel) Width(n int) *InputModel {
	if n >= 0 {
		m.width = n
	}
	return m
}

// Value returns the typed text and true, or ("", false) if the prompt was canceled.
func (m *InputModel) Value() (string, bool) {
	if m.canceled {
		return "", false
	}
	return string(m.value), true
}

// Update advances the model in response to one event. It implements Model.
// Printable runes and Backspace are intrinsic edits; cursor movement is bindable
// but disabled entirely in EchoNone (append + backspace only).
func (m *InputModel) Update(e Event) (Model, Cmd) {
	switch {
	case m.keymap.matches(e, m.keymap.Cancel):
		m.canceled = true
		return m, CmdCancel
	case m.keymap.matches(e, m.keymap.Submit):
		return m, CmdSubmit
	case e.Type == KeyRune:
		m.insert(e.Rune)
	case e.Type == KeyBackspace:
		m.backspace()
	case m.echo == EchoNone:
		// In no-echo (secret) mode the cursor never moves: ignore the rest.
	case m.keymap.matches(e, m.keymap.Left):
		if m.cursor > 0 {
			m.cursor--
		}
	case m.keymap.matches(e, m.keymap.Right):
		if m.cursor < len(m.value) {
			m.cursor++
		}
	case m.keymap.matches(e, m.keymap.Home):
		m.cursor = 0
	case m.keymap.matches(e, m.keymap.End):
		m.cursor = len(m.value)
	}
	return m, CmdNone
}

func (m *InputModel) insert(r rune) {
	v := make([]rune, 0, len(m.value)+1)
	v = append(v, m.value[:m.cursor]...)
	v = append(v, r)
	v = append(v, m.value[m.cursor:]...)
	m.value = v
	m.cursor++
}

func (m *InputModel) backspace() {
	if m.cursor == 0 {
		return
	}
	m.value = append(m.value[:m.cursor-1], m.value[m.cursor:]...)
	m.cursor--
}

// View renders the title (if any) and the field line. No trailing newline.
func (m *InputModel) View() string {
	var lines []string
	if m.title != "" {
		lines = append(lines, styled(m.theme.Title, fitWidth(m.title, m.width, 0)))
	}
	lines = append(lines, m.fieldLine())
	return strings.Join(lines, "\n")
}

// fieldLine renders the prompt prefix, the (echo-dependent) value with a visible
// cursor, or the placeholder when empty. The cursor is an explicit glyph rather
// than the terminal's own cursor so it survives in color-stripped snapshot frames.
func (m *InputModel) fieldLine() string {
	prompt := styled(m.theme.Normal, m.prompt)
	cursor := styled(m.theme.Cursor, cursorGlyph)
	reserve := runewidth.StringWidth(m.prompt) + 1 // prompt + cursor column

	if len(m.value) == 0 && m.echo == EchoNormal && m.placeholder != "" {
		ph := fitWidth(m.placeholder, m.width, reserve)
		return prompt + cursor + styled(m.theme.Placeholder, ph)
	}

	left, right := m.displayHalves()
	if m.width > 0 {
		left, right = clampField(left, right, max(1, m.width-reserve))
	}
	return prompt + styled(m.theme.Normal, left) + cursor + styled(m.theme.Normal, right)
}

// displayHalves returns the plain text shown before and after the cursor, per the
// echo mode. EchoNone always renders empty halves, so the field never reveals the
// secret and every keystroke yields an identical frame.
func (m *InputModel) displayHalves() (left, right string) {
	switch m.echo {
	case EchoNone:
		return "", ""
	case EchoMasked:
		return strings.Repeat("•", m.cursor), strings.Repeat("•", len(m.value)-m.cursor)
	default:
		return string(m.value[:m.cursor]), string(m.value[m.cursor:])
	}
}

// Summary is the collapsed line shown after submit. It returns "" on cancel, and -
// crucially - never echoes a secret: EchoNone collapses to nothing and EchoMasked
// to dots. Implements Summarizer.
func (m *InputModel) Summary() string {
	if m.canceled || m.echo == EchoNone {
		return ""
	}
	prefix := m.prompt
	if prefix == "" {
		prefix = m.title
	}
	var shown string
	if m.echo == EchoMasked {
		shown = strings.Repeat("•", len(m.value))
	} else {
		shown = string(m.value)
	}
	return styled(m.theme.Normal, prefix) + styled(m.theme.Selected, shown)
}

// clampField keeps the cursor visible within budget columns: the right (post-cursor)
// side is trimmed first, then the left side is trimmed from its head (keeping the
// runes nearest the cursor) with a leading ellipsis. v1 has no horizontal scroll
// beyond this; the full value is always preserved in Value regardless of display.
func clampField(left, right string, budget int) (string, string) {
	lw := runewidth.StringWidth(left)
	rw := runewidth.StringWidth(right)
	if lw+rw <= budget {
		return left, right
	}
	if lw > budget {
		// Even the pre-cursor text overflows; keep its cursor-adjacent tail.
		return "…" + keepTail(left, budget-1), ""
	}
	if budget-lw <= 0 {
		// The pre-cursor text fills the budget exactly; drop the post-cursor text.
		return left, ""
	}
	return left, runewidth.Truncate(right, budget-lw, "…")
}

// keepTail returns the longest suffix of s that fits in w columns.
func keepTail(s string, w int) string {
	if w <= 0 {
		return ""
	}
	r := []rune(s)
	for i := range r {
		if runewidth.StringWidth(string(r[i:])) <= w {
			return string(r[i:])
		}
	}
	return ""
}
