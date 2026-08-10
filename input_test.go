package radish

import (
	"errors"
	"strings"
	"testing"
)

func driveInput(t *testing.T, m *InputModel, events ...Event) (*ScriptDriver, Result, *InputModel) {
	t.Helper()
	d := NewScriptDriver(events)
	res, _, err := d.Run(m)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	return d, res, m
}

func typeRunes(s string) []Event {
	evs := make([]Event, 0, len(s))
	for _, r := range s {
		evs = append(evs, RuneEvent(r))
	}
	return evs
}

func TestInputValidateBlocksSubmitUntilValid(t *testing.T) {
	m := NewInput().Prompt("> ").Validate(func(s string) error {
		if s != "ok" {
			return errors.New("must be ok")
		}
		return nil
	})
	d, res, im := driveInput(t, m,
		RuneEvent('n'), RuneEvent('o'), KeyEvent(KeyEnter), // blocked
		KeyEvent(KeyBackspace), KeyEvent(KeyBackspace),
		RuneEvent('o'), RuneEvent('k'), KeyEvent(KeyEnter)) // accepted

	if res.Canceled {
		t.Fatalf("result = %+v, want submitted", res)
	}
	if got, _ := im.Value(); got != "ok" {
		t.Fatalf("Value() = %q, want \"ok\"", got)
	}
	// Frame after the blocked Enter shows the error line under the field.
	blocked := d.Frames()[3] // initial, n, o, blocked-enter, ...
	if blocked != "> no"+cursorGlyph+"\nmust be ok" {
		t.Errorf("blocked frame = %q, want field + error line", blocked)
	}
	// First edit after the failure clears the error.
	afterEdit := d.Frames()[4]
	if strings.Contains(afterEdit, "must be ok") {
		t.Errorf("error should clear on edit, got %q", afterEdit)
	}
}

func TestInputValidateCanRequireNonEmpty(t *testing.T) {
	m := NewInput().Prompt("> ").Validate(func(s string) error {
		if s == "" {
			return errors.New("value required")
		}
		return nil
	})
	d, res, im := driveInput(t, m, KeyEvent(KeyEnter), RuneEvent('x'), KeyEvent(KeyEnter))

	if res.Canceled {
		t.Fatalf("result = %+v, want submitted", res)
	}
	if got, _ := im.Value(); got != "x" {
		t.Fatalf("Value() = %q, want \"x\"", got)
	}
	if blocked := d.Frames()[1]; !strings.Contains(blocked, "value required") {
		t.Errorf("empty submit should render the error, got %q", blocked)
	}
}

func TestClampField(t *testing.T) {
	cases := []struct {
		name              string
		left, right       string
		budget            int
		wantLeft, wantRgt string
	}{
		{"both fit", "ab", "cd", 10, "ab", "cd"},
		{"left fits exactly, drop right", "abc", "d", 3, "abc", ""},
		{"left overflows, keep tail", "abcde", "f", 3, "…de", ""},
		{"left fits, right truncated", "ab", "cdef", 4, "ab", "c…"},
		{"exact total fit", "ab", "cd", 4, "ab", "cd"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotL, gotR := clampField(tc.left, tc.right, tc.budget)
			if gotL != tc.wantLeft || gotR != tc.wantRgt {
				t.Errorf("clampField(%q, %q, %d) = (%q, %q), want (%q, %q)",
					tc.left, tc.right, tc.budget, gotL, gotR, tc.wantLeft, tc.wantRgt)
			}
		})
	}
}

func TestKeepTail(t *testing.T) {
	cases := []struct {
		s    string
		w    int
		want string
	}{
		{"abcde", 2, "de"},
		{"abc", 0, ""},
		{"abc", 5, "abc"},
	}
	for _, tc := range cases {
		if got := keepTail(tc.s, tc.w); got != tc.want {
			t.Errorf("keepTail(%q, %d) = %q, want %q", tc.s, tc.w, got, tc.want)
		}
	}
}

func TestInputSummaryFunc(t *testing.T) {
	m := NewInput().Prompt("> ").SummaryFunc(func(v string) string {
		if v == "" {
			return "(skipped)"
		}
		return "got " + v
	})
	d, _, _ := driveInput(t, m, RuneEvent('x'), KeyEvent(KeyEnter))
	if last := lastFrame(d); last != "got x" {
		t.Errorf("summary = %q, want custom rendering", last)
	}

	m2 := NewInput().Prompt("> ").SummaryFunc(func(v string) string {
		if v == "" {
			return "(skipped)"
		}
		return "got " + v
	})
	d2, _, _ := driveInput(t, m2, KeyEvent(KeyEnter))
	if last := lastFrame(d2); last != "(skipped)" {
		t.Errorf("empty-value summary = %q, want skip marker", last)
	}
}

func TestInputSummaryFuncEmptyResultCollapsesToNothing(t *testing.T) {
	m := NewInput().Prompt("> ").SummaryFunc(func(string) string { return "" })
	d, _, _ := driveInput(t, m, RuneEvent('x'), KeyEvent(KeyEnter))
	// Finish("") records no frame, so the last frame is the pre-submit render.
	if last := lastFrame(d); last != "> x"+cursorGlyph {
		t.Errorf("last frame = %q, want no summary frame", last)
	}
}
