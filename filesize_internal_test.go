package filesize

import (
	"go/ast"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestLinesInFailsSafeOnAnUnknownFile pins the fail-safe: a node whose position
// the FileSet does not know reports zero lines, which can never exceed a limit.
// The analyzer stays silent rather than guessing at a size it cannot read — and
// rather than dereferencing the nil the FileSet returns.
func TestLinesInFailsSafeOnAnUnknownFile(t *testing.T) {
	t.Parallel()

	empty := token.NewFileSet()
	unknown := &ast.File{Package: token.Pos(1)}

	assert.Equal(t, lineCount(0), linesIn(empty, unknown))
}

// TestLinesInCountsAKnownFile pins the ordinary path against a FileSet that
// does know the file.
func TestLinesInCountsAKnownFile(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	added := fset.AddFile("known.go", -1, 30)
	added.SetLinesForContent([]byte("package a\n\nfunc A() {}\n"))

	assert.Equal(t, lineCount(3), linesIn(fset, &ast.File{Package: added.Pos(0)}))
}
