#!/usr/bin/env bash

set -x

CARGO_TARGET_DIR=../examples/testdata/target

rm -r $CARGO_TARGET_DIR

cargo build --target wasm32-wasi --target-dir $CARGO_TARGET_DIR --release
