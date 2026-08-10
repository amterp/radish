package radish_test

import (
	"testing"

	snap "github.com/amterp/go-snap"
	"github.com/amterp/go-snap/prompt"

	"github.com/amterp/radish"
)

// selectSuite snapshots the single-choice picker: a prompt described as data,
// a scripted keystroke sequence, and every frame it rendered on the way.
//
// The prompt heading is PROMPT rather than TITLE because go-snap owns TITLE for
// the case's own name. It feeds SelectModel.Title.
var selectSuite = snap.Suite{
	Run: runSelectCase,
	Inputs: []snap.Input{
		{Name: "PROMPT"},
		{Name: "OPTIONS", List: true, Quoted: true},
		{Name: "HEIGHT"},
		prompt.KeysSection,
	},
	Outputs: []snap.Output{
		{Name: "SELECTED"},
		prompt.FramesSection,
	},
	Parallel: true,
}

func TestSelectSnapshots(t *testing.T) {
	snap.Run(t, "snapshots/select", &selectSuite)
}

func runSelectCase(t *testing.T, c *snap.Case) {
	m := radish.NewSelect().Options(c.List("OPTIONS")...)
	if p := c.Text("PROMPT"); p != "" {
		m.Title(p)
	}
	if h := c.Text("HEIGHT"); h != "" {
		m.Height(atoi(t, "HEIGHT", h))
	}

	d, res := driveSnap(t, c, m)
	selected, ok := m.Selected()
	ok = ok && !res.Canceled
	c.Out("SELECTED", outcome(selected, ok))
	c.Out(prompt.FramesSection.Name, prompt.RenderFrames(d))
}
