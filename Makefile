SHELL=/bin/bash

test:
	diff <(go run ./cmd/gofmts ./example/example.go) ./example/example.go.golden
	go test ./...

lint:
	pre-commit run -a
