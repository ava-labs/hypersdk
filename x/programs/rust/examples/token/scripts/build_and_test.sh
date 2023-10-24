#!/usr/bin/env bash

set -euo pipefail

simulator_path="../../../cmd/simulator"
simulator_bin="${simulator_path}"/bin/simulator

echo "Building Simulator..."
go build -o "${simulator_bin}" "${simulator_path}"/simulator.go

# Set environment variables for the test

# The path to the simulator binary
export SIMULATOR_PATH="${simulator_bin}"

# The path to the compiled Wasm program to be tested
export PROGRAM_PATH="../../../examples/testdata/token.wasm"

echo "Running Simulator Tests..."

cargo test --package token --lib nocapture -- tests::test_token_plan --exact --nocapture --ignored
