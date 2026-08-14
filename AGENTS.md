# Agent & contributor guide

radish is a small, testable terminal-interactivity library. The user-facing pitch and usage
live in `README.md`; the architecture is summarized in the package godoc (`doc.go`). This file
is the *why* and the house rules - read it before extending radish so new work stays coherent
as the prompt family grows.

## Design principles

- **Testability is structural, not bolted on.** Every component is a pure `Model` - all state,
  logic, and rendering, with no I/O, no globals, no time - behind a swappable I/O edge
  (`EventSource` + `FrameSink`). The same Model runs against a real terminal in production and a
  scripted driver in tests. The driver is *not* a mock of the logic; it only substitutes the
  byte-level edge, so tests exercise the real rendering and behavior. **Never let I/O leak into
  a Model.**
- **Open where comparable libraries are closed.** The matcher, theme, and key bindings are
  injectable, each with a sane default so the simple case stays a one-liner. This openness is
  the reason radish exists (we left huh partly for being closed) - when adding behavior, prefer
  an injection point over a hardcoded policy.
- **Minimal on purpose.** radish renders inline prompts and one inline multi-line editor -
  the input half of a REPL - and nothing more.

## Non-goals

radish is **not** a general TUI framework. No alt-screen, no animation/spinner loop, no mouse,
no full-screen layout, no general-purpose diffing renderer, no large cross-terminal key table.
We deliberately don't carry the machinery that inline prompts don't need - that machinery is
exactly what we left huh to escape. A feature that requires it doesn't belong here.

`EditorModel` is also **not** a readline: no completion, no syntax highlighting, no reverse
search, no vi mode, no kill ring, no persistent history. It is a text buffer with a cursor
that knows how to wrap and when to ask the caller whether it is finished. Anything that needs
to understand the *language* being typed belongs in the caller, reached through an injection
point - see `Complete`/`Indent` below.

## Extending the prompt family

Today: single-select (`Select`), multi-select (`MultiSelect`), single-line text (`Input`),
multi-line text (`Editor`). `Select` and `MultiSelect` share the filter/viewport/navigation
core in the unexported `list` struct (`list.go`), which both embed - extend or fix list
behavior there, not in two places. To keep new components coherent, a new component MUST:

- be a pure `Model` driven by `Run`, reusing the shared contract: `Result{Canceled}` for the
  outcome, a typed `RunX(...) (value, ok, err)` convenience that spares callers a Model type
  assertion, and `Title(...)` for the heading line. When two bits of outcome aren't enough,
  report a typed one from the component (`EditorModel.Outcome()`) rather than widening
  `Result` - the value has always lived on the component, and this is the same rule. Make the
  zero value the *unfinished* state: `Run` ends on a source EOF without the Model seeing an
  event, so a zero value meaning "submitted" would claim a success that never happened;
