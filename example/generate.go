package example

//go:generate sh -c "echo '//go:build ignore' > example.go.golden"
//go:generate sh -c "go run ../cmd/gofmts < example.go >> example.go.golden"
