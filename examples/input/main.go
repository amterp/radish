// Command input is a tiny manual smoke test for radish's text-input prompt.
//
// Run it in a real terminal:
//
//	go run ./examples/input
//
// It asks for a name (normal echo) and then a secret (no echo). Each answer is
// printed to stdout; the prompts draw to stderr.
package main

import (
	"fmt"
	"os"

	"github.com/amterp/radish"
)

func main() {
	name, ok, err := radish.RunInput(
		radish.NewInput().Prompt("Name > ").Placeholder("your name"),
		os.Stdin, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if !ok {
		fmt.Fprintln(os.Stderr, "canceled")
		os.Exit(130)
	}
	fmt.Println("name:", name)

	secret, ok, err := radish.RunInput(
		radish.NewInput().Prompt("Secret (no echo) > ").Echo(radish.EchoNone),
		os.Stdin, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if !ok {
		fmt.Fprintln(os.Stderr, "canceled")
		os.Exit(130)
	}
	fmt.Println("secret length:", len(secret))
}
