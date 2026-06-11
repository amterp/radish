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

func TestMultiSelectToggleSpaceAndTab(t *testing.T) {
	m := NewMultiSelect().Title("Pick").Options("a", "b", "c")
	// Space toggles "a"; Down to "b"; Tab toggles "b"; submit.
	_, res, mm := driveMulti(t, m, space, KeyEvent(KeyDown), KeyEvent(KeyTab), KeyEvent(KeyEnter))

	if res.Canceled {
		t.Fatalf("result = %+v, want submitted", res)
	}
	if got := mm.Selected(); !eqStrs(got, []string{"a", "b"}) {
		t.Fatalf("Selected() = %v, want [a b]", got)
	}
}

func TestMultiSelectNavigateToggleMultiple(t *testing.T) {
	m := NewMultiSelect().Title("Pick").Options("o1", "o2", "o3", "o4", "o5")
	_, _, mm := driveMulti(t, m,
		KeyEvent(KeyDown), space, // o2
		KeyEvent(KeyDown), KeyEvent(KeyDown), space, // o4
		KeyEvent(KeyEnter))
	if got := mm.Selected(); !eqStrs(got, []string{"o2", "o4"}) {
		t.Fatalf("Selected() = %v, want [o2 o4]", got)
	}
}

func TestMultiSelectFilterPreservesSelectionByOriginalIndex(t *testing.T) {
	m := NewMultiSelect().Title("Pick").Options("apple", "banana", "avocado", "cherry")
	// Select apple, filter to "av" (avocado only), select it, clear filter, submit.
	_, _, mm := driveMulti(t, m,
		space,                          // apple (index 0)
		RuneEvent('a'), RuneEvent('v'), // filter narrows to avocado
		space,              // avocado (index 2)
		KeyEvent(KeyCtrlU), // clear filter
		KeyEvent(KeyEnter))
	if got := mm.Selected(); !eqStrs(got, []string{"apple", "avocado"}) {
		t.Fatalf("Selected() = %v, want [apple avocado] (selection keyed by original index)", got)
	}
}

func TestMultiSelectMaxBlocksExtraToggle(t *testing.T) {
	m := NewMultiSelect().Title("Pick").Options("a", "b", "c").Max(2)
	// Select a, b, then try c (blocked at max 2).
	_, _, mm := driveMulti(t, m,
		space, KeyEvent(KeyDown), space, KeyEvent(KeyDown), space, KeyEvent(KeyEnter))
	if got := mm.Selected(); !eqStrs(got, []string{"a", "b"}) {
		t.Fatalf("Selected() = %v, want [a b] (third toggle blocked by Max)", got)
	}
}

func TestMultiSelectMinBlocksSubmit(t *testing.T) {
	m := NewMultiSelect().Title("Pick").Options("a", "b", "c").Min(2)
	// First Enter (1 selected) is a no-op; after selecting a second, Enter submits.
	d, res, mm := driveMulti(t, m,
		space,                    // a (1 selected)
		KeyEvent(KeyEnter),       // blocked: under min
		KeyEvent(KeyDown), space, // b (2 selected)
		KeyEvent(KeyEnter)) // submits
	if res.Canceled {
		t.Fatalf("result = %+v, want submitted after reaching min", res)
	}
	if got := mm.Selected(); !eqStrs(got, []string{"a", "b"}) {
		t.Fatalf("Selected() = %v, want [a b] (first Enter must not have submitted [a])", got)
	}
	// The frame after selecting just one shows how many more are needed.
	afterOne := d.Frames()[1]
	if !strings.Contains(afterOne, "select 1 more") {
		t.Errorf("under-min frame should show the remaining-count hint:\n%s", afterOne)
	}
}

func TestMultiSelectMaxReachedHint(t *testing.T) {
	m := NewMultiSelect().Title("Pick").Options("a", "b", "c").Max(1)
	d, _, _ := driveMulti(t, m, space, KeyEvent(KeyEnter))
	// Once at the max, a hint explains why further toggles are blocked.
	if afterMax := d.Frames()[1]; !strings.Contains(afterMax, "max 1 selected") {
		t.Errorf("at-max frame should show the max hint:\n%s", afterMax)
	}
}

