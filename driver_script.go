package radish

import "io"

// ScriptDriver replays a fixed sequence of Events through one or more Models and
// records every frame the Models render. It fills the same role as the production
// terminal driver, but with no terminal: the real Model and View run unchanged -
// only the I/O edge is scripted. This is what makes deterministic, end-to-end
// snapshot tests of the real interactive UI possible.
//
// The event source is consumed across Run calls: a second Run continues from
// wherever the first stopped, mirroring how sequential prompts share one real
// terminal. Frames likewise accumulate across Runs.
type ScriptDriver struct {
	src  *sliceEventSource
	sink *recordingSink
}

// NewScriptDriver creates a driver that will feed events in order, then EOF.
func NewScriptDriver(events []Event) *ScriptDriver {
	src := &sliceEventSource{events: events}
	return &ScriptDriver{src: src, sink: &recordingSink{src: src}}
}

// Run drives model to completion against the scripted events, recording frames.
func (d *ScriptDriver) Run(model Model) (Result, Model, error) {
	return Run(model, d.src, d.sink)
}

// Frames returns the recorded frames in order across all Runs: each prompt's
// initial render, the render after each consumed event, and (on submit) the
// final collapsed summary. Use FrameEventIdx to map a frame back to the event
// that produced it.
func (d *ScriptDriver) Frames() []string { return d.sink.frames }

// FrameEventIdx returns, for each recorded frame, the index into Events() of
// the event that triggered it, or -1 for a frame rendered without consuming an
// event (a prompt's initial render). Aligned with Frames().
func (d *ScriptDriver) FrameEventIdx() []int { return d.sink.eventIdx }

// Events returns the scripted events, for callers that label frames by their
// triggering keystroke.
func (d *ScriptDriver) Events() []Event { return d.src.events }

type sliceEventSource struct {
	events []Event
	i      int
}

func (s *sliceEventSource) Next() (Event, error) {
	if s.i >= len(s.events) {
		return Event{}, io.EOF
	}
	e := s.events[s.i]
	s.i++
	return e, nil
}

// Close is a no-op: Run closes its source when a prompt ends, but the script
// continues feeding any subsequent prompt from where it left off.
func (s *sliceEventSource) Close() error { return nil }

// recordingSink records frames alongside the index of the event that produced
// each one, read from the shared source's consumption cursor.
type recordingSink struct {
	src      *sliceEventSource
	frames   []string
	eventIdx []int
	lastIdx  int // src.i at the previous record, to detect event-less renders
}

func (r *recordingSink) record(frame string) {
	idx := -1
	if r.src.i > r.lastIdx {
		idx = r.src.i - 1
	}
	r.lastIdx = r.src.i
	r.frames = append(r.frames, frame)
	r.eventIdx = append(r.eventIdx, idx)
}

func (r *recordingSink) Render(frame string) error {
	r.record(frame)
	return nil
}

func (r *recordingSink) Finish(final string) error {
	if final != "" {
		r.record(final)
	}
	// Even when nothing is recorded (cancel collapses to no summary), advance the
	// cursor so the next prompt's initial render is not blamed on the ending event.
	r.lastIdx = r.src.i
	return nil
}

func (r *recordingSink) Close() error { return nil }
