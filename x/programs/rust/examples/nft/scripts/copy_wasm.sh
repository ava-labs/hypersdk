#!/usr/bin/env bash

set -euo pipefail

if ! [[ "$0" =~ scripts/copy_wasm.sh ]]; then
  echo "must be run from crate root"
  exit 255
fi

root="$(pwd)"

# Build the program
build_script_path="${root}"/../../scripts/build.sh
sh "${build_script_path}" out

# Copy wasm file over
cp out/nft.wasm "${root}"/../examples/testdata/nft.wasm

# Delete build artifacts
rm -rf "${root}"/out

echo 'Successfully copied nft.wasm to examples/testdata'
