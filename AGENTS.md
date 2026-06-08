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

Today: single-select (`Select`). Planned, all on the same seam: `Confirm`, `Input`,
`MultiSelect`. To keep them coherent, a new component MUST:

- be a pure `Model` driven by `Run`, reusing the shared contract: `Result{Canceled}` for the
  outcome, a typed `RunX(...) (value, ok, err)` convenience that spares callers a Model type
  assertion, and `Title(...)` for the prompt line;
- route navigation and commands through bindable `KeyMap` actions, treating only printable
  runes and Backspace as the intrinsic, non-remappable text-input pair;
- truncate every rendered line to the configured width *before* styling (color-safe), so each
  frame line is exactly one visual row - the inline renderer's redraw accounting depends on it;
- render only via the injected sink - a Model never writes to a terminal directly.

`MultiSelect` reuses `Select`'s state with a Tab/Space toggle. When in doubt, match `Select`'s
shape rather than inventing a parallel one - cross-component consistency is the point.

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
