package gofmts

import (
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatter(t *testing.T) {
	makeInputs := func(t *testing.T, src string) ([]byte, *token.FileSet, *ast.File) {
		fset := token.NewFileSet()
		src = strings.TrimLeftFunc(src, unicode.IsSpace)
		formatted, err := format.Source([]byte(src))
		require.NoError(t, err)
		f, err := parser.ParseFile(fset, "", string(formatted), parser.ParseComments)
		require.NoError(t, err)
		return formatted, fset, f
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
		require.Implements(t, (*IssueWithReplacement)(nil), issues[0])
		assert.Equal(t, "`\n\t\t{\n\t\t  \"a\": 1\n\t\t}\n\t\t`",
			issues[0].(IssueWithReplacement).Replacement())
	})

	t.Run("inline directive", func(t *testing.T) {
		issues, err := fmtr.Run(makeInputs(t,
			`package main

				const json = `+"`{\"a\":    1}`"+`//gofmts:json`))
		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, "json formatting differs", issues[0].Details())
		assert.Equal(t, 3, issues[0].Position().Line)
		require.Implements(t, (*IssueWithReplacement)(nil), issues[0])
		assert.Equal(t, "`\n\t\t{\n\t\t  \"a\": 1\n\t\t}\n\t\t`",
			issues[0].(IssueWithReplacement).Replacement())
	})

	t.Run("a directive works on a function argument", func(t *testing.T) {
		issues, err := fmtr.Run(makeInputs(t,
			`package main

				var _ = run(
					//gofmts:json
					`+"`[1 ]`)"))
		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, "json formatting differs", issues[0].Details())
		assert.Equal(t, 5, issues[0].Position().Line)
		require.Implements(t, (*IssueWithReplacement)(nil), issues[0])
		assert.Equal(t, "`\n\t\t[1]\n\t\t`", issues[0].(IssueWithReplacement).Replacement())
	})

	t.Run("json directive for invalid json generates an error", func(t *testing.T) {
		issues, err := fmtr.Run(makeInputs(t,
			`package main

				//gofmts:json
				const json = `+"`{noquotes: \"1\"}`"))
		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, `failed directive "json": json is not valid`, issues[0].Details())
		assert.Equal(t, 3, issues[0].Position().Line)
	})

	t.Run("wrong quotes for multiline string generates an error", func(t *testing.T) {
		issues, err := fmtr.Run(makeInputs(t,
			//gofmts:go
			`
				package main
				
				//gofmts:sql
				const sql = "SELECT * FROM mytable"
				`))
		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, `failed directive "sql": reformatted string will be multiline and must be quoted using backticks`, issues[0].Details())
		assert.Equal(t, 4, issues[0].Position().Line)
	})

	t.Run("sql directive reformats sql", func(t *testing.T) {
		issues, err := fmtr.Run(makeInputs(t,
			`package main

				//gofmts:sql
				const sql = `+"`select * from mytable`"))
		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, "sql formatting differs", issues[0].Details())
		assert.Equal(t, 4, issues[0].Position().Line)
		require.Implements(t, (*IssueWithReplacement)(nil), issues[0])
		assert.Equal(t, "`\n\t\tSELECT\n\t\t  *\n\t\tFROM\n\t\t  mytable\n\t\t`",
			issues[0].(IssueWithReplacement).Replacement())
	})

	t.Run("sql on line boundary uses tabs for indenting", func(t *testing.T) {
		issues, err := fmtr.Run(makeInputs(t,
			`package main

				var sql = ("" +
					//gofmts:sql
					`+"`SELECT * FROM mytable`)"))
		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, "sql formatting differs", issues[0].Details())
		assert.Equal(t, 5, issues[0].Position().Line)
		require.Implements(t, (*IssueWithReplacement)(nil), issues[0])
		assert.Equal(t, "`\n\t\tSELECT\n\t\t  *\n\t\tFROM\n\t\t  mytable\n\t\t`",
			issues[0].(IssueWithReplacement).Replacement())
	})

	t.Run("an unknown directive generates an error", func(t *testing.T) {
		//gofmts:go
		issues, err := fmtr.Run(makeInputs(t,
			`
				package main
				
				//gofmts:unknown
				const value = ""
				`))
		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, "unknown directive `gofmts:unknown`", issues[0].Details())
		assert.Equal(t, 3, issues[0].Position().Line)
	})

	t.Run("an unused directive generates an error", func(t *testing.T) {
		issues, err := fmtr.Run(makeInputs(t,
			//gofmts:go
			`
				package main
				
				//gofmts:sql
				`))
		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, "unused directive `gofmts:sql`", issues[0].Details())
		assert.Equal(t, 3, issues[0].Position().Line)
	})

	t.Run("go directive formats go code", func(t *testing.T) {
		issues, err := fmtr.Run(makeInputs(t,
			//gofmts:go
			`
				package main
				
				//gofmts:go
				const expr = "1  + 2"
				`))
		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, "go formatting differs", issues[0].Details())
		assert.Equal(t, 4, issues[0].Position().Line)
		require.Implements(t, (*IssueWithReplacement)(nil), issues[0])
		assert.Equal(t, `"1 + 2"`, issues[0].(IssueWithReplacement).Replacement())
	})

	t.Run("bad go code generates an error", func(t *testing.T) {
		issues, err := fmtr.Run(makeInputs(t,
			//gofmts:go
			`
				package main
				
				//gofmts:go
				const expr = "1 +"
				`))
		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, `failed directive "go": unable to format go code: 3:1: expected operand, found '}'`,
			issues[0].Details())
		assert.Equal(t, 3, issues[0].Position().Line)
	})
}
