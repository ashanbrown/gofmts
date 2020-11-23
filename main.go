package main

import (
	"bytes"
	"flag"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"log"
	"os"

	"golang.org/x/tools/go/ast/astutil"

	"github.com/ashanbrown/gfmts/pkg/formatter"
)

func main() {
	log.SetPrefix("gofmts")
	log.SetFlags(0) // remove log timestamp
	inPlace := flag.Bool("i", false, "do in-place formatting")

	setExitStatus := flag.Bool("set_exit_status", false,
		"Set exit status to 1 if any allIssues are found")
	flag.Parse()

	if len(flag.Args()) > 1 && !*inPlace {
		log.Fatalf("multiple files may only be listed with in-place formatting")
	}

	files := flag.Args()

	fmtr := formatter.NewFormatter()

	var src io.Reader
	if len(files) == 0 {
		src = os.Stdin
	}

	code := 0
	hasReplacements := false
	fset := token.NewFileSet()
	for _, file := range files {
		fileNode, err := parser.ParseFile(fset, file, src, parser.ParseComments)
		if err != nil {
			log.Fatalf("failed to parse file %q", file)
		}
		issues, err := fmtr.Run(fset, fileNode)
		if err != nil {
			log.Fatalf("failed: %s", err)
		}

		hasError := false
		replacementsByPosition := make(map[token.Position]formatter.Issue, len(issues))
		for _, i := range issues {
			if i.Replacement() != nil {
				replacementsByPosition[i.Position()] = i
				hasReplacements = true
			} else {
				hasError = true
				log.Printf("error at %s: %s", i.Position(), i.Details())
			}
		}

		if hasError { // skip rewrite on error
			code = 1
			continue
		}

		writeBuf := new(bytes.Buffer)

		// apply replacements
		astutil.Apply(fileNode, nil, func(c *astutil.Cursor) bool {
			switch node := c.Node().(type) {
			case *ast.BasicLit:
				issue, exists := replacementsByPosition[fset.Position(node.Pos())]
				if !exists && issue.Replacement() != nil {
					break
				}
				replacementNode := &ast.BasicLit{
					Kind:  token.STRING,
					Value: *issue.Replacement(),
				}
				c.Replace(replacementNode)
			}
			return true
		})

		if err := printer.Fprint(writeBuf, fset, fileNode); err != nil {
			log.Printf("unable to rewrite %q: %s", file, err)
			code = 1
		}

		formatted, err := format.Source(writeBuf.Bytes())
		if err != nil {
			log.Printf("unable to format %q: %s", file, err)
			code = 1
		}

		var w io.Writer
		if src == os.Stdin || !*inPlace && len(files) == 1 {
			w = os.Stdout
		} else if *inPlace {
			var err error
			w, err = os.OpenFile(file, os.O_WRONLY, 0 /* file should already exist */)
			if err != nil {
				log.Printf("could not open file %q for writing", file)
				code = 1
			}
		}

		if w != nil {
			if _, err := w.Write(formatted); err != nil {
				log.Printf("write failed for file %q", file)
				code = 1
			}
		}
	}

	if *setExitStatus && hasReplacements {
		code = 1
	}

	os.Exit(code)
}
