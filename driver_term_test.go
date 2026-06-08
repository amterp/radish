package radish

import (
	"errors"
	"io"
	"os"
	"testing"
)

func TestInlineRendererFirstRenderHidesCursor(t *testing.T) {
	var r inlineRenderer
	got := r.render("a\nb\nc")
	want := escHideCursor + escEraseBelow + "a\r\nb\r\nc"
	if got != want {
		t.Errorf("first render = %q, want %q", got, want)
	}
	if r.lastLines != 3 {
		t.Errorf("lastLines = %d, want 3", r.lastLines)
	}
}

func TestInlineRendererRedrawMovesUpAndClears(t *testing.T) {
	var r inlineRenderer
	_ = r.render("a\nb\nc") // 3 lines
	got := r.render("x\ny") // redraw
	// Cursor already hidden; go to column 0, up 2 lines (3-1), erase below, rewrite.
	want := "\r" + "\x1b[2A" + escEraseBelow + "x\r\ny"
	if got != want {
		t.Errorf("redraw = %q, want %q", got, want)
	}
	if r.lastLines != 2 {
		t.Errorf("lastLines = %d, want 2", r.lastLines)
	}
}

func TestInlineRendererSingleLineRedrawNoUpMove(t *testing.T) {
	var r inlineRenderer
	_ = r.render("hello") // 1 line
	got := r.render("world")
	want := "\r" + escEraseBelow + "world" // "\r" only, no cursor-up
	if got != want {
		t.Errorf("single-line redraw = %q, want %q", got, want)
	}
}

func TestInlineRendererFinishWritesSummaryAndShowsCursor(t *testing.T) {
	var r inlineRenderer
	_ = r.render("a\nb") // 2 lines
	got := r.finish("Pick: a")
	want := "\r" + "\x1b[1A" + escEraseBelow + "Pick: a\r\n" + escShowCursor
	if got != want {
		t.Errorf("finish = %q, want %q", got, want)
	}
	if r.lastLines != 0 {
		t.Errorf("lastLines = %d, want 0 after finish", r.lastLines)
	}
}

func TestInlineRendererFinishCancelClearsBlock(t *testing.T) {
	var r inlineRenderer
	_ = r.render("a\nb")
	got := r.finish("") // cancel: no summary
	want := "\r" + "\x1b[1A" + escEraseBelow + escShowCursor
	if got != want {
		t.Errorf("finish(cancel) = %q, want %q", got, want)
	}
}

// RunTerminal must refuse a non-terminal input cleanly, which is the contract rad
// relies on for its no-TTY policy.
func TestRunTerminalNonTTYReturnsErrNotInteractive(t *testing.T) {
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer pr.Close()
	defer pw.Close()

	_, _, err = RunTerminal(NewSelect().Options("a", "b"), pr, io.Discard)
	if !errors.Is(err, ErrNotInteractive) {
		t.Fatalf("err = %v, want ErrNotInteractive", err)
	}
}
