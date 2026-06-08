// Command pick is a tiny manual smoke test for radish's single-select prompt.
//
// Run it in a real terminal:
//
//	go run ./examples/pick
//
// Use the arrow keys to move, type to filter, Enter to choose, Esc/Ctrl-C to
// cancel. The chosen value is printed to stdout (the menu itself draws to stderr),
// so `go run ./examples/pick > /tmp/choice` captures just the selection.
package main

import (
	"fmt"
	"os"

	"github.com/amterp/radish"
)

func main() {
	fruits := []string{
		"Apple", "Apricot", "Banana", "Blueberry", "Cherry", "Date",
		"Elderberry", "Fig", "Grape", "Kiwi", "Lemon", "Mango",
		"Nectarine", "Orange", "Papaya", "Quince", "Raspberry", "Strawberry",
	}

	model := radish.NewSelect().
		Prompt("Pick a fruit (type to filter)").
		Options(fruits...).
		Height(8)

	res, final, err := radish.RunTerminal(model, os.Stdin, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if res.Canceled {
		fmt.Fprintln(os.Stderr, "canceled")
		os.Exit(130)
	}

	sel, _ := final.(*radish.SelectModel).Selected()
	fmt.Println(sel)
}
