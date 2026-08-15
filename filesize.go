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
// Test files are not judged. Not because tests deserve less care, but because
// LENGTH specifically says nothing true about them: the number of tests a
// function needs is set by its branches rather than by its own length, so one
// boundary, three error paths and a table of cases legitimately cost many times
// the source they cover, and a rule written against source growth reads that as
// a defect when it is the coverage requirement being met. Correctness rules
// still apply to test code exactly as they apply to source; length is the only
// property tests are excused from, and that exemption is the owner's decision
// rather than this analyzer's inference.
//
// The exemption is decided by the file's name, and the name is what makes it
// unforgeable — not any sentence here. isTest applies
// strings.HasSuffix(name, "_test.go"), which is the expression go/build itself
// applies when deciding which files it compiles into the test binary alone, so
// acquiring the marker IS being dropped from the package build and no spelling
// buys the silence without paying for it. The claim is deliberately no wider
// than that: the go tool also compiles a generated _testmain.go into the test
// binary alone, and this rule does not spare it, because a generated file is
// the framework's exemption to apply rather than this one's.
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
// It deliberately declares NO TestScope, and that is a decision rather than an
// omission. TestScopeAll asks the driver to filter nothing, which is exactly
// right here: report has already refused every test file, so there is nothing
// left for a second filter to remove and something it would wrongly remove.
// go-yze decides a source-only rule's drop from the RESOLVED position path —
// outOfScope reads Diagnostic.Path, which convert.go takes from fset.Position —
// and fset.Position applies //line directives. So `//line zz_test.go:1` on the
// first line of an ordinary source file, one that go/build still compiles into
// the package and any importer still links, makes that driver drop a finding
// this analyzer correctly emitted. isTest reads token.File.Name(), which no
// directive rewrites.
//
// Declaring the scope would therefore buy no silence this analyzer does not
// already produce correctly, and one it must not produce: a forgeable escape
// costing a single comment, standing beside an unforgeable one that costs being
// compiled out of the package. The rule stays where its matcher cannot be
// rewritten by the file it is judging.
var Registration = goyze.Registration{
	Name:       "filesize",
	Categories: []goyze.Category{"structure", "maintainability"},
	URL:        "https://docs.gomatic.dev/yze/filesize",
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
