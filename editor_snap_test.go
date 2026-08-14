package radish_test

import (
	"strings"
	"testing"

	snap "github.com/amterp/go-snap"
	"github.com/amterp/go-snap/prompt"

	"github.com/amterp/radish"
)

// editorSuite snapshots the multi-line editor.
//
// FRAMES earns its keep more here than anywhere else in radish: the editor is
// the only model whose cursor is the terminal's own rather than a drawn glyph,
// and ScriptDriver marks its position with a caret so a reviewer can still see
// where it sat. Wrapping, auto-indent and history recall are all "did the right
// thing appear on the right row" claims that a frame sequence answers and an
// assertion does not.
//
// IsComplete cannot come from a text file, so cases pick one of a few named
// stand-ins via the POLICY section. The real Rad predicate is tested in rad.
var editorSuite = snap.Suite{
	Run: runEditorCase,
	Inputs: []snap.Input{
		{Name: "PROMPT"},
		{Name: "CONT_PROMPT"},
		{Name: "WIDTH"},
		{Name: "MAX_HEIGHT"},
		{Name: "HISTORY", List: true},
		{Name: "POLICY"},
		prompt.KeysSection,
	},
	Outputs: []snap.Output{
		{Name: "VALUE"},
		{Name: "OUTCOME"},
		prompt.FramesSection,
	},
	Parallel: true,
}

func TestEditorSnapshots(t *testing.T) {
	snap.Run(t, "snapshots/editor", &editorSuite)
}

func runEditorCase(t *testing.T, c *snap.Case) {
	m := radish.NewEditor()
	if p := c.Text("PROMPT"); p != "" {
		m.Prompt(p)
	}
	if p := c.Text("CONT_PROMPT"); p != "" {
		m.ContPrompt(p)
	}
	if w := c.Text("WIDTH"); w != "" {
		m.Width(atoi(t, "WIDTH", w))
	}
	if h := c.Text("MAX_HEIGHT"); h != "" {
		m.MaxHeight(atoi(t, "MAX_HEIGHT", h))
	}
	if h := c.List("HISTORY"); len(h) > 0 {
		m.History(h)
	}
	if p := c.Text("POLICY"); p != "" {
		m.IsComplete(editorPolicy(t, p))
		m.Indent(blockIndent)
	}

	d, _ := driveSnap(t, c, m)
	value, ok := m.Value()
	c.Out("VALUE", outcome(value, ok))
	c.Out("OUTCOME", editorOutcomeName(m.Outcome()))
	c.Out(prompt.FramesSection.Name, prompt.RenderFrames(d))
}

// editorPolicy returns a named stand-in for the caller's IsComplete hook.
func editorPolicy(t *testing.T, s string) func(string) bool {
	t.Helper()
	switch s {
	case "block":
		// A crude echo of Rad's real rule: once a line ends with ':' the buffer
		// stays open. Only the editor's own blank-line rule closes it.
		return func(buf string) bool {
			for _, line := range strings.Split(buf, "\n") {
				if strings.HasSuffix(strings.TrimSpace(line), ":") {
					return false
				}
			}
			return true
		}
	case "never":
		return func(string) bool { return false }
	}
	t.Fatalf("POLICY: unknown policy %q; want block or never", s)
	return nil
}

// blockIndent adds one level after a line ending in ':', otherwise carries the
// current indent - the shape of Rad's auto-indent, without Rad's grammar.
func blockIndent(buf string) string {
	lines := strings.Split(buf, "\n")
	last := lines[len(lines)-1]
	indent := last[:len(last)-len(strings.TrimLeft(last, " "))]
	if strings.HasSuffix(strings.TrimSpace(last), ":") {
		indent += "    "
	}
	return indent
}

func editorOutcomeName(o radish.EditorOutcome) string {
	switch o {
	case radish.EditorSubmitted:
		return "submitted"
	case radish.EditorDiscarded:
		return "discarded"
	case radish.EditorEOF:
		return "eof"
	}
	return "unknown"
}
