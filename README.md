# gofmts formats strings in go programs

## Usage

    //gofmts:sql
    x := `SELECT *    FROM    mytable`

or

    //gofmts:json
    x := `{x:    1}`

It is recommended to use this with `go:generate` by adding this to any file that needs gofmts and calling
`go generate ./...` on your project.  For any file containing `gofmts`, you'd add:

    //go:generate gofmts -i $GOFILE
