package radish

// KeyMap binds actions to one or more KeyTypes. Override any field to remap an
// action; a nil/empty binding means the action is unbound. Typing a printable rune
// and Backspace are the intrinsic text-input pair and are always filter edits
// (never remappable), which keeps "type to filter" unambiguous; higher-level
// editing commands like ClearFilter are bindable actions so they stay open to
// customization.
type KeyMap struct {
	Up          []KeyType
	Down        []KeyType
	PageUp      []KeyType
	PageDown    []KeyType
	Home        []KeyType
	End         []KeyType
	Submit      []KeyType
	Cancel      []KeyType
	ClearFilter []KeyType
}

// DefaultKeyMap is the conventional binding: arrows navigate, PageUp/PageDown jump
// a page, Home/End jump to the ends, Enter submits, Esc/Ctrl-C/Ctrl-D cancel, and
// Ctrl-U clears the filter.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:          []KeyType{KeyUp},
		Down:        []KeyType{KeyDown},
		PageUp:      []KeyType{KeyPageUp},
		PageDown:    []KeyType{KeyPageDown},
		Home:        []KeyType{KeyHome},
		End:         []KeyType{KeyEnd},
		Submit:      []KeyType{KeyEnter},
		Cancel:      []KeyType{KeyEsc, KeyCtrlC, KeyCtrlD},
		ClearFilter: []KeyType{KeyCtrlU},
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
