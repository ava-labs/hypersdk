#!/usr/bin/env bash

set -euo pipefail

if ! [[ "$0" =~ scripts/tests.simulator.sh ]]; then
  echo "must be run from csamm crate root"
  exit 255
fi

simulator_path="${PWD}"/../../../cmd/simulator
simulator_bin="${simulator_path}"/bin/simulator

# Set environment variables for the test

# The path to the simulator binary
export SIMULATOR_PATH="${simulator_bin}"

# The path to the compiled Wasm program to be tested
export TOKEN_PROGRAM_PATH="${PWD}"/../../build/token.wasm
export AMM_PROGRAM_PATH="${PWD}"/build/csamm.wasm

echo "Building CSAMM example..."
./../../scripts/build.sh

echo "Downloading dependencies..."
cd "${simulator_path}"
go mod download

echo "Building Simulator..."
go build -o "${simulator_bin}" "${simulator_path}"/simulator.go

echo "Running Simulator Tests..."

cd -
cargo test --lib -- --include-ignored
