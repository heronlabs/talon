package analyzertest

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	dir := analysistest.TestData()
	_ = analysistest.TestData()

	analysistest.Run(t, dir)
}
