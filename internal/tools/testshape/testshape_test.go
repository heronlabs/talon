package testshape_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/heronlabs/vps-agent/internal/tools/testshape"
)

func TestAnalyzer_AcceptsOneAssertionAndIgnoresNonTests(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), testshape.Analyzer, "example.com/valid")
}

func TestAnalyzer_ReportsForLoop(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), testshape.Analyzer, "example.com/loop")
}

func TestAnalyzer_ReportsRangeLoop(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), testshape.Analyzer, "example.com/ranges")
}

func TestAnalyzer_ReportsSubtest(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), testshape.Analyzer, "example.com/subtest")
}

func TestAnalyzer_ReportsTwoAssertions(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), testshape.Analyzer, "example.com/twoassertions")
}

func TestAnalyzer_ReportsNoAssertion(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), testshape.Analyzer, "example.com/noassertion")
}

func TestAnalyzer_CountsAnalysistestRun(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), testshape.Analyzer, "example.com/analyzertest")
}
