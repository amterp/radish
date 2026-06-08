package radish

import "io"

// ScriptDriver replays a fixed sequence of Events through a Model and records
// every frame the Model renders. It fills the same role as the production
// terminal driver, but with no terminal: the real Model and View run unchanged -
// only the I/O edge is scripted. This is what makes deterministic, end-to-end
// snapshot tests of the real interactive UI possible.
type ScriptDriver struct {
	events []Event
	sink   *recordingSink
}

// NewScriptDriver creates a driver that will feed events in order, then EOF.
func NewScriptDriver(events []Event) *ScriptDriver {
	return &ScriptDriver{events: events, sink: &recordingSink{}}
}

// Run drives model to completion against the scripted events, recording frames.
func (d *ScriptDriver) Run(model Model) (Result, Model, error) {
	src := &sliceEventSource{events: d.events}
	return Run(model, src, d.sink)
}

// Frames returns the recorded frames in order: the initial render, the render
// after each consumed event, and (on submit) the final collapsed summary.
//
// The frames align positionally with the events: Frames()[0] is the initial
// frame, and Frames()[j] (j>=1) is the frame produced after Events()[j-1].
func (d *ScriptDriver) Frames() []string { return d.sink.frames }

// Events returns the scripted events, for callers that label frames by their
// triggering keystroke.
func (d *ScriptDriver) Events() []Event { return d.events }

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

func (s *sliceEventSource) Close() error { return nil }

type recordingSink struct {
	frames []string
}

func (r *recordingSink) Render(frame string) error {
	r.frames = append(r.frames, frame)
	return nil
}

func (r *recordingSink) Finish(final string) error {
	if final != "" {
		r.frames = append(r.frames, final)
	}
	return nil
}

func (r *recordingSink) Close() error { return nil }
