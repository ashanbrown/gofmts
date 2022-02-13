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

	t.Run("it sorts strings by value", func(t *testing.T) {
		issues, err := srtr.Run(makeInputs(t,
			//gofmts:go
			`
					package main
					
					var x = []string{
						//gofmts:sort
						"Z",
						"A",
					}
					`))
		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, "block is unsorted", issues[0].Details())
		assert.Equal(t, 5, issues[0].Position().Line)
		require.Implements(t, (*IssueWithReplacement)(nil), issues[0])
		assert.Equal(t, "\t\"A\",\n\t\"Z\",\n", issues[0].(IssueWithReplacement).Replacement())
	})

	t.Run("it sorts literals by value", func(t *testing.T) {
		issues, err := srtr.Run(makeInputs(t,
			//gofmts:go
			`
					package main
					
					var x = []float{
						//gofmts:sort
						12.3,
						2,
					}
					`))
		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, "block is unsorted", issues[0].Details())
		assert.Equal(t, 5, issues[0].Position().Line)
		require.Implements(t, (*IssueWithReplacement)(nil), issues[0])
		assert.Equal(t, "\t2,\n\t12.3,\n", issues[0].(IssueWithReplacement).Replacement())
	})

	t.Run("it sorts struct fields", func(t *testing.T) {
		issues, err := srtr.Run(makeInputs(t,
			//gofmts:go
			`
					package main
					
					type t struct {
						//gofmts:sort
						b int
						a int
					}
					`))
		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, "block is unsorted", issues[0].Details())
		assert.Equal(t, 5, issues[0].Position().Line)
		require.Implements(t, (*IssueWithReplacement)(nil), issues[0])
		assert.Equal(t, "\ta int\n\tb int\n", issues[0].(IssueWithReplacement).Replacement())
	})

	t.Run("it sorts anonymous struct fields", func(t *testing.T) {
		issues, err := srtr.Run(makeInputs(t,
			//gofmts:go
			`
					package main
					
					type t struct {
						//gofmts:sort
						B
						A
					}
					`))
		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, "block is unsorted", issues[0].Details())
		assert.Equal(t, 5, issues[0].Position().Line)
		require.Implements(t, (*IssueWithReplacement)(nil), issues[0])
		assert.Equal(t, "\tA\n\tB\n", issues[0].(IssueWithReplacement).Replacement())
	})

	t.Run("it doesn't blow up wih mismatched types", func(t *testing.T) {
		issues, err := srtr.Run(makeInputs(t,
			//gofmts:go
			`
					package main
					
					var x = []int{
						//gofmts:sort
						1,
						"not a number",
					}
					`))
		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, "block is unsorted", issues[0].Details())
		assert.Equal(t, 5, issues[0].Position().Line)
		require.Implements(t, (*IssueWithReplacement)(nil), issues[0])
		assert.Equal(t, "\t\"not a number\",\n\t1,\n", issues[0].(IssueWithReplacement).Replacement())
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
