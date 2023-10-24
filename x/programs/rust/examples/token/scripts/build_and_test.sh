#!/usr/bin/env bash

set -euo pipefail

simulator_path="../../../cmd/simulator"
simulator_bin="${simulator_path}"/bin/simulator

echo "Building Simulator..."
go build -o "${simulator_bin}" "${simulator_path}"/simulator.go

export SIMULATOR_PATH="${simulator_bin}"
export PROGRAM_PATH="../../../examples/testdata/token.wasm"

echo "Running Simulator Tests..."

cargo test --package token --lib nocapture -- tests::test_token_plan --exact --nocapture --ignored
