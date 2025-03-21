@echo off
echo Building githookkit for Windows...

echo Running unit tests with coverage...
go test -v -coverprofile=coverage.out ./...

echo Generating coverage report...
go tool cover -html=coverage.out -o coverage.html

@echo Building commit-received application...
@go build -o bin/commit-received.exe ./cmd/commit-received
echo Building ref-update application...
go build -o bin/ref-update.exe ./cmd/ref-update

echo Build completed successfully!
echo Coverage report available at: coverage.html
echo Executable available at: bin/