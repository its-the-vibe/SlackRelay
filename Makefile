.PHONY: build test lint ci clean

BINARY_NAME=slack-relay

build:
	go build -o $(BINARY_NAME) .

test:
	go test ./...

lint:
	go vet ./...
	@test -z "$$(gofmt -l .)" || (echo "The following files need gofmt:"; gofmt -l .; exit 1)

ci: lint build test

clean:
	rm -f $(BINARY_NAME)
