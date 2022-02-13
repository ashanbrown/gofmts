package main

import (
	"golang.org/x/tools/go/analysis"

	"github.com/ashanbrown/gofmts/pkg/analyzer"
)

// build this plugin for testing
//go:generate go build -buildmode=plugin ./plugin.go

type analyzerPlugin struct{}

func (*analyzerPlugin) GetAnalyzers() []*analysis.Analyzer {
	return []*analysis.Analyzer{
		analyzer.FormatAnalyzer,
		analyzer.SortAnalyzer,
	}
}

// This must be defined and named 'AnalyzerPlugin'
var AnalyzerPlugin analyzerPlugin //nolint:deadcode,gochecknoglobals,unused // this is used by golangci-lint

// This is just here to satisfy the build pre-commit.  It is not used anywhere.
func main() {}
