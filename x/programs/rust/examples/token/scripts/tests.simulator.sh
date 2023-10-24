#!/usr/bin/env bash

set -euo pipefail

if ! [[ "$0" =~ scripts/tests.simulator.sh ]]; then
  echo "must be run from token crate root"
  exit 255
fi

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
