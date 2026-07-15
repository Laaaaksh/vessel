.PHONY: build run test lint tidy clean

BINARY := vessel
MODULE := github.com/Laaaaksh/vessel

build:
	go build -o $(BINARY) .

run:
	go run .

test:
	go test ./... -race -cover

lint:
	golangci-lint run ./...

tidy:
	go mod tidy

clean:
	rm -f $(BINARY)
	go clean -testcache
