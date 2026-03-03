SHELL := /bin/bash

# Variables
APP_NAME=mm-guest-convert
VERSION := $(shell cat VERSION)

# Build all platforms
build-all: pre-build-check fmt imports staticcheck vet
	@echo "Building for all platforms..."
	@mkdir -p bin
	GOOS=linux GOARCH=amd64 go build -ldflags="-X 'main.Version=${VERSION}'" -o bin/$(APP_NAME)_linux_amd64
	GOOS=linux GOARCH=arm64 go build -ldflags="-X 'main.Version=${VERSION}'" -o bin/$(APP_NAME)_linux_arm64
	GOOS=darwin GOARCH=arm64 go build -ldflags="-X 'main.Version=${VERSION}'" -o bin/$(APP_NAME)_macos_apple
	GOOS=darwin GOARCH=amd64 go build -ldflags="-X 'main.Version=${VERSION}'" -o bin/$(APP_NAME)_macos_intel
	GOOS=windows GOARCH=amd64 go build -ldflags="-X 'main.Version=${VERSION}'" -o bin/$(APP_NAME)_windows.exe

.PHONY: fmt imports staticcheck vet build-all clean test test-cover

# Code quality checks
fmt:
	@echo "Running gofmt..."
	@gofmt -d -e -s . 2>&1 | read; if [ $$? -eq 0 ]; then echo "Code is not formatted, please run 'gofmt -w .'" && exit 1; fi

imports:
	@echo "Running goimports..."
	@goimports -l . 2>&1 | read; if [ $$? -eq 0 ]; then echo "Imports are not properly organized, please run 'goimports -w .'" && exit 1; fi

staticcheck:
	@echo "Running staticcheck..."
	@staticcheck ./... || (echo "Staticcheck identified problems" && exit 1)

vet:
	@echo "Running go vet..."
	@go vet ./... || (echo "Go vet identified problems" && exit 1)

# Pre-build check to ensure version tag does not already exist
pre-build-check:
	@echo "Fetching remote tags..."
	@git fetch --tags
	@if git tag -l "$(VERSION)" | grep -q .; then \
		echo "Error: Tag $(VERSION) already exists. Please update the VERSION file."; \
		exit 1; \
	else \
		echo "Tag $(VERSION) does not exist. Proceeding with the build."; \
	fi

# Tests
test:
	go test ./... -v

test-cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out

# Clean up
clean:
	@echo "Cleaning up..."
	rm -rf bin/
