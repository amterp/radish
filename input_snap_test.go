package radish_test

import (
	"testing"

	snap "github.com/amterp/go-snap"
	"github.com/amterp/go-snap/prompt"

	"github.com/amterp/radish"
)

// inputSuite snapshots the free-text field.
//
// The masked and secret cases are the ones that gain most from being snapshots:
// "the value never appears in any frame" is a claim a reviewer can check by
// reading FRAMES, rather than one they have to take on trust from an assertion.
var inputSuite = snap.Suite{
	Run: runInputCase,
	Inputs: []snap.Input{
		{Name: "PROMPT"},
		{Name: "PLACEHOLDER"},
		{Name: "ECHO"},
		{Name: "WIDTH"},
		prompt.KeysSection,
	},
	Outputs: []snap.Output{
		{Name: "VALUE"},
		prompt.FramesSection,
	},
	Parallel: true,
}

func TestInputSnapshots(t *testing.T) {
	snap.Run(t, "snapshots/input", &inputSuite)
}

func runInputCase(t *testing.T, c *snap.Case) {
	m := radish.NewInput()
	if p := c.Text("PROMPT"); p != "" {
		m.Prompt(p)
	}
	if p := c.Text("PLACEHOLDER"); p != "" {
		m.Placeholder(p)
	}
	if e := c.Text("ECHO"); e != "" {
		m.Echo(echoMode(t, e))
	}
	if w := c.Text("WIDTH"); w != "" {
		m.Width(atoi(t, "WIDTH", w))
	}

	d, res := driveSnap(t, c, m)
	value, ok := m.Value()
	ok = ok && !res.Canceled
	c.Out("VALUE", outcome(value, ok))
	c.Out(prompt.FramesSection.Name, prompt.RenderFrames(d))
}

func echoMode(t *testing.T, s string) radish.EchoMode {
	t.Helper()
	switch s {
	case "normal":
		return radish.EchoNormal
	case "none":
		return radish.EchoNone
	case "masked":
		return radish.EchoMasked
	}
	t.Fatalf("ECHO: unknown mode %q; want normal, none or masked", s)
	return radish.EchoNormal
}
