package main

import (
	"go/ast"
	"go/printer"
	"go/scanner"

	"github.com/ashanbrown/gfmts/pkg/gofmts"
)

func reformatFile(file *ast.File) error {
	if err := handleIssues(gofmts.FormatFile(fileSet, file)); err != nil {
		return err
	}
	return nil
}

func sortFile(src []byte) ([]byte, error) {
	file, sourceAdj, indentAdj, err := parse(fileSet, "presorted", src, true)
	if err != nil {
		return nil, err
	}

	if err := handleIssues(gofmts.SortFile(fileSet, file)); err != nil {
		return nil, err
	}

	return format(fileSet, file, sourceAdj, indentAdj, src, printer.Config{Mode: printerMode, Tabwidth: tabWidth})
}

func handleIssues(issues []gofmts.Issue, err error) error {
	if err != nil {
		return err
	}
	errList := scanner.ErrorList{}
	for _, i := range issues {
		if _, hasReplacement := i.(gofmts.IssueWithReplacement); !hasReplacement {
			errList.Add(i.Position(), i.Details())
		}
	}
	return errList.Err()
}
