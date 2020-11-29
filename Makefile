SHELL=/bin/bash

test:
	diff <(go run ./cmd/gofmts ./example/example.go) ./example/example_expected.go
	go test ./...

lint:
	pre-commit run -a
