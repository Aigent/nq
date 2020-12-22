THIS_MAKEFILE_PATH := $(abspath $(lastword $(MAKEFILE_LIST)))
THIS_MAKEFILE_DIR := $(patsubst %/,%,$(dir $(THIS_MAKEFILE_PATH)))


all: build

build:
	go build ./...

golangci-lint:
	GOBIN="$(THIS_MAKEFILE_DIR)" go install github.com/golangci/golangci-lint/cmd/golangci-lint
	go mod tidy

lint: golangci-lint
	./golangci-lint run ./...

# regex to pick a tests (make test T='TestOnlyThis*')
# match everything by default
T?=.*
test:
	go test -run "$(T)" -count=1 -race -timeout 5m ./...

# regex to pick a benchmarks (make bench B='BenchmarkOnlyThis*')
# match everything by default
B?=.*
bench:
	go test -bench "$(B)" -benchmem -run 'noTestsPlease' ./... -v

clean:
	rm -f bin/*

.PHONY: all build lint test clean
