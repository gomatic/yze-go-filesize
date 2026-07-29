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
// Generated files are not this analyzer's concern: the yze framework drops
// findings in files carrying the standard generated marker before they are
// reported, so a machine-authored parser or protobuf stub is never counted.
package filesize

import (
	"go/ast"
	"go/token"
	"strconv"

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
var Registration = goyze.Registration{
	Name:       "filesize",
	Categories: []goyze.Category{"structure", "maintainability"},
	URL:        "https://docs.gomatic.dev/yze/filesize",
	Analyzer:   Analyzer,
}

// lineCount is a source file's length in lines.
type lineCount int

// run reports every file in the pass that exceeds the limit.
func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		report(pass, file)
	}
	return nil, nil
}

// report emits a diagnostic when the file exceeds the limit, anchored at the
// file's package clause so the finding lands at a stable position rather than
// at whichever line happens to be last.
func report(pass *analysis.Pass, file *ast.File) {
	lines := linesIn(pass.Fset, file)
	if int(lines) <= maxLines {
		return
	}
	pass.Reportf(file.Package, message, int(lines), maxLines)
}

// linesIn is the number of lines in the file containing the node. A file the
// FileSet does not know is reported as empty, which can never exceed a limit —
// the analyzer stays silent rather than guessing at a size it cannot read.
func linesIn(fset *token.FileSet, file *ast.File) lineCount {
	found := fset.File(file.Package)
	if found == nil {
		return 0
	}
	return lineCount(found.LineCount())
}
