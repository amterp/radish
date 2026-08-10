package radish_test

import (
	"strings"
	"testing"

	snap "github.com/amterp/go-snap"
	"github.com/amterp/go-snap/prompt"

	"github.com/amterp/radish"
)

// multiSelectSuite snapshots the multi-choice picker. SELECTED holds one label
// per line, in option order rather than toggle order.
var multiSelectSuite = snap.Suite{
	Run: runMultiSelectCase,
	Inputs: []snap.Input{
		{Name: "PROMPT"},
		{Name: "OPTIONS", List: true, Quoted: true},
		{Name: "PRESELECT", List: true, Quoted: true},
		{Name: "HINT"},
		{Name: "HEIGHT"},
		{Name: "MIN"},
		{Name: "MAX"},
		prompt.KeysSection,
	},
	Outputs: []snap.Output{
		{Name: "SELECTED"},
		prompt.FramesSection,
	},
	Parallel: true,
}

func TestMultiSelectSnapshots(t *testing.T) {
	snap.Run(t, "snapshots/multiselect", &multiSelectSuite)
}

func runMultiSelectCase(t *testing.T, c *snap.Case) {
	m := radish.NewMultiSelect()
	// Preselect is applied before Options on purpose in some cases: the two are
	// order-independent, and a case says which order it means by section order
	// only if the runner honours one. This runner always preselects first, so a
	// case that needs the other order is a Go test, not a snapshot.
	if pre := c.List("PRESELECT"); len(pre) > 0 {
		m.Preselect(pre...)
	}
	m.Options(c.List("OPTIONS")...)
	if p := c.Text("PROMPT"); p != "" {
		m.Title(p)
	}
	if h := c.Text("HINT"); h != "" {
		m.Hint(h)
	}
	if h := c.Text("HEIGHT"); h != "" {
		m.Height(atoi(t, "HEIGHT", h))
	}
	if n := c.Text("MIN"); n != "" {
		m.Min(atoi(t, "MIN", n))
	}
	if n := c.Text("MAX"); n != "" {
		m.Max(atoi(t, "MAX", n))
	}

	d, res := driveSnap(t, c, m)
	c.Out("SELECTED", outcome(strings.Join(m.Selected(), "\n"), !res.Canceled))
	c.Out(prompt.FramesSection.Name, prompt.RenderFrames(d))
}
