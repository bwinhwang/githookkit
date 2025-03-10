#!/bin/bash
set -e

echo "Building githookkit for Linux..."

echo "Running unit tests with coverage..."
go test -v -coverprofile=coverage.out ./...

echo "Generating coverage report..."
go tool cover -html=coverage.out -o coverage.html

echo "Building commit-received application..."
mkdir -p bin
go build -o bin/commit-received ./cmd/commit-received

echo "Build completed successfully!"
echo "Coverage report available at: coverage.html"
echo "Executable available at: bin/commit-received"

# Make the binary executable
chmod +x bin/commit-received 