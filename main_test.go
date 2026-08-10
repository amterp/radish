package radish

import (
	"os"
	"testing"

	"github.com/amterp/color"
)

// Frames are asserted as plain text, so disable color for the whole package.
//
// This lives in its own file because the internal and external test packages
// compile into one binary and may declare TestMain only once between them; the
// snapshot suites in package radish_test rely on it too.
func TestMain(m *testing.M) {
	color.NoColor = true
	os.Exit(m.Run())
}
