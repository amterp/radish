package radish

import "unicode/utf8"

// Parse converts raw terminal input bytes into high-level Events.
//
// It returns the parsed events and the number of leading bytes consumed. A
// trailing *incomplete* escape sequence or partial UTF-8 rune is left unconsumed
// (consumed < len(b)), so a streaming caller can keep b[consumed:], append the
// next read, and call Parse again. Bytes that decode to nothing meaningful (e.g.
// unmapped control characters) are consumed but produce no event.
//
// Parse is pure: no I/O, no state. It is the entire cross-platform input surface,
// which is why it is unit-tested exhaustively in isolation.
func Parse(b []byte) (events []Event, consumed int) {
	i := 0
	for i < len(b) {
		ev, n, ok := parseOne(b[i:])
		if !ok {
			break // incomplete sequence at the tail; leave b[i:] for the caller
		}
		i += n
		if ev.Type != KeyNone {
			events = append(events, ev)
		}
	}
	return events, i
}

// parseOne decodes a single Event from the front of b (which is non-empty).
// ok is false when b holds only the start of a longer sequence and more bytes are
// needed; in that case n is 0 and nothing should be consumed.
func parseOne(b []byte) (ev Event, n int, ok bool) {
	c := b[0]
	switch {
	case c == 0x1b: // ESC - possibly the start of a CSI/SS3 sequence
		return parseEsc(b)
	case c == '\r' || c == '\n':
		return KeyEvent(KeyEnter), 1, true
	case c == 0x7f || c == 0x08: // DEL, BS
		return KeyEvent(KeyBackspace), 1, true
	case c == 0x09: // Tab
		return KeyEvent(KeyTab), 1, true
	case c == 0x03: // Ctrl-C
		return KeyEvent(KeyCtrlC), 1, true
	case c == 0x04: // Ctrl-D
		return KeyEvent(KeyCtrlD), 1, true
	case c == 0x15: // Ctrl-U
		return KeyEvent(KeyCtrlU), 1, true
	case c < 0x20: // other C0 control bytes: drop
		return KeyEvent(KeyNone), 1, true
	default: // printable: decode one UTF-8 rune (space included)
		r, size := utf8.DecodeRune(b)
		if r == utf8.RuneError && size == 1 {
			if !utf8.FullRune(b) {
				return Event{}, 0, false // incomplete multibyte rune at the tail
			}
			return KeyEvent(KeyNone), 1, true // genuinely invalid byte: drop it
		}
		return RuneEvent(r), size, true
	}
}

// parseEsc handles an ESC-led sequence. b[0] is ESC.
//
// A lone trailing ESC is reported as KeyEsc immediately rather than treated as
// incomplete: terminals deliver multi-byte key sequences atomically in a single
// read, so a solitary ESC is almost always a real Esc press, and treating it as
// incomplete would block the Esc key. (The rare cost: a sequence split with ESC
// alone in one read would mis-parse. If that ever bites, a small read-timeout in
// the terminal driver is the fix - the Model and tests are unaffected.)
func parseEsc(b []byte) (Event, int, bool) {
	if len(b) == 1 {
		return KeyEvent(KeyEsc), 1, true
	}
	switch b[1] {
	case '[': // CSI
		return parseCSI(b)
	case 'O': // SS3 (application cursor keys): ESC O A, etc.
		return parseSS3(b)
	default:
		// ESC + other byte: we don't model Alt-<key>; treat ESC as a standalone
		// Esc and let the following byte parse on its own next iteration.
		return KeyEvent(KeyEsc), 1, true
	}
}

// parseCSI handles ESC [ ... <final>. b[0]=ESC, b[1]='['.
func parseCSI(b []byte) (Event, int, bool) {
	// Collect parameter/intermediate bytes (0x20-0x3f); the final byte is 0x40-0x7e.
	i := 2
	for i < len(b) && b[i] >= 0x20 && b[i] <= 0x3f {
		i++
	}
	if i >= len(b) {
		return Event{}, 0, false // no final byte yet: incomplete
	}
	final := b[i]
	params := string(b[2:i])
	consumed := i + 1

	switch final {
	case 'A':
		return KeyEvent(KeyUp), consumed, true
	case 'B':
		return KeyEvent(KeyDown), consumed, true
	case 'C':
		return KeyEvent(KeyRight), consumed, true
	case 'D':
		return KeyEvent(KeyLeft), consumed, true
	case 'H':
		return KeyEvent(KeyHome), consumed, true
	case 'F':
		return KeyEvent(KeyEnd), consumed, true
	case 'Z':
		return KeyEvent(KeyShiftTab), consumed, true
	case '~':
		switch params {
		case "1", "7":
			return KeyEvent(KeyHome), consumed, true
		case "4", "8":
			return KeyEvent(KeyEnd), consumed, true
		case "5":
			return KeyEvent(KeyPageUp), consumed, true
		case "6":
			return KeyEvent(KeyPageDown), consumed, true
		default: // "3"=Delete, ... - recognized shape, unmapped
			return KeyEvent(KeyNone), consumed, true
		}
	default:
		// Modified arrows like ESC [ 1;5 A (Ctrl+Up) land here when params is
		// non-empty but the final letter is handled above; anything else is an
		// unmapped CSI we consume and drop.
		return KeyEvent(KeyNone), consumed, true
	}
}

// parseSS3 handles ESC O <final> (application cursor mode). b[0]=ESC, b[1]='O'.
func parseSS3(b []byte) (Event, int, bool) {
	if len(b) < 3 {
		return Event{}, 0, false // incomplete
	}
	switch b[2] {
	case 'A':
		return KeyEvent(KeyUp), 3, true
	case 'B':
		return KeyEvent(KeyDown), 3, true
	case 'C':
		return KeyEvent(KeyRight), 3, true
	case 'D':
		return KeyEvent(KeyLeft), 3, true
	case 'H':
		return KeyEvent(KeyHome), 3, true
	case 'F':
		return KeyEvent(KeyEnd), 3, true
	default:
		return KeyEvent(KeyNone), 3, true
	}
}
