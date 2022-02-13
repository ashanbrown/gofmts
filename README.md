# gofmts formats strings in go code

![Test](https://github.com/ashanbrown/gofmts/workflows/test/badge.svg)

`gofmts` allows you to be opinionated about the format of strings and ordering of lines in your go source code.

**What can you do with `gofmts`?**

1. *You can standardize strings from other languages embedded in your code.*

`gofmts` supports `sql`, `json` and `go` itself as embedded languages.  For example,

    //gofmts:sql
    query := `
         SELECT
           *
         FROM
           mytable
    `

or

    //gofmts:json
    numbers := `[1, 2, 3]`

or

    //gofmts:go
    expr := `x := 1"


2. *You can keep groups of lines sorted alphabetically in your programs.*

You can use the `//gofmts:sort` directive to ensure groups of lines stay lexicographic order:

    const (
        //gofmts:sort
        X = 1
        Y = 2

        //gofmts:sort
        A = 1
        B = 2
    )

**Why do you care?**

`go` is an opinionated language but when embedding strings from other languages, it can become a free-for-all.  This tool attempts to solve that problem by ensuring that strings look the same, no matter who writes them, in which editor.  To make this as painless as possible, `gofmts` fixes the code rather than just reporting that it violates the standard.

## Running gofmts

You can run `gofmts` on specific files as part of your `generate` step:

    //go:generate gofmts -w $GOFILE

You can also run it on all your files via your pre-commit.com pre-commit hooks by putting this in your `.pre-commit-config.yaml`:

```
   - repo: github.com/ashanbrown/gofmts
     rev: v0.1.2
     hooks:
        - id: gofmts-docker
```

## Exported Analyzers for use with `go/aanalysis`.

In `pkg/analyzers`, both a `SortAnalyzer` and `FormatAnalyzer` are exported.  These implement the [`Analyzer` interface](https://pkg.go.dev/golang.org/x/tools/go/analysis#hdr-Analyzer) from the [`go/analysis` package](https://pkg.go.dev/golang.org/x/tools/go/analysis).  Because these the analyzer interface does not provide the source code with the formatter, the indent positioning of a formatted string may differ.

## Golangci-lint Plugin

A [plugin](./golangci-lint/plugin.go) is provided for use with the [Golangci-lint metalinter](https://github.com/golangci/golangci-lint).  Beacuse `SuggestedFixes` has not been implemented yet in golangci-lint, the plugin can only report errors.  It can be configured as follows:

```yaml
linters-settings:
  custom:
    gofmts:
      path: golangci-lint/plugin.so
      description: gofmts
      original-url: github.com/ashanbrown/gofmts
```

## Notes

Format directives in `gofmts` have two goals:

1. Pretty-printing your embedded code.
2. Indenting your embedded code for easier readability.

Right now `gofmts` is more interested in being opinionated than being pretty.  Pretty can come later.

## Technical details

`gofmts` works at the AST level, which means a couple of things:
1. We have to rewrite from the AST to generate the replacement text.  This could potentially lead to surprises if the generated code isn't identical to the input code.  Code run through gofmt first should generally be rewritten the same as it arrived.
2. For sorting, we sort AST nodes, assuming one per line.  In the future, we might be able to sort other list of nodes such as slices that fit within a line.

`gofmts` is written as a linter that returns issues so that it can one day be added as a linter/fixer combination to `golangci-lint`.

## Future plans

Some possible future work includes:

1. Allowing for customization of string formatting for different languages.
2. Support for more embedded languages (yaml?).
3. Support for custom string formatters.
4. Support for sorting using other criteria (RHS value?).
