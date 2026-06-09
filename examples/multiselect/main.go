// Command multiselect is a tiny manual smoke test for radish's multi-select prompt.
//
// Run it in a real terminal:
//
//	go run ./examples/multiselect
//
// Arrow keys move, type to filter, Tab/Space toggles, Enter submits, Esc/Ctrl-C
// cancels. Pick between 1 and 3 toppings. The chosen values print to stdout (the
// menu draws to stderr).
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/amterp/radish"
)

func main() {
	toppings := []string{
		"Pepperoni", "Mushroom", "Onion", "Sausage", "Bacon",
		"Olives", "Peppers", "Pineapple", "Spinach", "Anchovies",
	}

	model := radish.NewMultiSelect().
		Title("Pick 1-3 toppings (Tab/Space to toggle)").
		Options(toppings...).
		Min(1).
		Max(3).
		Height(6)

	picks, ok, err := radish.RunMultiSelect(model, os.Stdin, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if !ok {
		fmt.Fprintln(os.Stderr, "canceled")
		os.Exit(130)
	}
	fmt.Println(strings.Join(picks, ", "))
}
