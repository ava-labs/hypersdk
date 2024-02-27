#!/usr/bin/env bash

set -euo pipefail

if ! [[ "$0" =~ scripts/tests.simulator.sh ]]; then
  echo "must be run from counter crate root"
  exit 255
fi

simulator_path="${PWD}"/../../../cmd/simulator_api
simulator_bin="${simulator_path}"/bin/simulator

# Set environment variables for the test

# The path to the simulator binary
export SIMULATOR_PATH="${simulator_bin}"

# The path to the compiled Wasm program to be tested
export PROGRAM_PATH="${PWD}"/build/counter.wasm

echo "Building Counter example..."
./../../scripts/build.sh

example_path=$(pwd)
echo "Downloading dependencies..."
cd "${simulator_path}"
go mod download

echo "Building Simulator..."
go build -o "${simulator_bin}" "${simulator_path}"

echo "Running Simulator Tests..."
cd "${example_path}"
cargo test -- --include-ignored --nocapture
