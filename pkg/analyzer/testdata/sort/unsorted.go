package sort

//gofmts:sort
const Z = 1 // want "block is unsorted"
const A = 2

const (
	//gofmts:sort
	// ignore this
	Z1 = 1 // want "block is unsorted"
	A2 = 2
)

const (
	//gofmts:sort
	Z3 = 1 // want "block is unsorted"
	// move this
	A3 = 2
)
