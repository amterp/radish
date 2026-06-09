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
- **Minimal on purpose.** radish renders inline prompts and nothing more.

## Non-goals

radish is **not** a general TUI framework. No alt-screen, no animation/spinner loop, no mouse,
no full-screen layout, no general-purpose diffing renderer, no large cross-terminal key table.
We deliberately don't carry the machinery that inline prompts don't need - that machinery is
exactly what we left huh to escape. A feature that requires it doesn't belong here.

## Extending the prompt family

Today: single-select (`Select`), multi-select (`MultiSelect`), single-line text (`Input`).
`Select` and `MultiSelect` share the filter/viewport/navigation core in the unexported `list`
struct (`list.go`), which both embed - extend or fix list behavior there, not in two places.
To keep new components coherent, a new component MUST:

- be a pure `Model` driven by `Run`, reusing the shared contract: `Result{Canceled}` for the
  outcome, a typed `RunX(...) (value, ok, err)` convenience that spares callers a Model type
  assertion, and `Title(...)` for the heading line;
- route navigation and commands through bindable `KeyMap` actions, treating only printable
  runes and Backspace as the intrinsic, non-remappable text-input pair (the one sanctioned
  exception is MultiSelect's Space-to-toggle, a deliberate convention noted in `keymap.go`);
- truncate every rendered line to the configured width *before* styling (color-safe), so each
  frame line is exactly one visual row - the inline renderer's redraw accounting depends on it;
- render only via the injected sink - a Model never writes to a terminal directly;
- never reveal a secret in any frame: a masked/no-echo input renders placeholder glyphs (or
  nothing) and its `Summary()` must not echo the value.

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
