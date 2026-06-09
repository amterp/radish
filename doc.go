// Package radish is a small, open, testable terminal-interactivity library.
//
// radish provides interactive terminal prompts - single-select (Select),
// multi-select (MultiSelect), and single-line text (Input) - built around a strict
// separation between a pure, deterministic Model and a thin I/O edge:
//
//   - A Model holds all state, logic, and rendering. It is pure: no I/O, no
//     globals, no time. Update(Event) advances state; View() renders a frame.
//   - The I/O edge is two interfaces - EventSource (where input Events come
//     from) and FrameSink (where rendered frames go) - that Run is
//     parameterized by.
//
// In production the edge is a real terminal in raw mode. In tests it is a
// scripted source of Events and a recording sink of frames. The same Model and
// View run in both, so the real rendering and logic are exercised directly in
// deterministic tests rather than mocked away.
//
// radish is deliberately open where comparable libraries are closed: the match
// function, key bindings, styling, the event source, and the render target are
// all injectable, each with a sane default so the simple case stays a one-liner.
package radish
