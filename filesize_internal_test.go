package filesize

import (
	"go/ast"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis"
)

// collector is a Pass wired to record diagnostics instead of printing them, so
// report can be driven directly against a FileSet the test controls.
func collector(fset *token.FileSet, into *[]analysis.Diagnostic) *analysis.Pass {
	return &analysis.Pass{
		Fset:   fset,
		Report: func(d analysis.Diagnostic) { *into = append(*into, d) },
	}
}

// known adds a file of the given name and length to the FileSet and returns the
// AST node positioned at its first byte, which is what report resolves.
func known(t *testing.T, fset *token.FileSet, name string, lines int) *ast.File {
	t.Helper()

	content := make([]byte, 0, lines)
	for range lines {
		content = append(content, '\n')
	}
	added := fset.AddFile(name, -1, len(content))
	added.SetLinesForContent(content)
	require.Equal(t, lines, added.LineCount())

	return &ast.File{Package: added.Pos(0)}
}

// TestReportIsSilentOnAFileTheFileSetDoesNotKnow pins the fail-safe: a node
// whose position the FileSet cannot resolve has neither a name nor a size, so
// the analyzer stays silent rather than guessing at either — and rather than
// dereferencing the nil the FileSet returns.
func TestReportIsSilentOnAFileTheFileSetDoesNotKnow(t *testing.T) {
	var reported []analysis.Diagnostic

	report(collector(token.NewFileSet(), &reported), &ast.File{Package: token.Pos(1)})

	assert.Empty(t, reported)
}

// TestReportJudgesSourceAndSpareTestAtTheSameLength pins the file class the rule
// judges, with length held constant so the name is the only thing that can
// decide the outcome. Both files are one line past the shipped default; the
// source is reported and the test file is not.
func TestReportJudgesSourceAndSpareTestAtTheSameLength(t *testing.T) {
	require.Equal(t, defaultMax, maxLines, "the pair is judged at the shipped default")

	fset := token.NewFileSet()
	source := known(t, fset, "pkg/thing.go", defaultMax+1)
	tests := known(t, fset, "pkg/thing_test.go", defaultMax+1)

	var reported []analysis.Diagnostic
	pass := collector(fset, &reported)
	report(pass, source)
	report(pass, tests)

	require.Len(t, reported, 1, "only the source file is judged")
	assert.Equal(t, source.Package, reported[0].Pos)
	assert.Equal(
		t,
		"file is 301 lines, over the 300-line limit; split it along the seam its sections already suggest",
		reported[0].Message,
	)
}

// TestIsTestMatchesTheGoToolsOwnSuffixOnBothSides pins the matcher at its
// boundary in both directions. The conforming near-misses are names that merely
// contain or resemble the suffix; the violating near-misses are names that carry
// it. A matcher loose enough to spare `thingtest.go`, or tight enough to judge
// `a_test.go`, disagrees with which files the go tool compiles into the test
// binary — and that compilation, not the spelling, is what the exemption rests
// on.
func TestIsTestMatchesTheGoToolsOwnSuffixOnBothSides(t *testing.T) {
	t.Parallel()

	for _, name := range []sourceName{"thing.go", "thingtest.go", "test.go", "thing_test.go.go", "thing_tests.go"} {
		assert.False(t, name.isTest(), "%s is compiled into the package", name)
	}
	for _, name := range []sourceName{"thing_test.go", "a_test.go", "pkg/nested/thing_test.go"} {
		assert.True(t, name.isTest(), "%s is compiled into the test binary alone", name)
	}
}
