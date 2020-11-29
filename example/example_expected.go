// +build never
// +build !analyze

package example

//gofmts:sql
const Sql = `
		SELECT
		  *
		FROM
		  mytable
		`

//gofmts:json
const Json = `
		{
		  "a": 1,
		  "b": 2,
		  "c": [1, 2, 3]
		}
		`

//gofmts:sort
const A = 2
const Z = 1

const (
	//gofmts:sort
	// ignore this
	A2 = 2
	Z1 = 1
)

const (
	//gofmts:sort
	// move this
	A3 = 2
	Z3 = 1
)
