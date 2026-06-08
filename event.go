package radish

import "strconv"

// KeyType identifies a logical key. Printable input is delivered as KeyRune (the
// character is in Event.Rune); everything else is a named key. The set is kept
// deliberately small - just what the interactive components need - and grows only
// as new components require it.
type KeyType int

const (
	KeyNone      KeyType = iota // no event / dropped input
	KeyRune                     // a printable rune; Event.Rune holds the character
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyEnter
	KeyBackspace
	KeyTab
	KeyShiftTab
	KeyEsc
	KeyHome
	KeyEnd
	KeyCtrlC
	KeyCtrlD
)

// Modifier is a reserved bitfield for chord modifiers. The current parser never
// sets it (Ctrl-C / Ctrl-D have dedicated KeyTypes), but the field on Event keeps
// the type source-stable for when Alt/Ctrl chords are modelled later.
type Modifier uint8

const ModNone Modifier = 0

const (
	ModCtrl Modifier = 1 << iota // 1
	ModAlt                       // 2
)

// Event is one logical input. For KeyRune, Rune holds the character; for named
// keys, Rune is unused.
type Event struct {
	Type KeyType
	Rune rune
	Mods Modifier
}

// RuneEvent builds a printable-key Event.
func RuneEvent(r rune) Event { return Event{Type: KeyRune, Rune: r} }

// KeyEvent builds a named-key Event.
func KeyEvent(t KeyType) Event { return Event{Type: t} }

// keyNames is the canonical name for each named key. It is the single shared
// vocabulary between Event.String (display) and the snapshot-test key DSL
// (parsing), so the two never drift.
var keyNames = map[KeyType]string{
	KeyNone:      "none",
	KeyUp:        "up",
	KeyDown:      "down",
	KeyLeft:      "left",
	KeyRight:     "right",
	KeyEnter:     "enter",
	KeyBackspace: "backspace",
	KeyTab:       "tab",
	KeyShiftTab:  "shift-tab",
	KeyEsc:       "esc",
	KeyHome:      "home",
	KeyEnd:       "end",
	KeyCtrlC:     "ctrl-c",
	KeyCtrlD:     "ctrl-d",
}

// String returns a short, human-readable label, e.g. "up", "enter", "ctrl-c", or
// the character itself ("a", or "space" for ' '). Used in debug output and in
// snapshot frame markers.
func (e Event) String() string {
	if e.Type == KeyRune {
		if e.Rune == ' ' {
			return "space"
		}
		return string(e.Rune)
	}
	if name, ok := keyNames[e.Type]; ok {
		return name
	}
	return "key(" + strconv.Itoa(int(e.Type)) + ")"
}
