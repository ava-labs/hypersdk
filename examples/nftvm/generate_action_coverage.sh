#!/bin/bash

# Currently set up for checking unit test coverage

# Run tests with coverage
go test ./actions -coverprofile=coverage.out -coverpkg=./actions 

# Generate an HTML report
go tool cover -html=coverage.out -o coverage.html

# Open the HTML report (this will work on macOS, for Linux or Windows, use the appropriate command to open a file)
open coverage.html