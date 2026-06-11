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

func TestInputTyping(t *testing.T) {
	m := NewInput().Prompt("> ")
	d, res, im := driveInput(t, m, append(typeRunes("hello"), KeyEvent(KeyEnter))...)

	if res.Canceled {
		t.Fatalf("result = %+v, want submitted", res)
	}
	if got, ok := im.Value(); !ok || got != "hello" {
		t.Fatalf("Value() = %q, %v; want \"hello\", true", got, ok)
	}
	if init := d.Frames()[0]; init != "> "+cursorGlyph {
		t.Errorf("initial frame = %q, want prompt+cursor", init)
	}
	if summary := lastFrame(d); summary != "> hello" {
		t.Errorf("summary frame = %q, want %q", summary, "> hello")
	}
}

func TestInputCursorVisibleAndMidTextEdit(t *testing.T) {
	m := NewInput().Prompt("> ")
	// Type "ac", move Left (cursor between a and c), insert "b" -> "abc".
	d, _, im := driveInput(t, m,
		RuneEvent('a'), RuneEvent('c'), KeyEvent(KeyLeft), RuneEvent('b'), KeyEvent(KeyEnter))

	if got, _ := im.Value(); got != "abc" {
		t.Fatalf("Value() = %q, want \"abc\" after mid-text insert", got)
	}
	// Frame after Left: cursor sits before 'c'.
	afterLeft := d.Frames()[3] // initial, a, c, left, b, summary
	if afterLeft != "> a"+cursorGlyph+"c" {
		t.Errorf("after-Left frame = %q, want cursor before c", afterLeft)
	}
}

func TestInputBackspace(t *testing.T) {
	m := NewInput().Prompt("> ")
	_, _, im := driveInput(t, m,
		RuneEvent('a'), RuneEvent('b'), RuneEvent('c'), KeyEvent(KeyBackspace), KeyEvent(KeyEnter))
	if got, _ := im.Value(); got != "ab" {
		t.Fatalf("Value() = %q, want \"ab\" after backspace", got)
	}
}

func TestInputHomeEnd(t *testing.T) {
	m := NewInput().Prompt("> ")
	// Type "bc", Home, type "a" (-> "abc"), End, type "d" (-> "abcd").
	_, _, im := driveInput(t, m,
		RuneEvent('b'), RuneEvent('c'),
		KeyEvent(KeyHome), RuneEvent('a'),
		KeyEvent(KeyEnd), RuneEvent('d'),
		KeyEvent(KeyEnter))
	if got, _ := im.Value(); got != "abcd" {
		t.Fatalf("Value() = %q, want \"abcd\" after Home/End edits", got)
	}
}

func TestInputPlaceholderShownWhenEmpty(t *testing.T) {
	m := NewInput().Prompt("> ").Placeholder("name here")
	d, _, _ := driveInput(t, m, RuneEvent('x'), KeyEvent(KeyEnter))

	if init := d.Frames()[0]; !strings.Contains(init, "name here") {
		t.Errorf("empty field should show placeholder:\n%q", init)
	}
	if afterType := d.Frames()[1]; strings.Contains(afterType, "name here") {
		t.Errorf("placeholder should vanish once typing starts:\n%q", afterType)
	}
}

func TestInputEchoNoneIsSecretFreeAndDeterministic(t *testing.T) {
	const secret = "hunter2"
	m := NewInput().Prompt("Password > ").Echo(EchoNone)
	d, res, im := driveInput(t, m, append(typeRunes(secret), KeyEvent(KeyEnter))...)

	if res.Canceled {
		t.Fatalf("result = %+v, want submitted", res)
	}
	if got, _ := im.Value(); got != secret {
		t.Fatalf("Value() = %q, want %q (the real value still flows through)", got, secret)
	}
	frames := d.Frames()
	// No summary on EchoNone submit, so it's initial + one frame per keystroke.
	want := "Password > " + cursorGlyph
	for i, f := range frames {
		if f != want {
			t.Errorf("frame %d = %q, want every frame identical to %q", i, f, want)
		}
		if strings.Contains(f, secret) {
			t.Errorf("frame %d leaked the secret: %q", i, f)
		}
	}
}

func TestInputEchoNoneIgnoresCursorMovement(t *testing.T) {
	m := NewInput().Prompt("> ").Echo(EchoNone)
	// Left/Home in secret mode must be no-ops; backspace still deletes the last rune.
	_, _, im := driveInput(t, m,
		RuneEvent('a'), RuneEvent('b'), KeyEvent(KeyLeft), KeyEvent(KeyHome),
		KeyEvent(KeyBackspace), KeyEvent(KeyEnter))
	if got, _ := im.Value(); got != "a" {
		t.Fatalf("Value() = %q, want \"a\" (cursor frozen, backspace drops last rune)", got)
	}
}

func TestInputEchoMaskedShowsDots(t *testing.T) {
	m := NewInput().Prompt("> ").Echo(EchoMasked)
	d, _, im := driveInput(t, m, RuneEvent('a'), RuneEvent('b'), RuneEvent('c'), KeyEvent(KeyEnter))

	if got, _ := im.Value(); got != "abc" {
		t.Fatalf("Value() = %q, want \"abc\"", got)
	}
	preSubmit := d.Frames()[3] // initial, a, b, c, summary
	if preSubmit != "> •••"+cursorGlyph {
		t.Errorf("masked frame = %q, want three dots + cursor", preSubmit)
	}
	if strings.Contains(preSubmit, "abc") {
		t.Errorf("masked frame must not reveal the value:\n%q", preSubmit)
	}
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

func TestInputCancelLeavesNoSummary(t *testing.T) {
	m := NewInput().Prompt("> ")
	d, res, im := driveInput(t, m, RuneEvent('x'), KeyEvent(KeyCtrlC))

	if !res.Canceled {
		t.Fatalf("result = %+v, want canceled", res)
	}
	if got, ok := im.Value(); ok || got != "" {
		t.Errorf("Value() = %q, %v; want \"\", false after cancel", got, ok)
	}
	if last := lastFrame(d); last != "> x"+cursorGlyph {
		t.Errorf("last frame = %q, want the interactive render (no summary)", last)
	}
}

func TestInputMaskedSummaryDoesNotLeak(t *testing.T) {
	m := NewInput().Prompt("pw > ").Echo(EchoMasked)
	d, _, _ := driveInput(t, m, RuneEvent('s'), RuneEvent('e'), RuneEvent('c'), KeyEvent(KeyEnter))
	if summary := lastFrame(d); strings.Contains(summary, "sec") {
		t.Errorf("masked summary leaked the value: %q", summary)
	}
}

func TestInputWidthTruncation(t *testing.T) {
	// Narrow width forces the field to clamp; the full value still returns.
	m := NewInput().Prompt("> ").Width(6)
	_, _, im := driveInput(t, m, append(typeRunes("abcdefghij"), KeyEvent(KeyEnter))...)
	if got, _ := im.Value(); got != "abcdefghij" {
		t.Fatalf("Value() = %q, want full value despite narrow display", got)
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
