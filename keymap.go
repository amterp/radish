package radish

// KeyMap binds actions to one or more KeyTypes. Override any field to remap an
// action; a nil/empty binding means the action is unbound. Typing a printable rune
// and Backspace are the intrinsic text-input pair and are always filter edits
// (never remappable), which keeps "type to filter" unambiguous; higher-level
// editing commands like ClearFilter are bindable actions so they stay open to
// customization.
//
// One KeyMap serves the whole prompt family, so a given component reads only the
// actions it supports and silently ignores the rest (e.g. Input ignores Up/Down/
// Toggle; Select ignores Left/Right). MultiSelect also toggles on Space, which is a
// deliberate, non-remappable exception to "only runes and Backspace are intrinsic"
// (Space-to-toggle is a near-universal convention and Space is useless as a filter
// character).
type KeyMap struct {
	Up          []KeyType
	Down        []KeyType
	Left        []KeyType // move the text cursor left (Input)
	Right       []KeyType // move the text cursor right (Input)
	PageUp      []KeyType
	PageDown    []KeyType
	Home        []KeyType
	End         []KeyType
	Toggle      []KeyType // toggle the row's selection (MultiSelect); Space also toggles
	Submit      []KeyType
	Cancel      []KeyType
	ClearFilter []KeyType

	// Editor-only actions. Discard and EndOfInput exist because an editor has to
	// tell "throw this away and give me a fresh prompt" (Ctrl-C) apart from "I am
	// done providing input" (Ctrl-D on an empty buffer), which the single Cancel
	// action cannot express.
	Discard        []KeyType
	EndOfInput     []KeyType
	KillLine       []KeyType // delete the whole current line
	KillToEnd      []KeyType // delete from the cursor to end of line
	DeleteWordBack []KeyType
}

// DefaultKeyMap is the conventional binding: arrows navigate (Up/Down) or move the
// text cursor (Left/Right), PageUp/PageDown jump a page, Home/End jump to the ends
// (Ctrl-A/Ctrl-E too, as in every readline), Enter submits, Esc/Ctrl-C/Ctrl-D
// cancel, and Ctrl-U clears the filter.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:          []KeyType{KeyUp},
		Down:        []KeyType{KeyDown},
		Left:        []KeyType{KeyLeft},
		Right:       []KeyType{KeyRight},
		PageUp:      []KeyType{KeyPageUp},
		PageDown:    []KeyType{KeyPageDown},
		Home:        []KeyType{KeyHome, KeyCtrlA},
		End:         []KeyType{KeyEnd, KeyCtrlE},
		Toggle:      []KeyType{KeyTab},
		Submit:      []KeyType{KeyEnter},
		Cancel:      []KeyType{KeyEsc, KeyCtrlC, KeyCtrlD},
		ClearFilter: []KeyType{KeyCtrlU},

		Discard:        []KeyType{KeyCtrlC, KeyEsc},
		EndOfInput:     []KeyType{KeyCtrlD},
		KillLine:       []KeyType{KeyCtrlU},
		KillToEnd:      []KeyType{KeyCtrlK},
		DeleteWordBack: []KeyType{KeyCtrlW},
	}
}

func (k KeyMap) matches(e Event, binding []KeyType) bool {
	for _, kt := range binding {
		if e.Type == kt {
			return true
		}
	}
	return false
}
