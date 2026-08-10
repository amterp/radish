package radish

import (
	"strings"
	"testing"
)

func driveMulti(t *testing.T, m *MultiSelectModel, events ...Event) (*ScriptDriver, Result, *MultiSelectModel) {
	t.Helper()
	d := NewScriptDriver(events)
	res, _, err := d.Run(m)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	return d, res, m
}

func eqStrs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

var space = RuneEvent(' ')

func TestMultiSelectPreselectBeforeOptions(t *testing.T) {
	// Preselect is order-independent with Options: labels are remembered and
	// applied when the options arrive.
	m := NewMultiSelect().Title("Pick").Preselect("b").Options("a", "b", "c")
	d, _, mm := driveMulti(t, m, KeyEvent(KeyEnter))

	if got := mm.Selected(); !eqStrs(got, []string{"b"}) {
		t.Fatalf("Selected() = %v, want [b]", got)
	}
	if init := d.Frames()[0]; !strings.Contains(init, "[x] b") {
		t.Errorf("initial frame should show b preselected:\n%s", init)
	}
}

func TestMultiSelectSummaryFunc(t *testing.T) {
	m := NewMultiSelect().Options("a", "b").
		SummaryFunc(func(sel []string) string { return "chose: " + strings.Join(sel, "+") })
	d, _, _ := driveMulti(t, m, space, KeyEvent(KeyDown), space, KeyEvent(KeyEnter))

	if last := lastFrame(d); last != "chose: a+b" {
		t.Errorf("summary = %q, want custom rendering", last)
	}
}
