#!/usr/bin/env bash
#
# $ DEBUG=1 ./scripts/build.sh .
#
# If DEBUG=1 is passed we compile the program with wasi support for
# easier debugging.
set -x

target="wasm32-unknown-unknown"
if [ -n "${DEBUG:-}" ]; then
  target="wasm32-wasi"
fi

# Set a default value for the cargo target dir if it's not provided as the first
# argument.
target_dir="${1:-build}"

# Clean the cargo project
cargo clean

# Compile the Program into the target directory 
cargo build \
  --target "${target}" \
  --target-dir "${target_dir}" \
  --release

# Copy the compiled program into the build directory
cp ./"${target_dir}"/"${target}"/release/*.wasm "./${target_dir}/"

echo "Wasm files are in ./${target_dir}"
