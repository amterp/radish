package radish

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name         string
		in           []byte
		wantEvents   []Event
		wantConsumed int
	}{
		{"empty", []byte{}, nil, 0},

		// Control keys
		{"enter CR", []byte{'\r'}, []Event{KeyEvent(KeyEnter)}, 1},
		{"enter LF", []byte{'\n'}, []Event{KeyEvent(KeyEnter)}, 1},
		{"backspace DEL", []byte{0x7f}, []Event{KeyEvent(KeyBackspace)}, 1},
		{"backspace BS", []byte{0x08}, []Event{KeyEvent(KeyBackspace)}, 1},
		{"tab", []byte{0x09}, []Event{KeyEvent(KeyTab)}, 1},
		{"ctrl-c", []byte{0x03}, []Event{KeyEvent(KeyCtrlC)}, 1},
		{"ctrl-d", []byte{0x04}, []Event{KeyEvent(KeyCtrlD)}, 1},
		{"unknown control dropped", []byte{0x01}, nil, 1},
		{"ctrl-z dropped", []byte{0x1a}, nil, 1},

		// Printable / UTF-8
		{"printable ascii", []byte("a"), []Event{RuneEvent('a')}, 1},
		{"space is a rune", []byte(" "), []Event{RuneEvent(' ')}, 1},
		{"typed word", []byte("ham"), []Event{RuneEvent('h'), RuneEvent('a'), RuneEvent('m')}, 3},
		{"utf8 two-byte", []byte("é"), []Event{RuneEvent('é')}, 2},
		{"utf8 emoji four-byte", []byte("🍔"), []Event{RuneEvent('🍔')}, 4},
		{"invalid utf8 byte dropped", []byte{0x80}, nil, 1},

		// CSI cursor keys
		{"up CSI", []byte("\x1b[A"), []Event{KeyEvent(KeyUp)}, 3},
		{"down CSI", []byte("\x1b[B"), []Event{KeyEvent(KeyDown)}, 3},
		{"right CSI", []byte("\x1b[C"), []Event{KeyEvent(KeyRight)}, 3},
		{"left CSI", []byte("\x1b[D"), []Event{KeyEvent(KeyLeft)}, 3},
		{"home CSI letter", []byte("\x1b[H"), []Event{KeyEvent(KeyHome)}, 3},
		{"end CSI letter", []byte("\x1b[F"), []Event{KeyEvent(KeyEnd)}, 3},
		{"shift-tab CSI", []byte("\x1b[Z"), []Event{KeyEvent(KeyShiftTab)}, 3},

		// CSI tilde forms
		{"home tilde 1", []byte("\x1b[1~"), []Event{KeyEvent(KeyHome)}, 4},
		{"home tilde 7", []byte("\x1b[7~"), []Event{KeyEvent(KeyHome)}, 4},
		{"end tilde 4", []byte("\x1b[4~"), []Event{KeyEvent(KeyEnd)}, 4},
		{"end tilde 8", []byte("\x1b[8~"), []Event{KeyEvent(KeyEnd)}, 4},
		{"delete tilde 3 dropped", []byte("\x1b[3~"), nil, 4},

		// Modified arrow (Ctrl+Up): modifier ignored, still navigates
		{"ctrl+up modified -> up", []byte("\x1b[1;5A"), []Event{KeyEvent(KeyUp)}, 6},

		// SS3 (application cursor mode)
		{"up SS3", []byte("\x1bOA"), []Event{KeyEvent(KeyUp)}, 3},
		{"down SS3", []byte("\x1bOB"), []Event{KeyEvent(KeyDown)}, 3},
		{"home SS3", []byte("\x1bOH"), []Event{KeyEvent(KeyHome)}, 3},

		// Lone ESC
		{"lone esc", []byte{0x1b}, []Event{KeyEvent(KeyEsc)}, 1},
		{"esc then plain byte", []byte("\x1bx"), []Event{KeyEvent(KeyEsc), RuneEvent('x')}, 2},

		// Mixed bursts (paste / fast typing)
		{
			"mixed burst",
			[]byte("ab\x1b[Bc\r"),
			[]Event{RuneEvent('a'), RuneEvent('b'), KeyEvent(KeyDown), RuneEvent('c'), KeyEvent(KeyEnter)},
			7,
		},
		{"down then enter", []byte("\x1b[B\r"), []Event{KeyEvent(KeyDown), KeyEvent(KeyEnter)}, 4},
		{"filter text with space", []byte("new y"), []Event{
			RuneEvent('n'), RuneEvent('e'), RuneEvent('w'), RuneEvent(' '), RuneEvent('y'),
		}, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotEvents, gotConsumed := Parse(tt.in)
			if gotConsumed != tt.wantConsumed {
				t.Errorf("consumed = %d, want %d", gotConsumed, tt.wantConsumed)
			}
			if !reflect.DeepEqual(gotEvents, tt.wantEvents) {
				t.Errorf("events = %v, want %v", gotEvents, tt.wantEvents)
			}
		})
	}
}

