package filesize_test

import (
	"testing"

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
