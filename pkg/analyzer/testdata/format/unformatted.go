package format

//gofmts:sql
const Sql = /* want "sql formatting differs" */ `SELECT * FROM mytable `

//gofmts:json
const Json = /* want "json formatting differs" */ `{"a":  1, "b":2, "c": [1, 2, 3]}`

//gofmts:go
const expr = "1 +  2" // want "go formatting differs"
