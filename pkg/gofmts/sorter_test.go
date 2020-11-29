package gofmts

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSorter(t *testing.T) {
	makeInputs := func(t *testing.T, src string) (*token.FileSet, *ast.File) {
		fset := token.NewFileSet()
		src = strings.TrimLeftFunc(src, unicode.IsSpace)
		//formatted, err := format.Source([]byte(src))
		//require.NoError(t, err)
		f, err := parser.ParseFile(fset, "", src, parser.ParseComments)
		require.NoError(t, err)
		return fset, f
	}

	srtr := Sorter{}

	t.Run("previous directive", func(t *testing.T) {
		issues, err := srtr.Run(makeInputs(t,
			//gofmts:go
			`
				package main
				
				const (
					//gofmts:sort
					Z = 2
					A = 1
				)
				`))
		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, "block is unsorted", issues[0].Details())
		assert.Equal(t, 5, issues[0].Position().Line)
		require.Implements(t, (*IssueWithReplacement)(nil), issues[0])
		assert.Equal(t, "\tA = 1\n\tZ = 2\n", issues[0].(IssueWithReplacement).Replacement())
	})

	t.Run("it drags comments around with anything that moves", func(t *testing.T) {
		issues, err := srtr.Run(makeInputs(t,
			//gofmts:go
			`
				package main
				
				const (
					//gofmts:sort
					Z = 2
					// B = 1
					A = 1
				)
				`))
		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, "block is unsorted", issues[0].Details())
		assert.Equal(t, 5, issues[0].Position().Line)
		require.Implements(t, (*IssueWithReplacement)(nil), issues[0])
		assert.Equal(t, "\t// B = 1\n\tA = 1\n\tZ = 2\n",
			issues[0].(IssueWithReplacement).Replacement())
	})

	t.Run("it works with a comment after the directive", func(t *testing.T) {
		issues, err := srtr.Run(makeInputs(t,
			//gofmts:go
			`
				package main
				
				const (
					//gofmts:sort
					// B = 1
					Z = 2
					A = 1
				)
				`))
		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, "block is unsorted", issues[0].Details())
		assert.Equal(t, 6, issues[0].Position().Line)
		require.Implements(t, (*IssueWithReplacement)(nil), issues[0])
		assert.Equal(t, "\tA = 1\n\tZ = 2\n",
			issues[0].(IssueWithReplacement).Replacement())
	})

	t.Run("it works with top-level const", func(t *testing.T) {
		issues, err := srtr.Run(makeInputs(t,
			//gofmts:go
			`
				package main
				
				//gofmts:sort
				const Z = 1
				const A = 2
				
				const B = 3
				`))
		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, "block is unsorted", issues[0].Details())
		assert.Equal(t, 4, issues[0].Position().Line)
		require.Implements(t, (*IssueWithReplacement)(nil), issues[0])
		assert.Equal(t, "const A = 2\nconst Z = 1\n",
			issues[0].(IssueWithReplacement).Replacement())
	})

	t.Run("it fails back-to-back with unused directive", func(t *testing.T) {
		issues, err := srtr.Run(makeInputs(t,
			//gofmts:go
			`
				package main
				
				//gofmts:sort
				const Z = 1
				const A = 2
				
				//gofmts:sort
				const Y = 1
				const B = 2
				`))
		require.NoError(t, err)
		require.Len(t, issues, 2)
		assert.Equal(t, "block is unsorted", issues[0].Details())
		assert.Equal(t, 4, issues[0].Position().Line)
		require.Implements(t, (*IssueWithReplacement)(nil), issues[0])

		assert.Equal(t, "unused directive `gofmts:sort`", issues[1].Details())
		assert.Equal(t, 7, issues[1].Position().Line)
	})

	t.Run("unused directive", func(t *testing.T) {
		issues, err := srtr.Run(makeInputs(t,
			//gofmts:go
			`
				package main
				
				//gofmts:sort
				`))
		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, "unused directive `gofmts:sort`", issues[0].Details())
		assert.Equal(t, 3, issues[0].Position().Line)
	})

	t.Run("unused directive due to whitespace", func(t *testing.T) {
		issues, err := srtr.Run(makeInputs(t,
			//gofmts:go
			`
				package main
				
				//gofmts:sort
				
				const A = 1
				`))
		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, "unused directive `gofmts:sort`", issues[0].Details())
		assert.Equal(t, 3, issues[0].Position().Line)
	})
}