// A multi-byte escape sequence split across two reads must not be mis-parsed: the
// partial tail is retained (consumed stops before it) and resolves once the rest
// of the bytes arrive.
func TestParseSplitEscapeResumes(t *testing.T) {
	// First read: a rune, then the start of a down-arrow.
	buf := []byte("a\x1b[")
	evs, n := Parse(buf)
	if want := []Event{RuneEvent('a')}; !reflect.DeepEqual(evs, want) {
		t.Fatalf("first read events = %v, want %v", evs, want)
	}
	if n != 1 {
		t.Fatalf("first read consumed = %d, want 1 (partial CSI retained)", n)
	}

	// Caller keeps the unconsumed tail and appends the next read's bytes.
	rest := append([]byte{}, buf[n:]...) // "\x1b["
	rest = append(rest, 'B')             // "\x1b[B"
	evs2, n2 := Parse(rest)
	if want := []Event{KeyEvent(KeyDown)}; !reflect.DeepEqual(evs2, want) {
		t.Fatalf("resumed events = %v, want %v", evs2, want)
	}
	if n2 != 3 {
		t.Fatalf("resumed consumed = %d, want 3", n2)
	}
}

// A multi-byte UTF-8 rune split across reads is likewise retained until complete.
func TestParseSplitUTF8Resumes(t *testing.T) {
	full := []byte("🍔") // 4 bytes
	partial := full[:2]

	evs, n := Parse(partial)
	if len(evs) != 0 {
		t.Fatalf("partial rune produced events %v, want none", evs)
	}
	if n != 0 {
		t.Fatalf("partial rune consumed = %d, want 0", n)
	}

	evs2, n2 := Parse(full)
	if want := []Event{RuneEvent('🍔')}; !reflect.DeepEqual(evs2, want) {
		t.Fatalf("full rune events = %v, want %v", evs2, want)
	}
	if n2 != 4 {
		t.Fatalf("full rune consumed = %d, want 4", n2)
	}
}

func TestEventString(t *testing.T) {
	cases := []struct {
		ev   Event
		want string
	}{
		{KeyEvent(KeyUp), "up"},
		{KeyEvent(KeyDown), "down"},
		{KeyEvent(KeyEnter), "enter"},
		{KeyEvent(KeyBackspace), "backspace"},
		{KeyEvent(KeyEsc), "esc"},
		{KeyEvent(KeyCtrlC), "ctrl-c"},
		{KeyEvent(KeyShiftTab), "shift-tab"},
		{RuneEvent('a'), "a"},
		{RuneEvent('🍔'), "🍔"},
		{RuneEvent(' '), "space"},
	}
	for _, c := range cases {
		if got := c.ev.String(); got != c.want {
			t.Errorf("Event{%v}.String() = %q, want %q", c.ev, got, c.want)
		}
	}
}