- route navigation and commands through bindable `KeyMap` actions, treating only printable
  runes and Backspace as the intrinsic, non-remappable text-input pair (the one sanctioned
  exception is MultiSelect's Space-to-toggle, a deliberate convention noted in `keymap.go`);
- truncate every rendered line to the configured width *before* styling (color-safe), so each
  frame line is exactly one visual row - the inline renderer's redraw accounting depends on it.
  A Model may soft-wrap to width instead of truncating, and if it does it MUST implement
  `Cursorer` and cap its rendered height: a wrapped frame reflows as a glyph cursor moves, and
  a frame taller than the terminal scrolls off the top, permanently desyncing `moveToStart`;
- render only via the injected sink - a Model never writes to a terminal directly. Note this
  is why there is no "clear the screen" key: that is I/O, so it belongs to the caller, which
  can act on an outcome and re-run the prompt;
- never reveal a secret in any frame: a masked/no-echo input renders placeholder glyphs (or
  nothing) and its `Summary()` must not echo the value.

`Editor` is where the "prefer an injection point over a hardcoded policy" principle does its
heaviest lifting. `Complete(func(string) bool)` decides whether Enter submits or opens a line,
and `Indent(func(string) string)` supplies the new line's leading whitespace. Both default to
single-line behavior. Every rule about the *language* being edited lives behind them, which is
what keeps a code editor rad-agnostic. The one rule the editor keeps for itself is that a blank
line always submits - without it a caller whose predicate never returns true would hold a
buffer no keystroke could close.

Conventions worth matching: `Title(...)` is always the optional heading line; `Input` adds an
inline `Prompt(...)` prefix rendered on the field line itself (mirroring how a shell prompt
sits before the cursor), and its `Summary()` uses that prompt (not the title) as the collapsed
label. `MultiSelect` reuses `Select`'s state with a Tab/Space toggle and `Min`/`Max` bounds
(`Max` blocks extra toggles, `Min` gates submit). Injectable openness is per-capability, not
universal: `Matcher` only exists on the filterable list prompts (`Select`/`MultiSelect`), not
on `Input`. When in doubt, match an existing component's shape rather than inventing a parallel
one - cross-component consistency is the point.

There is intentionally **no `Confirm` widget**: a yes/no prompt is an `Input` whose result the
caller interprets (e.g. empty or a `y`-prefix means yes). Keeping that policy with the caller
keeps radish minimal and avoids a near-duplicate of `Input`.

## Snapshot tests

Prompt behavior that is visible in a rendered frame is tested with
[go-snap](https://github.com/amterp/go-snap): a case in `snapshots/<model>/*.snap`
describes the prompt as data, scripts keystrokes in `### KEYS ###`, and stores every
frame it rendered in `### FRAMES ###`, each labeled with the key that produced it.
Prefer adding a case there over a new Go test whenever both would do - a labeled frame
sequence shows a reviewer what the user saw, where `strings.Contains` on `Frames()[3]`
only asserts that one detail was somewhere on screen.

Regenerate expected frames with `go test . -update=<path-substr>`, or `-update-all` for a
sweep, and read the diff: these files are the review surface, not a cache.

Two constraints shape where a test can live:

- The suites are in `package radish_test`. `go-snap/prompt` imports radish, so an
  in-package test importing it would be an import cycle. `TestMain` lives in
  `main_test.go` because the internal and external test packages share one binary and
  may declare it only once between them.
- `### TITLE ###` belongs to go-snap, which uses it for the case name, so the prompt
  heading is `### PROMPT ###`.

Keep a test in Go when a snapshot cannot express it: anything configured by an injected
closure (`Matcher`, `Validate`, `SummaryFunc`) since a text file cannot carry a function,
anything needing unexported access (`driver_term.go`'s ANSI output, `clampField`,
`keepTail`), and assertions about a count rather than content. The editor suite works around
the closure limit with a `POLICY` section naming one of a few stand-in `Complete` functions -
enough to exercise wrapping and continuation, while the real predicate is tested by its owner.

A `Cursorer` Model uses the terminal's own cursor, which a recorded frame cannot carry, so
`ScriptDriver` marks the position with `‸` in the frames it records. Keep that in radish
rather than pushing it into go-snap: the cursor is radish's protocol, and go-snap only ever
formats strings it is handed.

## Conventions & workflow

- `go test ./...`, `go vet ./...`, and `gofmt -l .` must all be clean before committing.
- Keep radish **rad-agnostic**: it must not import or know anything about Rad. Rad wires into
  it; radish stays a general-purpose library.
- The escape-sequence parser (`keyparser.go`) is the whole cross-platform input surface - keep
  it a pure `[]byte -> []Event` function and cover new keys with table-driven unit tests.
- radish is consumed by Rad through a local `replace` directive during development. **Do not cut
  version tags while iterating** - Rad pins the local path; tags come only at a real release.
- Commit messages: conventional prefixes (`feat:` / `fix:` / `refactor:` / `docs:` / `test:`),
  explaining *why*, not just *what*.
