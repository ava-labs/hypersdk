#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -euo pipefail

check_repository_root scripts/tests.simulator.sh

simulator_path="${PWD}"/../../../cmd/simulator
simulator_bin="${simulator_path}"/bin/simulator

# Set environment variables for the test

# The path to the simulator binary
export SIMULATOR_PATH="${simulator_bin}"

# The path to the compiled Wasm program to be tested
export PROGRAM_PATH="${PWD}"/../../../examples/testdata/token.wasm

echo "Downloading dependencies..."
cd "${simulator_path}"
go mod download

echo "Building Simulator..."
go build -o "${simulator_bin}" "${simulator_path}"/simulator.go

echo "Running Simulator Tests..."

cargo test --lib -- --include-ignored
