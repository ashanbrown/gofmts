package gofmts

import (
	"go/token"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "gofmts",
	Doc:  "check for sort order of blocks",
	Run:  runAnalysis,
}

func runAnalysis(pass *analysis.Pass) (interface{}, error) {
	srtr := NewSorter()
	issues, err := srtr.Run(pass.Fset, pass.Files...)
	if err != nil {
		return nil, err
	}

	for _, i := range issues {
		diag := analysis.Diagnostic{
			Pos:      i.Pos(),
			Message:  i.Details(),
			Category: "style",
		}
		if ii, ok := i.(IssueWithReplacement); ok {
			diag.End = diag.Pos + token.Pos(ii.Length())
			diag.SuggestedFixes = []analysis.SuggestedFix{{
				Message: "reorder?",
				TextEdits: []analysis.TextEdit{{
					Pos:     diag.Pos,
					End:     diag.End,
					NewText: []byte(ii.Replacement()),
				}},
			}}
		}
		pass.Report(diag)
	}

	return nil, nil
}
