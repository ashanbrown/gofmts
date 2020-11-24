package formatter

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatter(t *testing.T) {
	makeInputs := func(t *testing.T, src string) (*token.FileSet, *ast.File) {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "", src, parser.ParseComments)
		require.NoError(t, err)
		return fset, f
	}

	fmtr := NewFormatter()

	t.Run("previous directive", func(t *testing.T) {
		issues, err := fmtr.Run(makeInputs(t,
			`package main

				//gofmts:json
				const json = `+"`{\"a\":    1}`"))
		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, "json formatting differs", issues[0].Details())
		assert.Equal(t, 4, issues[0].Position().Line)
		assert.Equal(t, "`"+`
		  {
		    "a": 1
		  }
		  `+"`", *issues[0].Replacement())
		assert.Equal(t, "json formatting differs at 4:18", issues[0].String())
	})

	t.Run("inline directive", func(t *testing.T) {
		issues, err := fmtr.Run(makeInputs(t,
			`package main

				const json = `+"`{\"a\":    \"1\"}`"+`//gofmts:json`))
		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, "json formatting differs", issues[0].Details())
		assert.Equal(t, 3, issues[0].Position().Line)
		assert.Equal(t, "`"+`
		  {
		    "a": 1
		  }
		  `+"`", *issues[0].Replacement())
		assert.Equal(t, "json formatting differs at 3:18", issues[0].String())
	})

	t.Run("invalid json", func(t *testing.T) {
		issues, err := fmtr.Run(makeInputs(t,
			`package main

				//gofmts:json
				const json = `+"`{noquotes: \"1\"}`"))
		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, `failed directive "json": json is not valid`, issues[0].Details())
		assert.Equal(t, 3, issues[0].Position().Line)
		assert.Nil(t, issues[0].Replacement())
		assert.Equal(t, `failed directive "json": json is not valid at 3:48`, issues[0].String())
	})

	t.Run("sql directive", func(t *testing.T) {
		issues, err := fmtr.Run(makeInputs(t,
			`package main

				//gofmts:sql
				const sql = `+"`select * from mytable`"))
		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, "sql formatting differs", issues[0].Details())
		assert.Equal(t, 4, issues[0].Position().Line)
		assert.Equal(t, "`"+`
		 SELECT
		   *
		 FROM
		   mytable
		 `+"`", *issues[0].Replacement())
		assert.Equal(t, "sql formatting differs at 4:17", issues[0].String())
	})

	t.Run("unknown directive", func(t *testing.T) {
		issues, err := fmtr.Run(makeInputs(t,
			`package main

				//gofmts:unknown
				const value = ""
				`))
		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, "unknown directive `gofmts:unknown`", issues[0].Details())
		assert.Equal(t, 3, issues[0].Position().Line)
		assert.Nil(t, issues[0].Replacement())
		assert.Equal(t, "unknown directive `gofmts:unknown` at 3:21", issues[0].String())
	})

	t.Run("unused directive", func(t *testing.T) {
		issues, err := fmtr.Run(makeInputs(t,
			`package main

				//gofmts:sql
				`))
		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, "unused directive `gofmts:sql`", issues[0].Details())
		assert.Equal(t, 3, issues[0].Position().Line)
		assert.Nil(t, issues[0].Replacement())
		assert.Equal(t, "unused directive `gofmts:sql` at 3:17", issues[0].String())
	})
}
