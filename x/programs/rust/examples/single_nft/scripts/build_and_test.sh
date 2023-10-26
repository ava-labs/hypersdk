#!/usr/bin/env bash

set -euo pipefail

if ! [[ "$0" =~ scripts/build_and_test.sh ]]; then
  echo "must be run from crate root"
  exit 255
fi

root="$(pwd)"

simulator_path="${PWD}"/../../../cmd/simulator
simulator_bin="${simulator_path}"/bin/simulator

echo "Downloading dependencies..."
cd "${simulator_path}"
go mod download

echo "Building Simulator..."
go build -o "${simulator_bin}" "${simulator_path}"/simulator.go

# Set environment variables for the test

# The path to the simulator binary
export SIMULATOR_PATH="${simulator_bin}"

# The path to the compiled Wasm program to be tested
export PROGRAM_PATH="${root}"/../examples/testdata/single_nft.wasm

echo "Running Simulator Tests..."

cargo test -p single_nft
