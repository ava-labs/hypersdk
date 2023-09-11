#!/usr/bin/env bash

set -x

CARGO_TARGET_DIR=../examples/testdata/target

cargo build --target wasm32-unknown-unknown --target-dir $CARGO_TARGET_DIR --release
