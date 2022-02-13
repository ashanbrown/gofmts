package analyzer

import (
	"go/token"

	"github.com/ashanbrown/gofmts/pkg/gofmts"
	"github.com/pkg/errors"
	"golang.org/x/tools/go/analysis"
)

var SortAnalyzer = &analysis.Analyzer{
	Name: "gofmts:sort",
	Doc:  "ensure sort order of code blocks",
	Run:  runSortAnalysis,
}

func runSortAnalysis(pass *analysis.Pass) (interface{}, error) {
	srtr := gofmts.NewSorter()
	issues, err := srtr.Run(pass.Fset, pass.Files...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to analyze file for sort")
	}

	reportIssues(pass, issues, "sort?")
	return nil, nil
}

var FormatAnalyzer = &analysis.Analyzer{
	Name: "gofmts",
	Doc:  "canonicalize string formatting",
	Run:  runFormatAnalysis,
}

func runFormatAnalysis(pass *analysis.Pass) (interface{}, error) {
	fmtr := gofmts.NewFormatter()
	for _, file := range pass.Files {
		issues, err := fmtr.Run(nil /* this means we can't guess what the tab stop is */, pass.Fset, file)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to format file")
		}
		reportIssues(pass, issues, "reformat?")
	}

	return nil, nil
}

func reportIssues(pass *analysis.Pass, issues []gofmts.Issue, prompt string) {
	for _, i := range issues {
		diag := analysis.Diagnostic{
			Pos:      i.Pos(),
			Message:  i.Details(),
			Category: "style",
		}
		if ii, ok := i.(gofmts.IssueWithReplacement); ok {
			diag.End = diag.Pos + token.Pos(ii.Length()) + 1
			diag.SuggestedFixes = []analysis.SuggestedFix{{
				Message: prompt,
				TextEdits: []analysis.TextEdit{{
					Pos:     diag.Pos,
					End:     diag.End,
					NewText: []byte(ii.Replacement()),
				}},
			}}
		}
		pass.Report(diag)
	}
}
