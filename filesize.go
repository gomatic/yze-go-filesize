// Package filesize provides a go/analysis analyzer that reports source files
// longer than a configured number of lines.
//
// File length is a proxy for how much one file is being asked to do, and it is
// the constraint that other rules only detect indirectly. A 1:1 test-layout
// rule, for instance, notices an oversized source only once somebody has
// already worked around it by splitting the tests — the symptom, one step
// removed and long after the fact. Measuring the file itself catches the cause.
//
// The default limit is deliberately loose rather than aspirational: measured
// across a real fleet the median Go file was 54 lines and the 95th percentile
// 285, so 300 flags the genuine outliers without reshaping ordinary code. A
// repository with a different profile sets -max.
//
// Test files are not judged, and _test.go is the whole of that rule. Length is a
// proxy, and every property it stands in for — complexity, naming, structure,
// assertions, error handling — is measured directly by an analyzer that applies
// to test code exactly as it applies to source, so where those hold the proxy
// has nothing left to say. What length additionally gets wrong about tests is
// that the number of tests a function needs is set by its branches rather than
// by its own length: one boundary, three error paths and a table of cases
// legitimately cost many times the source they cover, and a rule written against
// source growth reads that as a defect when it is the coverage requirement being
// met. The exemption cannot be forged without acquiring what it exempts —
// renaming a file to _test.go is what makes the go tool compile it into the test
// binary alone, so a source file cannot take the escape and stay source.
//
// Generated files are not this analyzer's concern: the yze framework drops
// findings in files carrying the standard generated marker before they are
// reported, so a machine-authored parser or protobuf stub is never counted.
package filesize

import (
	"go/ast"
	"strconv"
	"strings"

	goyze "github.com/gomatic/go-yze"
	"golang.org/x/tools/go/analysis"
)

// defaultMax is the line limit applied when -max is not set.
const defaultMax = 300

// message is the diagnostic emitted for an oversized file.
const message = "file is %d lines, over the %d-line limit; split it along the seam its sections already suggest"

// maxLines is the configured limit, bound to the -max flag.
var maxLines = defaultMax

// Analyzer reports files longer than the configured limit.
var Analyzer = newAnalyzer()

// newAnalyzer builds the analyzer and binds its flags.
func newAnalyzer() *analysis.Analyzer {
	a := &analysis.Analyzer{
		Name: "filesize",
		Doc:  "reports source files longer than the configured line limit (default " + strconv.Itoa(defaultMax) + ")",
		Run:  run,
	}
	a.Flags.IntVar(&maxLines, "max", defaultMax, "maximum number of lines permitted in one source file")
	return a
}

// Registration declares this analyzer to the yze framework.
//
// The scope is declared here rather than left to the runner's table because the
// rule is the analyzer's, not the catalog's: report already refuses test files,
// so a registration saying TestScopeAll would claim the analyzer reports in
// every file when it does not. Declaring it makes the two agree without adding
// a second mechanism — the driver's drop is a no-op for findings this analyzer
// never emits, and every caller that skips the driver (the standalone binary,
// `go vet -vettool`, analysistest) still gets the same answer.
var Registration = goyze.Registration{
	Name:       "filesize",
	Categories: []goyze.Category{"structure", "maintainability"},
	URL:        "https://docs.gomatic.dev/yze/filesize",
	TestScope:  goyze.TestScopeSourceOnly,
	Analyzer:   Analyzer,
}

// testSuffix names the files the go tool compiles into the test binary alone.
const testSuffix = "_test.go"

// lineCount is a source file's length in lines.
type lineCount int

// sourceName is a file's name as the FileSet knows it.
type sourceName string

// isTest reports whether the name belongs to a file compiled into the test
// binary alone, which this analyzer does not judge.
func (n sourceName) isTest() bool {
	return strings.HasSuffix(string(n), testSuffix)
}

// run reports every file in the pass that exceeds the limit.
func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		report(pass, file)
	}
	return nil, nil
}

// report emits a diagnostic when a source file exceeds the limit, anchored at
// the file's package clause so the finding lands at a stable position rather
// than at whichever line happens to be last.
//
// The FileSet is read once, because a second read is a second chance to
// disagree with the first: one lookup answers both what the file is called and
// how long it is. A file the FileSet does not know has neither answer, and the
// analyzer stays silent rather than guessing at either — or dereferencing the
// nil the FileSet returns.
func report(pass *analysis.Pass, file *ast.File) {
	found := pass.Fset.File(file.Package)
	if found == nil {
		return
	}
	if sourceName(found.Name()).isTest() {
		return
	}
	lines := lineCount(found.LineCount())
	if int(lines) <= maxLines {
		return
	}
	pass.Reportf(file.Package, message, int(lines), maxLines)
}
