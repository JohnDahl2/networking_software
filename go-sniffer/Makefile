.PHONY: build test lint race run

build:
	go build ./...

test:
	go test ./...

race:
	go test -race ./...

lint:
	golangci-lint run ./...

run:
	go run ./cmd/sniffer
