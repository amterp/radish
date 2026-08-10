package radish

import (
	"strings"
	"testing"
)

func drive(t *testing.T, m *SelectModel, events ...Event) (*ScriptDriver, Result, *SelectModel) {
	t.Helper()
	d := NewScriptDriver(events)
	res, _, err := d.Run(m)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	// m is mutated in place and is the final model; no type assertion needed.
	return d, res, m
}

func lastFrame(d *ScriptDriver) string {
	f := d.Frames()
	return f[len(f)-1]
}

func TestSelectInjectedMatcherControlsOrder(t *testing.T) {
	// Custom matcher: prefix matches, ranking an exact hit first.
	exactFirst := func(filter, label string) (bool, int) {
		if filter == "" {
			return true, 1
		}
		if !strings.HasPrefix(label, filter) {
			return false, 0
		}
		if label == filter {
			return true, 0
		}
		return true, 1
	}
	m := NewSelect().Title("Pick").
		Options("golang", "go", "gopher").
		Matcher(exactFirst)
	// Type "go": all three have the prefix, but "go" is exact and should rank first.
	d, _, sm := drive(t, m, RuneEvent('g'), RuneEvent('o'), KeyEvent(KeyEnter))

	if got, _ := sm.Selected(); got != "go" {
		t.Fatalf("Selected() = %q, want \"go\" (exact ranked first by injected matcher)", got)
	}
	preSubmit := d.Frames()[len(d.Frames())-2]
	if !strings.Contains(preSubmit, "> go") {
		t.Errorf("cursor should rest on the top-ranked exact match:\n%s", preSubmit)
	}
}

func TestSelectViewLineCountIsDataDriven(t *testing.T) {
	// Title + 3 options, viewport big enough for all, no filter => 4 lines, no hints.
	m := NewSelect().Title("Pick").Options("a", "b", "c")
	got := strings.Count(m.View(), "\n") + 1
	if got != 4 {
		t.Errorf("line count = %d, want 4 (title + 3 options):\n%s", got, m.View())
	}
}

func TestSelectSummaryFunc(t *testing.T) {
	m := NewSelect().Title("Pick").Options("a", "b").
		SummaryFunc(func(sel string) string { return "--flag " + sel })
	d := NewScriptDriver([]Event{KeyEvent(KeyDown), KeyEvent(KeyEnter)})
	if _, _, err := d.Run(m); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if last := lastFrame(d); last != "--flag b" {
		t.Errorf("summary = %q, want custom rendering", last)
	}
}
