package analyzer_test

import (
	"testing"

	"github.com/ashanbrown/gofmts/pkg/analyzer"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestSortAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.RunWithSuggestedFixes(t, testdata, analyzer.SortAnalyzer, "./sort")
}

func TestFormatAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.RunWithSuggestedFixes(t, testdata, analyzer.FormatAnalyzer, "./format")
}
