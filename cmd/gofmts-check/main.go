package main

import (
	"golang.org/x/tools/go/analysis/multichecker"

	"github.com/ashanbrown/gofmts/pkg/analyzer"
)

func main() {
	multichecker.Main(analyzer.FormatAnalyzer, analyzer.SortAnalyzer)
}
