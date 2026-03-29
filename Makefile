APP := easy-cmd

.PHONY: build test

build:
	go build -o tmp/ ./cmd/easy-cmd

test:
	go test ./...
