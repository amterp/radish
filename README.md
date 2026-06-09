# radish

A small, open, **testable** terminal-interactivity library for Go.

Most TUI prompt libraries are built on a general-purpose, timing-based renderer
that reads a real terminal in raw mode. That makes them lovely to use and nearly
impossible to test: you end up mocking the whole prompt, leaving the real
rendering and selection logic uncovered.

radish takes a different shape. Each prompt is a **pure `Model`** - all state,
logic, and rendering, with no I/O - sitting behind a thin, swappable **I/O edge**
(`EventSource` + `FrameSink`). In production the edge is a real terminal; in tests
it's a scripted list of keystrokes and a recording sink. The *same* `Model` and
`View()` run in both, so your tests exercise the real prompt end-to-end -
deterministically - instead of mocking it away.

It's also deliberately **open where comparable libraries are closed**: the
matcher, theme, key bindings, event source, and render target are all injectable,
each with a sane default so the simple case stays a one-liner.

## Install

```sh
go get github.com/amterp/radish
```

## Quick start

```go
model := radish.NewSelect().
    Title("Pick a fruit (type to filter)").
    Options("Apple", "Banana", "Cherry", "Date")

choice, ok, err := radish.RunSelect(model, os.Stdin, os.Stderr)
if errors.Is(err, radish.ErrNotInteractive) {
    // stdin isn't a TTY - fall back however you like
}
if !ok {
    return // canceled
}
fmt.Println(choice)
```

Arrow keys move, typing filters, Enter selects, Esc/Ctrl-C cancels. The menu draws
to the writer you pass (conventionally stderr), keeping stdout clean for your
program's actual result. When stdin isn't a terminal, `RunTerminal` returns
`ErrNotInteractive` without touching the terminal, so you control the fallback.

Try it: `go run ./examples/pick`.

## Testing - the point

Drive the real prompt with scripted keystrokes and inspect every rendered frame.
No terminal, no mocking:

```go
model := radish.NewSelect().Title("Pick").Options("Apple", "Banana", "Cherry")

driver := radish.NewScriptDriver([]radish.Event{
    radish.KeyEvent(radish.KeyDown),
    radish.KeyEvent(radish.KeyEnter),
})
driver.Run(model)

got, _ := model.Selected() // "Banana"
frames := driver.Frames()  // every frame, in order
```

`Frames()` returns the initial render, the frame after each keystroke, and the
final collapsed summary - ideal for snapshot tests. Disable color
(`color.NoColor = true`) and frames come out as clean plain text.

## Customize

Everything below is optional; the defaults are sensible.

```go
radish.NewSelect().
    Title("Pick").
    Options(opts...).
    Matcher(myMatcher).   // how typing filters; default is case-insensitive fuzzy
    Theme(myTheme).       // amterp/color styles; default is tasteful
    KeyMap(myKeyMap).     // rebind navigation; default is arrows + enter + esc
    Height(8).            // visible rows before the list scrolls
    Width(cols)           // truncate long labels to terminal width
```

- **Matcher** `func(filter, label string) (matched bool, rank int)` - inject your
  own matching/ranking; `DefaultMatcher` is a fuzzy subsequence match.
- **Theme** - a flat struct of `*color.Color` styles (via
  [`amterp/color`](https://github.com/amterp/color)); nil fields render plain.
- **KeyMap** - `[]KeyType` slices per action; trivially remappable.

## The prompt family

All three share the `Model`/`Run` seam and the same testability story:

- **`Select`** - single-select with live type-to-filter. `RunSelect`.
- **`MultiSelect`** - multi-select; Tab/Space toggles, with optional `Min`/`Max`
  bounds (`Max` blocks extra toggles, `Min` gates submit). Reuses Select's
  filter/viewport core. `RunMultiSelect`.
- **`Input`** - single-line text with an inline `Prompt`, optional `Title` heading
  and `Placeholder`, and an `Echo` mode: `EchoNormal`, `EchoMasked` (`•`), or
  `EchoNone` (sudo-style no-echo for secrets - nothing is rendered as you type, so
  the value never appears in any frame). `RunInput`.

There is intentionally no `Confirm` widget: a yes/no prompt is just an `Input` whose
result the caller interprets, so radish stays minimal and the policy lives with the
caller.

```go
// MultiSelect
picks, ok, err := radish.RunMultiSelect(
    radish.NewMultiSelect().Title("Pick toppings").Options(opts...).Max(3),
    os.Stdin, os.Stderr)

// Input (secret)
pw, ok, err := radish.RunInput(
    radish.NewInput().Prompt("Password > ").Echo(radish.EchoNone),
    os.Stdin, os.Stderr)
```

Try them: `go run ./examples/multiselect`, `go run ./examples/input`.

**Known limitations:** the prompt reads terminal width once at construction, so a
mid-prompt resize (SIGWINCH) may misalign one redraw; the seam can carry a resize
event later without changing the `Model` contract. `Input` truncates an
over-width line at the edges rather than horizontally scrolling - the full value is
always returned regardless of display.

## License

MIT - see [LICENSE](LICENSE).
