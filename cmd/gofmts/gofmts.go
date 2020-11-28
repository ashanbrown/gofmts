package main

import (
	"go/ast"
	"go/token"

	"github.com/ashanbrown/gfmts/pkg/gofmts"
)

func rewriteAndSort(fset *token.FileSet, file *ast.File) error {
	return gofmts.Rewrite(fset, file)
}
