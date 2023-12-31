
.PHONY: all generate build run

all: generate build test

generate:
	echo "Generating..."
	go generate ./...

build:
	echo "Building..."
	go build -o bin/compiler ./cmd/compiler
	go build -o bin/executor ./cmd/executor
	go vet ./...
	go install honnef.co/go/tools/cmd/staticcheck@latest
	staticcheck ./... || true

test:
	echo "Testing..."
	go test -v ./...

run-executor:
	./bin/executor -v=3
