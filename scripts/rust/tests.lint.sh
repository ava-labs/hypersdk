#!/usr/bin/env bash
set -xue

if ! [[ "$0" =~ scripts/rust/tests.lint.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

# https://rust-lang.github.io/rustup/installation/index.html
# rustup toolchain install nightly --allow-downgrade --profile minimal --component clippy

rustup default stable

cargo fmt --all --verbose -- --check

rustup default nightly

# use rust nightly clippy to check tests and all crate features
cargo +nightly clippy --all --all-features -- -D warnings

rustup default stable

echo "ALL SUCCESS!"
