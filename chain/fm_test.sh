#!/usr/bin/env bash
go test -coverprofile=coverage.out
go tool cover -html=coverage.out