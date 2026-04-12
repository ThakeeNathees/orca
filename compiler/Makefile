# Orca compiler — local build and quality targets.
# Run from compiler/ (see CLAUDE.md). Produces bin/orca.

.PHONY: build test test-v coverage run clean fmt vet lint golden

APP_NAME := orca
BUILD_DIR := bin

# Compile the orca CLI into $(BUILD_DIR)/$(APP_NAME).
build:
	go build -o $(BUILD_DIR)/$(APP_NAME) .

# Build, then run the binary.
run: build
	./$(BUILD_DIR)/$(APP_NAME)

# Run all package tests.
test:
	go test ./...

# Same as test but with verbose test output.
test-v:
	go test -v ./...

# Generate coverage.out and openable coverage.html in this directory.
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

fmt:
	go fmt ./...

vet:
	go vet ./...

# Format and vet; use before committing.
lint: fmt vet

# Regenerate codegen golden files from current compiler output.
golden:
	go test ./codegen/ -run TestGoldenFiles -update-golden

# Remove build artifact and coverage outputs.
clean:
	rm -rf $(BUILD_DIR) coverage.out coverage.html
