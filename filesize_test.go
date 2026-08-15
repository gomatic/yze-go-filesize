package filesize_test

import (
	"testing"

	goyze "github.com/gomatic/go-yze"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis/analysistest"

	filesize "github.com/gomatic/yze-go-filesize"
)

// TestOversizedFilesAreReported pins the contract at a limit low enough for a
// fixture to exceed: the long file is reported with both its size and the
// limit, and the short one is silent.
func TestOversizedFilesAreReported(t *testing.T) {
	maxFlag := filesize.Analyzer.Flags.Lookup("max")
	original := maxFlag.Value.String()
	t.Cleanup(func() { require.NoError(t, maxFlag.Value.Set(original)) })
	require.NoError(t, maxFlag.Value.Set("5"))

	analysistest.Run(t, analysistest.TestData(), filesize.Analyzer, "a")
}

// TestDefaultLimitIsThreeHundred pins the shipped default, chosen from the
// measured distribution of real code rather than picked aspirationally.
func TestDefaultLimitIsThreeHundred(t *testing.T) {
	assert.Equal(t, "300", filesize.Analyzer.Flags.Lookup("max").DefValue)
}

// TestRegistrationIsWellFormed pins the yze wiring.
func TestRegistrationIsWellFormed(t *testing.T) {
	assert.NoError(t, filesize.Registration.Validate())
	assert.Equal(t, "yze/filesize", filesize.Registration.RuleID())
	assert.Same(t, filesize.Analyzer, filesize.Registration.Analyzer)
}

// TestRegistrationDeclaresTheScopeTheAnalyzerEnforces pins the half of the rule
// no run can observe. The analyzer refuses test files itself, so a runner that
// applies the declared scope and one that does not agree either way — which is
// exactly what makes the declaration silently wrong if it drifts. TestScopeAll
// here would assert the analyzer reports in every file while report refuses to,
// the shape that left one rule meaning two different things depending on whether
// it was reached through the framework or through the standalone binary.
func TestRegistrationDeclaresTheScopeTheAnalyzerEnforces(t *testing.T) {
	assert.Equal(t, goyze.TestScopeSourceOnly, filesize.Registration.TestScope)
}

// TestTestFilesAreOutOfScopeAtTheDefaultBoundary pins the file class the rule
// judges, at the only bound where the answer is decidable. Both fixtures are 301
// lines — one line past the shipped default — and differ only in the _test.go
// suffix, so a run with no -max override must report the source and stay silent
// on the test file. Nothing weaker discriminates: a shorter test file is silent
// because of its size rather than its class, and a run at a lowered limit cannot
// see a widening keyed on the default.
func TestTestFilesAreOutOfScopeAtTheDefaultBoundary(t *testing.T) {
	maxFlag := filesize.Analyzer.Flags.Lookup("max")
	require.Equal(t, maxFlag.DefValue, maxFlag.Value.String(), "the pair is judged at the shipped default")

	analysistest.Run(t, analysistest.TestData(), filesize.Analyzer, "scope")
}
