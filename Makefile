SHELL=/bin/bash

test:
	diff <(go run ./cmd/gofmts ./example/example.go) ./example/example.go.golden
	rm ./example/example_check.go; \
		cp ./example/example.go ./example/example_check.go; \
		go run ./cmd/gofmts-check -fix ./example/example_check.go || true; \
		diff -dw ./example/example_check.go ./example/example.go.golden
	go test ./...

lint:
	pre-commit run -a
