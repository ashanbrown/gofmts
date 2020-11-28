package gofmts

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSorter(t *testing.T) {
	makeInputs := func(t *testing.T, src string) (*token.FileSet, *ast.File) {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "", src, parser.ParseComments)
		require.NoError(t, err)
		return fset, f
	}

	srtr := Sorter{}

	t.Run("previous directive", func(t *testing.T) {
		issues, err := srtr.run(makeInputs(t,
			`package main

				const (
					//gofmts:sort
					Z = 2
					A = 1
				)`))
		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, "block is unsorted", issues[0].Details())
		assert.Equal(t, 5, issues[0].Position().Line)
		require.Implements(t, (*IssueWithReplacement)(nil), issues[0])
		assert.Equal(t, "\tA = 1\n\tZ = 2\n", issues[0].(IssueWithReplacement).Replacement())
		assert.Equal(t, "block is unsorted at 5:6", issues[0].String())
	})

	t.Run("it drags comments around with anything that moves", func(t *testing.T) {
		issues, err := srtr.run(makeInputs(t,
			`package main

				const (
					//gofmts:sort
					Z = 2
					// B = 1
					A = 1
				)`))
		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, "block is unsorted", issues[0].Details())
		assert.Equal(t, 5, issues[0].Position().Line)
		require.Implements(t, (*IssueWithReplacement)(nil), issues[0])
		assert.Equal(t, "\t// B = 1\n\tA = 1\n\tZ = 2\n",
			issues[0].(IssueWithReplacement).Replacement())
		assert.Equal(t, "block is unsorted at 5:6", issues[0].String())
	})

	t.Run("it works with a comment after the directive", func(t *testing.T) {
		issues, err := srtr.run(makeInputs(t,
			`package main

				const (
					//gofmts:sort
					// B = 1
					Z = 2
					A = 1
				)`))
		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, "block is unsorted", issues[0].Details())
		assert.Equal(t, 6, issues[0].Position().Line)
		require.Implements(t, (*IssueWithReplacement)(nil), issues[0])
		assert.Equal(t, "\tA = 1\n\tZ = 2\n",
			issues[0].(IssueWithReplacement).Replacement())
		assert.Equal(t, "block is unsorted at 6:6", issues[0].String())
	})

	t.Run("it works with top-level const", func(t *testing.T) {
		issues, err := srtr.run(makeInputs(t,
			`
package main

//gofmts:sort
const Z = 1
const A = 1

const B = 1
`))
		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, "block is unsorted", issues[0].Details())
		assert.Equal(t, 4, issues[0].Position().Line)
		require.Implements(t, (*IssueWithReplacement)(nil), issues[0])
		assert.Equal(t, "const A = 1\nconst Z = 2\n",
			issues[0].(IssueWithReplacement).Replacement())
		assert.Equal(t, "block is unsorted at 6:6", issues[0].String())
	})

	t.Run("it works back-to-back", func(t *testing.T) {
		issues, err := srtr.run(makeInputs(t,
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
		assert.Equal(t, "const A = 1\nconst Z = 2\n",
			issues[0].(IssueWithReplacement).Replacement())

		assert.Equal(t, "block is unsorted", issues[1].Details())
		assert.Equal(t, 7, issues[1].Position().Line)
		require.Implements(t, (*IssueWithReplacement)(nil), issues[1])
		assert.Equal(t, "const B = 1\nconst Y = 2\n",
			issues[1].(IssueWithReplacement).Replacement())
		assert.Equal(t, "block is unsorted at 7:14", issues[0].String())
	})

	t.Run("unused directive", func(t *testing.T) {
		issues, err := srtr.run(makeInputs(t,
			`package main

				//gofmts:sort
				`))
		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, "unused directive `gofmts:sort`", issues[0].Details())
		assert.Equal(t, 3, issues[0].Position().Line)
		assert.Equal(t, "unused directive `gofmts:sort` at 3:17", issues[0].String())
	})

	t.Run("unused directive due to whitespace", func(t *testing.T) {
		issues, err := srtr.run(makeInputs(t,
			`package main

				//gofmts:sort

				const A = 1
				`))
		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, "unused directive `gofmts:sort`", issues[0].Details())
		assert.Equal(t, 3, issues[0].Position().Line)
		assert.Equal(t, "unused directive `gofmts:sort` at 3:17", issues[0].String())
	})
}
