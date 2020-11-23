// +build !analyze

package example

//gofmts:sql
const Sql = `
	     select
	       *
	     from
	       mytable
	     `

//gofmts:json
const Json = `
	      {
	        "a": 1,
	        "b": 2
	      }
	      `
