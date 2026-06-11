package radish

import "testing"

// One ScriptDriver feeding two prompts in sequence: events are consumed across
// Runs (not replayed), and FrameEventIdx maps every recorded frame back to its
// triggering event, with -1 marking each prompt's initial render.
func TestScriptDriverSequentialPrompts(t *testing.T) {
	d := NewScriptDriver([]Event{
		RuneEvent('a'), KeyEvent(KeyEnter),
		RuneEvent('b'), KeyEvent(KeyEnter),
	})

	m1 := NewInput().Prompt("1> ")
	if res, _, err := d.Run(m1); err != nil || res.Canceled {
		t.Fatalf("first Run = (%+v, %v), want clean submit", res, err)
	}
	m2 := NewInput().Prompt("2> ")
	if res, _, err := d.Run(m2); err != nil || res.Canceled {
		t.Fatalf("second Run = (%+v, %v), want clean submit", res, err)
	}

	if got, _ := m1.Value(); got != "a" {
		t.Errorf("first prompt Value() = %q, want \"a\"", got)
	}
	if got, _ := m2.Value(); got != "b" {
		t.Errorf("second prompt Value() = %q, want \"b\" (events consumed, not replayed)", got)
	}

	// Per prompt: initial render, render after the rune, summary after Enter.
	wantIdx := []int{-1, 0, 1, -1, 2, 3}
	gotIdx := d.FrameEventIdx()
	if len(gotIdx) != len(wantIdx) {
		t.Fatalf("FrameEventIdx() = %v, want %v", gotIdx, wantIdx)
	}
	for i := range wantIdx {
		if gotIdx[i] != wantIdx[i] {
			t.Fatalf("FrameEventIdx() = %v, want %v", gotIdx, wantIdx)
		}
	}
	if frames := d.Frames(); len(frames) != 6 || frames[3] != "2> "+cursorGlyph {
		t.Errorf("frames across runs = %q, want second prompt's initial at index 3", frames)
	}
}

// A canceled prompt collapses without a summary frame; the next prompt's
// initial render must still be labelled -1, not blamed on the cancel event.
func TestScriptDriverCancelThenNextPrompt(t *testing.T) {
	d := NewScriptDriver([]Event{
		KeyEvent(KeyCtrlC),
		RuneEvent('x'), KeyEvent(KeyEnter),
	})

	if res, _, err := d.Run(NewInput().Prompt("1> ")); err != nil || !res.Canceled {
		t.Fatalf("first Run = (%+v, %v), want canceled", res, err)
	}
	m2 := NewInput().Prompt("2> ")
	if res, _, err := d.Run(m2); err != nil || res.Canceled {
		t.Fatalf("second Run = (%+v, %v), want clean submit", res, err)
	}
	if got, _ := m2.Value(); got != "x" {
		t.Errorf("second prompt Value() = %q, want \"x\"", got)
	}

	// Frames: prompt1 initial; prompt2 initial, 'x', summary. No cancel summary.
	wantIdx := []int{-1, -1, 1, 2}
	gotIdx := d.FrameEventIdx()
	if len(gotIdx) != len(wantIdx) {
		t.Fatalf("FrameEventIdx() = %v, want %v", gotIdx, wantIdx)
	}
	for i := range wantIdx {
		if gotIdx[i] != wantIdx[i] {
			t.Fatalf("FrameEventIdx() = %v, want %v", gotIdx, wantIdx)
		}
	}
}
