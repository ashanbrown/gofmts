// +build !analyze

package example

//gofmts:sql
const Sql = `
SELECT * FROM mytable `

//gofmts:json
const Json = `
{"a":  1, "b":2, "c": [1, 2, 3]}`

//gofmts:sort
const Z = 1
const A = 2

const (
	//gofmts:sort
	// ignore this
	Z1 = 1
	A2 = 2
)

const (
	//gofmts:sort
	Z3 = 1
	// move this
	A3 = 2
)

//gofmts:go
const expr = "1 +  2"
