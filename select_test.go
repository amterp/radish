package radish

import (
	"os"
	"strings"
	"testing"

	"github.com/amterp/color"
)

// Frames are asserted as plain text, so disable color for the whole package.
func TestMain(m *testing.M) {
	color.NoColor = true
	os.Exit(m.Run())
}

func drive(t *testing.T, m *SelectModel, events ...Event) (*ScriptDriver, Result, *SelectModel) {
	t.Helper()
	d := NewScriptDriver(events)
	res, final, err := d.Run(m)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	sm, ok := final.(*SelectModel)
	if !ok {
		t.Fatalf("final model type = %T, want *SelectModel", final)
	}
	return d, res, sm
}

func lastFrame(d *ScriptDriver) string {
	f := d.Frames()
	return f[len(f)-1]
}

func TestSelectBasicNavigation(t *testing.T) {
	m := NewSelect().Prompt("Pick a fruit").Options("Apple", "Banana", "Cherry")
	d, res, sm := drive(t, m, KeyEvent(KeyDown), KeyEvent(KeyEnter))

	if !res.Submitted || res.Canceled {
		t.Fatalf("result = %+v, want Submitted", res)
	}
	if got, ok := sm.Selected(); !ok || got != "Banana" {
		t.Fatalf("Selected() = %q, %v; want \"Banana\", true", got, ok)
	}

	frames := d.Frames()
	if len(frames) != 3 { // initial, after-down, summary
		t.Fatalf("len(frames) = %d, want 3: %q", len(frames), frames)
	}
	if !strings.Contains(frames[0], "> Apple") {
		t.Errorf("initial frame should mark Apple:\n%s", frames[0])
	}
	if !strings.Contains(frames[1], "> Banana") {
		t.Errorf("after-down frame should mark Banana:\n%s", frames[1])
	}
	if frames[2] != "Pick a fruit Banana" {
		t.Errorf("summary frame = %q, want %q", frames[2], "Pick a fruit Banana")
	}
}

func TestSelectLiveFilter(t *testing.T) {
	m := NewSelect().Prompt("Pick").Options("apple", "banana", "avocado")
	// Type "av": only "avocado" is a subsequence match.
	d, res, sm := drive(t, m, RuneEvent('a'), RuneEvent('v'), KeyEvent(KeyEnter))

	if !res.Submitted {
		t.Fatalf("result = %+v, want Submitted", res)
	}
	if got, _ := sm.Selected(); got != "avocado" {
		t.Fatalf("Selected() = %q, want \"avocado\"", got)
	}
	// The frame just before submit should show the filter line and the lone match.
	preSubmit := d.Frames()[len(d.Frames())-2]
	if !strings.Contains(preSubmit, "/av") {
		t.Errorf("filtered frame should show the filter line:\n%s", preSubmit)
	}
	if strings.Contains(preSubmit, "apple") || strings.Contains(preSubmit, "banana") {
		t.Errorf("filtered frame should hide non-matches:\n%s", preSubmit)
	}
}

func TestSelectFilterNoMatches(t *testing.T) {
	m := NewSelect().Prompt("Pick").Options("apple", "banana")
	d, res, sm := drive(t, m, RuneEvent('z'), KeyEvent(KeyEnter))

	// Enter with no matches is a no-op; EOF then cancels.
	if !res.Canceled {
		t.Fatalf("result = %+v, want Canceled (submit blocked on empty match set)", res)
	}
	if _, ok := sm.Selected(); ok {
		t.Errorf("Selected() should be false when nothing matched")
	}
	if !strings.Contains(lastFrame(d), "(no matches)") {
		t.Errorf("frame should show the no-matches hint:\n%s", lastFrame(d))
	}
}

func TestSelectScrollingViewport(t *testing.T) {
	m := NewSelect().Prompt("Pick").
		Options("o1", "o2", "o3", "o4", "o5", "o6").
		Height(3)
	d, res, sm := drive(t, m,
		KeyEvent(KeyDown), KeyEvent(KeyDown), KeyEvent(KeyDown), KeyEvent(KeyEnter))

	if got, _ := sm.Selected(); got != "o4" {
		t.Fatalf("Selected() = %q, want \"o4\"", got)
	}
	if !res.Submitted {
		t.Fatalf("result = %+v, want Submitted", res)
	}

	// Initial frame: top of list, only a down-hint.
	if init := d.Frames()[0]; !strings.Contains(init, "↓ 3 more") || strings.Contains(init, "↑") {
		t.Errorf("initial frame scroll hints wrong:\n%s", init)
	}
	// Frame after the third down: viewport scrolled, both hints present, cursor on o4.
	afterScroll := d.Frames()[3]
	for _, want := range []string{"↑ 1 more", "> o4", "↓ 2 more"} {
		if !strings.Contains(afterScroll, want) {
			t.Errorf("scrolled frame missing %q:\n%s", want, afterScroll)
		}
	}
	if strings.Contains(afterScroll, "o1") {
		t.Errorf("scrolled frame should have dropped o1 from the viewport:\n%s", afterScroll)
	}
}

func TestSelectCancelLeavesNoSummary(t *testing.T) {
	m := NewSelect().Prompt("Pick").Options("Apple", "Banana")
	d, res, sm := drive(t, m, KeyEvent(KeyDown), KeyEvent(KeyCtrlC))

	if !res.Canceled || res.Submitted {
		t.Fatalf("result = %+v, want Canceled", res)
	}
	if _, ok := sm.Selected(); ok {
		t.Errorf("Selected() should be false after cancel")
	}
	// No collapsed summary on cancel: last frame is the last interactive render.
	if !strings.Contains(lastFrame(d), "> Banana") {
		t.Errorf("last frame should be the interactive render, not a summary:\n%s", lastFrame(d))
	}
}

func TestSelectEmptyPromptNoLeadingBlankLine(t *testing.T) {
	m := NewSelect().Options("Apple", "Banana") // no prompt
	d, _, _ := drive(t, m, KeyEvent(KeyEnter))

	init := d.Frames()[0]
	if strings.HasPrefix(init, "\n") {
		t.Errorf("empty prompt must not produce a leading blank line:\n%q", init)
	}
	if !strings.HasPrefix(init, "> Apple") {
		t.Errorf("empty-prompt frame should start at the first option:\n%q", init)
	}
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
	m := NewSelect().Prompt("Pick").
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
	m := NewSelect().Prompt("Pick").Options("a", "b", "c")
	got := strings.Count(m.View(), "\n") + 1
	if got != 4 {
		t.Errorf("line count = %d, want 4 (title + 3 options):\n%s", got, m.View())
	}
}

func TestSelectHomeEnd(t *testing.T) {
	m := NewSelect().Prompt("Pick").Options("a", "b", "c", "d")
	// Move down twice, jump End (to d), then Home (back to a), submit.
	_, _, sm := drive(t, m,
		KeyEvent(KeyDown), KeyEvent(KeyDown), KeyEvent(KeyEnd), KeyEvent(KeyHome), KeyEvent(KeyEnter))
	if got, _ := sm.Selected(); got != "a" {
		t.Fatalf("Selected() = %q, want \"a\" after End then Home", got)
	}
}
