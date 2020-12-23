package main

import (
	"go/ast"
	"go/printer"
	"go/scanner"
	"io/ioutil"

	"github.com/pkg/errors"

	"github.com/ashanbrown/gofmts/pkg/gofmts"
)

func reformatFile(src []byte, file *ast.File) error {
	if err := handleIssues(gofmts.FormatFile(src, fileSet, file)); err != nil {
		return err
	}
	return nil
}

func sortFile(src []byte) ([]byte, error) {
	file, sourceAdj, indentAdj, err := parse(fileSet, "presorted", src, true)
	if err != nil {
		tmpfile, tmpfileErr := ioutil.TempFile("", "gofmts-presorted*.go")
		if tmpfileErr != nil {
			report(errors.Errorf("internal error: unable to write tempfile for presorted: %s", tmpfileErr))
		} else {
			report(errors.Errorf("internal error: writing temporary file for failed format prior to sort at %s", tmpfile.Name()))
			if _, err := tmpfile.Write(src); err != nil {
				report(errors.Errorf("internal error: unable to write to tempfile: %s", err))
			}
			if err := tmpfile.Close(); err != nil {
				report(errors.Errorf("internal error: unable to write to tempfile: %s", err))
			}
		}
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
