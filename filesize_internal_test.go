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
//
// isTest is handed a PATH, not a base name — every real run passes an absolute
// one — so the cases vary that dimension deliberately. A corpus that holds it
// constant cannot discriminate any rule keyed on it, and the escape a widening
// would take is a directory segment or a prefix rather than a suffix: silencing
// everything under /internal/, or under the absolute root every file in a fleet
// shares. Both spellings pass a matcher tested only on base names, and both go
// on passing a coverage gate, because a disjunct added to an existing return
// adds no statement.
func TestIsTestMatchesTheGoToolsOwnSuffixOnBothSides(t *testing.T) {
	t.Parallel()

	sources := []sourceName{
		"thing.go", "thingtest.go", "test.go", "thing_test.go.go", "thing_tests.go",
		"/Users/someone/src/repo/internal/engine/thing.go",
		"/Users/someone/src/repo/testdata/src/a/thing.go",
		"/private/tmp/build/generated_gen.go",
	}
	for _, name := range sources {
		assert.False(t, name.isTest(), "%s is compiled into the package", name)
	}

	tests := []sourceName{
		"thing_test.go", "a_test.go", "pkg/nested/thing_test.go",
		"/Users/someone/src/repo/internal/engine/thing_test.go",
	}
	for _, name := range tests {
		assert.True(t, name.isTest(), "%s is compiled into the test binary alone", name)
	}
}

// TestReportJudgesTheRealNameNotTheRewrittenPosition pins which of a file's two
// names decides the exemption, and it is the one the file cannot rewrite.
//
// A //line directive changes what fset.Position reports without changing what
// go/build compiles: the file below is still linked into its package under its
// real name. token.File.Name() is that real name; Position().Filename is the
// rewritten one. Judging by the rewritten name would hand every author a
// one-comment escape from the rule, which is exactly how go-yze's own
// source-only drop can be forged — the reason this analyzer declares no scope
// and filters for itself.
func TestReportJudgesTheRealNameNotTheRewrittenPosition(t *testing.T) {
	require.Equal(t, defaultMax, maxLines, "the case is judged at the shipped default")

	fset := token.NewFileSet()
	file := known(t, fset, "pkg/thing.go", defaultMax+1)
	found := fset.File(file.Package)
	found.AddLineColumnInfo(0, "zz_test.go", 1, 1)

	require.Equal(t, "pkg/thing.go", found.Name(), "the name go/build compiles by")
	require.Equal(t, "zz_test.go", fset.Position(file.Package).Filename, "the name the directive rewrote")

	var reported []analysis.Diagnostic
	report(collector(fset, &reported), file)

	assert.Len(t, reported, 1, "a source file does not escape by renaming its own position")
}
