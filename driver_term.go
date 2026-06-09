package radish

import (
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

const (
	escHideCursor = "\x1b[?25l"
	escShowCursor = "\x1b[?25h"
	escEraseBelow = "\x1b[J" // erase from cursor to end of screen
)

// RunTerminal drives a Model against a real terminal: in is put into raw mode and
// read for keystrokes, frames are rendered inline to out. It returns
// ErrNotInteractive (without touching the terminal) when in is not a TTY, so
// callers can fall back to non-interactive behavior. out is conventionally stderr,
// keeping stdout clean for a program's actual result.
func RunTerminal(model Model, in *os.File, out io.Writer) (Result, Model, error) {
	d, err := NewTermDriver(in, out)
	if err != nil {
		return Result{}, model, err
	}
	// d is both the EventSource and the FrameSink; Run closes it (idempotently).
	return Run(model, d, d)
}

// RunSelect runs a single-select prompt on a real terminal and returns the chosen
// value. ok is false when the user canceled (Esc/Ctrl-C). It returns
// ErrNotInteractive when in is not a TTY. This is the convenient one-call form;
// for full control over the I/O edge (e.g. tests) use Run with your own
// EventSource/FrameSink, or a ScriptDriver.
func RunSelect(m *SelectModel, in *os.File, out io.Writer) (value string, ok bool, err error) {
	res, _, err := RunTerminal(m, in, out)
	if err != nil {
		return "", false, err
	}
	if res.Canceled {
		return "", false, nil
	}
	v, _ := m.Selected()
	return v, true, nil
}

// RunInput runs a single-line text prompt on a real terminal and returns the typed
// value. ok is false when the user canceled (Esc/Ctrl-C). It returns
// ErrNotInteractive when in is not a TTY. The convenient one-call form; for full
// control over the I/O edge (e.g. tests) use Run with a ScriptDriver.
func RunInput(m *InputModel, in *os.File, out io.Writer) (value string, ok bool, err error) {
	res, _, err := RunTerminal(m, in, out)
	if err != nil {
		return "", false, err
	}
	if res.Canceled {
		return "", false, nil
	}
	v, _ := m.Value()
	return v, true, nil
}

// RunMultiSelect runs a multi-select prompt on a real terminal and returns the
// chosen values. ok is false when the user canceled (Esc/Ctrl-C). It returns
// ErrNotInteractive when in is not a TTY. The convenient one-call form; for full
// control over the I/O edge (e.g. tests) use Run with a ScriptDriver.
func RunMultiSelect(m *MultiSelectModel, in *os.File, out io.Writer) (values []string, ok bool, err error) {
	res, _, err := RunTerminal(m, in, out)
	if err != nil {
		return nil, false, err
	}
	if res.Canceled {
		return nil, false, nil
	}
	return m.Selected(), true, nil
}

// TermDriver reads raw keystrokes from a terminal and renders frames to it. It
// implements both EventSource and FrameSink. The escape-sequence accounting lives
// in inlineRenderer (pure, unit-tested); TermDriver only adds the raw-mode
// lifecycle and the byte reads, which inherently need a real terminal.
type TermDriver struct {
	in       *os.File
	out      io.Writer
	fd       int
	oldState *term.State

	readBuf  []byte  // scratch for one Read
	leftover []byte  // bytes read but not yet parsed (e.g. a split escape sequence)
	pending  []Event // parsed events not yet returned

	rend inlineRenderer
}

// NewTermDriver puts in into raw mode and returns a driver writing to out. It
// returns ErrNotInteractive if in is not a terminal.
func NewTermDriver(in *os.File, out io.Writer) (*TermDriver, error) {
	fd := int(in.Fd())
	if !term.IsTerminal(fd) {
		return nil, ErrNotInteractive
	}
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return nil, err
	}
	return &TermDriver{
		in:       in,
		out:      out,
		fd:       fd,
		oldState: oldState,
		readBuf:  make([]byte, 256),
	}, nil
}

// Next returns the next key event, reading and parsing more bytes as needed. A
// partial escape sequence at the end of a read is retained and completed by the
// following read.
func (d *TermDriver) Next() (Event, error) {
	for len(d.pending) == 0 {
		n, err := d.in.Read(d.readBuf)
		if n > 0 {
			d.leftover = append(d.leftover, d.readBuf[:n]...)
			events, consumed := Parse(d.leftover)
			if consumed > 0 {
				rem := copy(d.leftover, d.leftover[consumed:])
				d.leftover = d.leftover[:rem]
			}
			d.pending = append(d.pending, events...)
		}
		if err != nil {
			if len(d.pending) > 0 {
				break // surface buffered events before reporting the error
			}
			return Event{}, err
		}
	}
	e := d.pending[0]
	d.pending = d.pending[1:]
	return e, nil
}

func (d *TermDriver) Render(frame string) error {
	_, err := io.WriteString(d.out, d.rend.render(frame))
	return err
}

func (d *TermDriver) Finish(final string) error {
	_, err := io.WriteString(d.out, d.rend.finish(final))
	return err
}

// Close restores the terminal. It is safe to call more than once (Run defers it
// for both the source and the sink role).
func (d *TermDriver) Close() error {
	if d.rend.cursorHidden {
		_, _ = io.WriteString(d.out, escShowCursor)
		d.rend.cursorHidden = false
	}
	if d.oldState != nil {
		err := term.Restore(d.fd, d.oldState)
		d.oldState = nil
		return err
	}
	return nil
}

// inlineRenderer computes the byte sequences to display successive frames inline
// (no alt-screen). It tracks the previous frame's line count so it can move the
// cursor back to the block's start, clear, and redraw. It returns bytes rather
// than writing them, so the escape-sequence logic is unit-testable without a
// terminal.
type inlineRenderer struct {
	lastLines    int
	cursorHidden bool
}

func (r *inlineRenderer) render(frame string) string {
	var b strings.Builder
	if !r.cursorHidden {
		b.WriteString(escHideCursor)
		r.cursorHidden = true
	}
	r.moveToStart(&b)
	b.WriteString(escEraseBelow)
	b.WriteString(toRawLines(frame))
	r.lastLines = strings.Count(frame, "\n") + 1
	return b.String()
}

func (r *inlineRenderer) finish(final string) string {
	var b strings.Builder
	r.moveToStart(&b)
	b.WriteString(escEraseBelow)
	if final != "" {
		b.WriteString(toRawLines(final))
		b.WriteString("\r\n")
	}
	if r.cursorHidden {
		b.WriteString(escShowCursor)
		r.cursorHidden = false
	}
	r.lastLines = 0
	return b.String()
}

// moveToStart returns the cursor to column 0 of the first line of the previously
// rendered block, ready to clear and redraw.
func (r *inlineRenderer) moveToStart(b *strings.Builder) {
	if r.lastLines == 0 {
		return
	}
	b.WriteString("\r")
	if up := r.lastLines - 1; up > 0 {
		b.WriteString("\x1b[" + strconv.Itoa(up) + "A")
	}
}

// toRawLines converts "\n" to "\r\n", which is what a terminal in raw mode needs
// to actually return to column 0 on each new line.
func toRawLines(s string) string {
	return strings.ReplaceAll(s, "\n", "\r\n")
}
