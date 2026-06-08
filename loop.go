package radish

import (
	"errors"
	"io"
)

// Model is a pure, deterministic interactive component. Update advances state in
// response to one Event; View renders the current frame as a multi-line string
// with no trailing newline. A Model performs no I/O and holds no global state, so
// the same Model runs identically in production and in tests.
type Model interface {
	Update(Event) (Model, Cmd)
	View() string
}

// Cmd is a control-flow signal returned by Update. It is a tiny synchronous enum,
// not an async command runtime: Run evaluates it inline.
type Cmd int

const (
	CmdNone   Cmd = iota // keep going
	CmdSubmit            // user confirmed; Run stops with Result{} (Canceled:false)
	CmdCancel            // user aborted; Run stops with Result{Canceled:true}
)

// EventSource yields input Events, blocking until one is available. It returns
// io.EOF when input is exhausted. In production this reads a real terminal; in
// tests it replays a scripted slice.
type EventSource interface {
	Next() (Event, error)
	Close() error
}

// FrameSink consumes rendered frames. Render replaces the previously-shown frame;
// Finish performs the terminal "collapse" - clearing the interactive frame and
// emitting a final summary (or nothing). In production this drives a real
// terminal; in tests it records frames.
type FrameSink interface {
	Render(frame string) error
	Finish(final string) error
	Close() error
}

// Result reports how an interaction ended. A run that returns a nil error ended
// in exactly one of two ways: the user submitted (Canceled == false) or aborted
// (Canceled == true, via Esc/Ctrl-C/Ctrl-D or EOF). The chosen value, if any,
// lives on the component (e.g. SelectModel.Selected) or is returned by the typed
// runners (RunSelect).
type Result struct {
	Canceled bool
}

// Summarizer is an optional Model capability: the collapsed line shown after the
// interaction ends (e.g. "Pick a food: Pizza"). Models that don't implement it
// collapse to nothing.
type Summarizer interface {
	Summary() string
}

// ErrNotInteractive is returned by RunTerminal when the input is not a TTY.
var ErrNotInteractive = errors.New("radish: not an interactive terminal")

// Run drives a Model to completion against the given source and sink. It is the
// single place the event loop lives:
//
//  1. render the initial frame
//  2. read an event; on io.EOF, treat as cancel
//  3. Update the model; on CmdSubmit/CmdCancel, collapse and return
//  4. otherwise render the new frame and repeat
//
// It returns the Result, the final Model (type-assert it to read a typed value,
// e.g. *SelectModel), and any I/O error from the sink/source.
func Run(model Model, src EventSource, sink FrameSink) (Result, Model, error) {
	defer src.Close()
	defer sink.Close()

	if err := sink.Render(model.View()); err != nil {
		return Result{}, model, err
	}

	for {
		ev, err := src.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return collapse(model, sink, Result{Canceled: true})
			}
			return Result{}, model, err
		}

		var cmd Cmd
		model, cmd = model.Update(ev)

		switch cmd {
		case CmdSubmit:
			return collapse(model, sink, Result{})
		case CmdCancel:
			return collapse(model, sink, Result{Canceled: true})
		}

		if err := sink.Render(model.View()); err != nil {
			return Result{}, model, err
		}
	}
}

func collapse(model Model, sink FrameSink, res Result) (Result, Model, error) {
	final := ""
	if s, ok := model.(Summarizer); ok {
		final = s.Summary()
	}
	if err := sink.Finish(final); err != nil {
		return res, model, err
	}
	return res, model, nil
}
