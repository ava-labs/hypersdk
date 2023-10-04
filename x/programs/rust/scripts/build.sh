#!/usr/bin/env bash

set -x

# Set a default value for CARGO_TARGET_DIR if it's not provided as an argument
CARGO_TARGET_DIR="${1:-../examples/testdata/target}"

# Clean the cargo project
cargo clean

# Compile the Program into the target directory 
cargo build \
  --target wasm32-unknown-unknown \
  --target-dir "$CARGO_TARGET_DIR" \
  --release

cp "$CARGO_TARGET_DIR"/wasm32-unknown-unknown/release/*.wasm ../examples/testdata/

# cargo build \
#    --target wasm32-wasi \
#    --target-dir "$CARGO_TARGET_DIR" \
#    --release

# cp "$CARGO_TARGET_DIR"/wasm32-wasi/release/*.wasm ../examples/testdata/