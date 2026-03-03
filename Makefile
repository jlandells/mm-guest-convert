BINARY := mm-guest-convert
VERSION ?= dev

LDFLAGS := -ldflags="-X main.version=$(VERSION)"

.PHONY: build build-all test test-cover test-race lint clean

build:
	go build $(LDFLAGS) -o bin/$(BINARY) .

build-all:
	GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY)-linux-amd64 .
	GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY)-darwin-amd64 .
	GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY)-windows-amd64.exe .

test:
	go test ./... -v

test-cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out

test-race:
	go test -race ./... -v

lint:
	go vet ./...

clean:
	rm -rf bin/ coverage.out
