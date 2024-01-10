#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

HYPERSDK_PATH=$(
    cd "$(dirname "${BASH_SOURCE[0]}")"
    cd .. && pwd
)

source $HYPERSDK_PATH/scripts/common/utils.sh

set -xue

check_repository_root scripts/tests.lint.sh

# https://rust-lang.github.io/rustup/installation/index.html
# rustup toolchain install nightly --allow-downgrade --profile minimal --component clippy

rustup default stable

cargo fmt --all --verbose -- --check

rustup default nightly

# use rust nightly clippy to check tests and all crate features
cargo +nightly clippy --all --all-features -- -D warnings

rustup default stable

echo "ALL SUCCESS!"
