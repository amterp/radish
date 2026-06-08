# radish

A small, open, **testable** terminal-interactivity library for Go.

radish provides interactive terminal prompts built around a clean split between a
pure, deterministic `Model` (all state, logic, and rendering) and a thin I/O edge
(`EventSource` + `FrameSink`). In production the edge is a real terminal in raw
mode; in tests it's a scripted source of keystrokes and a recording sink of
frames. The same `Model` and `View()` run in both - so the real rendering and
logic are exercised directly in deterministic tests, not mocked.

It is deliberately **open where comparable libraries are closed**: the matcher,
key bindings, styling, event source, and render target are all injectable, each
with a sane default.

## Status

Early development. First component: single-select (`pick`).

## License

MIT - see [LICENSE](LICENSE).