func TestMultiSelectSubmitWhileFilteredToEmpty(t *testing.T) {
	m := NewMultiSelect().Title("Pick").Options("apple", "banana")
	// Select apple, filter to no matches, then Enter: the accumulated selection
	// submits regardless of the (empty) filter view.
	_, res, mm := driveMulti(t, m, space, RuneEvent('z'), KeyEvent(KeyEnter))
	if res.Canceled {
		t.Fatalf("result = %+v, want submitted (selection is independent of filter)", res)
	}
	if got := mm.Selected(); !eqStrs(got, []string{"apple"}) {
		t.Fatalf("Selected() = %v, want [apple]", got)
	}
}

func TestMultiSelectSelectedOrderIndependentOfToggleOrder(t *testing.T) {
	m := NewMultiSelect().Title("Pick").Options("a", "b", "c")
	// Toggle c first, then a; Selected must come back in option order [a c].
	_, _, mm := driveMulti(t, m,
		KeyEvent(KeyDown), KeyEvent(KeyDown), space, // c
		KeyEvent(KeyHome), space, // a
		KeyEvent(KeyEnter))
	if got := mm.Selected(); !eqStrs(got, []string{"a", "c"}) {
		t.Fatalf("Selected() = %v, want [a c] in option order", got)
	}
}

func TestMultiSelectCheckboxRendering(t *testing.T) {
	m := NewMultiSelect().Title("Pick").Options("a", "b")
	d, _, _ := driveMulti(t, m, space, KeyEvent(KeyEnter))
	// After selecting the cursor row, its checkbox is marked.
	afterSelect := d.Frames()[1]
	if !strings.Contains(afterSelect, "> [x] a") {
		t.Errorf("selected cursor row should show a checked box:\n%s", afterSelect)
	}
	if !strings.Contains(afterSelect, "[ ] b") {
		t.Errorf("unselected row should show an empty box:\n%s", afterSelect)
	}
}

func TestMultiSelectCancel(t *testing.T) {
	m := NewMultiSelect().Title("Pick").Options("a", "b")
	d, res, _ := driveMulti(t, m, space, KeyEvent(KeyCtrlC))
	if !res.Canceled {
		t.Fatalf("result = %+v, want canceled", res)
	}
	// No collapsed summary on cancel: last frame is the interactive render.
	if !strings.Contains(lastFrame(d), "[x] a") {
		t.Errorf("last frame should be the interactive render, not a summary:\n%s", lastFrame(d))
	}
}

func TestMultiSelectPreselect(t *testing.T) {
	m := NewMultiSelect().Title("Pick").Options("a", "b", "c").Preselect("b", "nope")
	// "b" opens checked ("nope" is ignored); untoggle it via Down+Space, check "a".
	d, _, mm := driveMulti(t, m, KeyEvent(KeyDown), space, space, KeyEvent(KeyEnter))

	// After Down+Space: b unchecked. Second Space re-checks b... so walk it:
	// initial: [ ]a [x]b [ ]c; Down -> cursor on b; Space -> b off; Space -> b on.
	if got := mm.Selected(); !eqStrs(got, []string{"b"}) {
		t.Fatalf("Selected() = %v, want [b]", got)
	}
	if init := d.Frames()[0]; !strings.Contains(init, "[x] b") || !strings.Contains(init, "[ ] a") {
		t.Errorf("initial frame should show b preselected:\n%s", init)
	}
}

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

func TestMultiSelectHint(t *testing.T) {
	m := NewMultiSelect().Title("Pick").Options("a", "b").Hint("space to toggle, enter to confirm")
	d, _, _ := driveMulti(t, m, KeyEvent(KeyEnter))

	if init := d.Frames()[0]; !strings.Contains(init, "space to toggle, enter to confirm") {
		t.Errorf("initial frame should render the hint footer:\n%s", init)
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
