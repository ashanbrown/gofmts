package example

//go:generate sh -c "echo '// +build never' > example_expected.go"
//go:generate sh -c "go run ../cmd/gofmts < example.go >> example_expected.go"
