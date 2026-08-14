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

func TestInlineRendererCursorPositionsWithinFrame(t *testing.T) {
	var r inlineRenderer
	got := r.renderCursor("abc\ndef\nghi", 1, 2)
	// Hide, erase, write 3 lines (cursor lands after "ghi"), return to column 0,
	// up 1 row to row 1, right 2, show.
	want := escHideCursor + escEraseBelow + "abc\r\ndef\r\nghi" +
		"\r" + "\x1b[1A" + "\x1b[2C" + escShowCursor
	if got != want {
		t.Errorf("renderCursor = %q, want %q", got, want)
	}
	if r.cursorRow != 1 {
		t.Errorf("cursorRow = %d, want 1", r.cursorRow)
	}
}

func TestInlineRendererCursorOnLastRowNeedsNoUpMove(t *testing.T) {
	var r inlineRenderer
	got := r.renderCursor("ab\ncd", 1, 0)
	want := escHideCursor + escEraseBelow + "ab\r\ncd" + "\r" + escShowCursor
	if got != want {
		t.Errorf("renderCursor = %q, want %q", got, want)
	}
}

// The redraw after a cursor render must walk up from where the cursor actually
// sits, not from the bottom of the block - otherwise every frame after the
// cursor leaves the last row drifts down the screen.
func TestInlineRendererRedrawAfterCursorWalksFromCursorRow(t *testing.T) {
	var r inlineRenderer
	_ = r.renderCursor("a\nb\nc", 0, 0) // cursor left on row 0 of 3, and shown
	got := r.render("x")
	// No cursor-up: already on row 0. Re-hidden because the cursor path had
	// shown it.
	want := escHideCursor + "\r" + escEraseBelow + "x"
	if got != want {
		t.Errorf("redraw after cursor = %q, want %q", got, want)
	}
}

func TestMarkCursorPlacesGlyph(t *testing.T) {
	cases := []struct {
		name     string
		frame    string
		row, col int
		want     string
	}{
		{"mid-line", "abc\ndef", 1, 1, "abc\nd‸ef"},
		{"line start", "abc", 0, 0, "‸abc"},
		{"line end", "abc", 0, 3, "abc‸"},
		{"past line end pads", "ab", 0, 4, "ab  ‸"},
		{"row out of range is untouched", "ab", 5, 0, "ab"},
		{"wide runes counted by display width", "🍔x", 0, 2, "🍔‸x"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := markCursor(c.frame, c.row, c.col); got != c.want {
				t.Errorf("markCursor = %q, want %q", got, c.want)
			}
		})
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
