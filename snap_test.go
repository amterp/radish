// Package radish_test holds the snapshot suites for the three prompt models.
//
// They live in the external test package on purpose: go-snap's prompt helpers
// import radish, so an in-package test importing them would be an import cycle.
// Tests that need unexported access (the terminal driver's ANSI output,
// clampField, keepTail) stay in package radish, as do the ones whose prompt is
// configured by an injected closure - Matcher, Validate, SummaryFunc - since a
// text file cannot carry a function.
package radish_test

import (
	"strconv"
	"testing"

	snap "github.com/amterp/go-snap"
	"github.com/amterp/go-snap/prompt"

	"github.com/amterp/radish"
)

// canceled is what an outcome channel reports when the prompt was dismissed
// rather than submitted. The angle brackets keep it distinguishable from a real
// value; FRAMES carries the visual truth either way.
const canceled = "<canceled>"

// driveSnap replays the case's KEYS through the model, returning the driver so
// the runner can render its frames and the result so it can tell a submission
// from a cancel.
//
// A case with no KEYS is a test-authoring mistake rather than a scenario: with
// no script the prompt would block on a terminal that is not there, so it fails
// here instead of hanging.
func driveSnap(t *testing.T, c *snap.Case, m radish.Model) (*radish.ScriptDriver, radish.Result) {
	t.Helper()
	keys := c.List(prompt.KeysSection.Name)
	if len(keys) == 0 {
		t.Fatalf("case scripts no keystrokes: add a %s section", prompt.KeysSection.Name)
	}
	d, err := prompt.Driver(keys)
	if err != nil {
		t.Fatalf("%s: %v", prompt.KeysSection.Name, err)
	}
	res, _, err := d.Run(m)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	return d, res
}

// outcome renders a submitted value, or the cancel marker when there is none.
//
// The marker matters most where the model has no way to say so itself:
// MultiSelectModel.Selected keeps reporting the rows that were ticked after a
// cancel, so a channel that echoed it verbatim would record a choice the caller
// never received.
func outcome(value string, ok bool) string {
	if !ok {
		return canceled
	}
	return value
}

func atoi(t *testing.T, section, s string) int {
	t.Helper()
	n, err := strconv.Atoi(s)
	if err != nil {
		t.Fatalf("%s: %v", section, err)
	}
	return n
}
