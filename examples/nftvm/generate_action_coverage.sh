#!/bin/bash

# Run tests with coverage
go test ./... -coverprofile=coverage.out

# Generate an HTML report
go tool cover -html=coverage.out -o coverage.html

# Open the HTML report (this will work on macOS, for Linux or Windows, use the appropriate command to open a file)
open coverage.html