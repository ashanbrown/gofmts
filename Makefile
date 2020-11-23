SHELL=/bin/bash

test:
	diff <(go run . ./example/example.go) ./example/example_expected.go

lint:
	pre-commit run -a
