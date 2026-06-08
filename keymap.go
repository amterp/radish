package radish

// KeyMap binds navigation actions to one or more KeyTypes. Override any field to
// remap an action; a nil/empty binding means the action is unbound. Printable
// runes and backspace always edit the filter and are intentionally not remappable,
// which keeps "type to filter" unambiguous.
type KeyMap struct {
	Up     []KeyType
	Down   []KeyType
	Submit []KeyType
	Cancel []KeyType
	Home   []KeyType
	End    []KeyType
}

// DefaultKeyMap is the conventional binding: arrows navigate, Enter submits,
// Esc/Ctrl-C/Ctrl-D cancel, Home/End jump to the ends.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:     []KeyType{KeyUp},
		Down:   []KeyType{KeyDown},
		Submit: []KeyType{KeyEnter},
		Cancel: []KeyType{KeyEsc, KeyCtrlC, KeyCtrlD},
		Home:   []KeyType{KeyHome},
		End:    []KeyType{KeyEnd},
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
