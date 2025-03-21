#!/bin/bash
set -e

echo "Building githookkit for Linux..."

echo "Running unit tests with coverage..."
go test -v -coverprofile=coverage.out ./...

echo "Generating coverage report..."
go tool cover -html=coverage.out -o coverage.html


mkdir -p bin

echo "Building ref-update application..."
CGO_ENABLED=0 go build -o bin/ref-update ./cmd/ref-update

echo "Build completed successfully!"
echo "Coverage report available at: coverage.html"
echo "Executables available at: bin/"

# Make the binary executable
chmod +x bin/commit-received
chmod +x bin/ref-update  