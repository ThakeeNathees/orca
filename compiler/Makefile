.PHONY: build test test-verbose test-cover coverage run clean fmt vet lint

APP_NAME := orca
BUILD_DIR := bin

build:
	go build -o $(BUILD_DIR)/$(APP_NAME) .

run: build
	./$(BUILD_DIR)/$(APP_NAME)

test:
	go test ./...

test-verbose:
	go test -v ./...

test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

fmt:
	go fmt ./...

vet:
	go vet ./...

lint: fmt vet

clean:
	rm -rf $(BUILD_DIR) coverage.out coverage.html
