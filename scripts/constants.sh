#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.
# Ignore warnings about variables appearing unused since this file is not the consumer of the variables it defines.
# shellcheck disable=SC2034

set -euo pipefail

AVALANCHE_VERSION=${AVALANCHE_VERSION:-'13c08681c17d0790a94ed8c8ef8a3c88f8bb196d'}

# Set the PATHS
GOPATH="$(go env GOPATH)"
DEFAULT_VM_ID="pkEmJQuTUic3dxzg8EYnktwn4W7uCHofNcwiYo458vodAUbY7"

# Avalabs docker hub
# avaplatform/avalanchego - defaults to local as to avoid unintentional pushes
# You should probably set it - export DOCKER_REPO='avaplatform/subnet-evm'
DOCKERHUB_REPO=${DOCKER_REPO:-"morpheusvm"}

# Shared between ./scripts/build_docker_image.sh
AVALANCHEGO_IMAGE_NAME="${AVALANCHEGO_IMAGE_NAME:-avaplatform/avalanchego}"

# set git constants if available, otherwise set to empty string (say building from a release)
if git rev-parse --is-inside-work-tree > /dev/null 2>&1; then
        # Current branch
    CURRENT_BRANCH=${CURRENT_BRANCH:-$(git describe --tags --exact-match 2>/dev/null || git symbolic-ref -q --short HEAD || git rev-parse --short HEAD || :)}

    # Image build id
    #
    # Use an abbreviated version of the full commit to tag the image.
    # WARNING: this will use the most recent commit even if there are un-committed changes present
    VM_COMMIT="$(git rev-parse HEAD || :)"
else
    CURRENT_BRANCH=""
    VM_COMMIT=""
fi

# Shared between ./scripts/build_docker_image.sh
DOCKERHUB_TAG=${VM_COMMIT::8}

echo "Using branch: ${CURRENT_BRANCH}"

# Static compilation
STATIC_LD_FLAGS=''
if [ "${STATIC_COMPILATION:-}" = 1 ]; then
    export CC=musl-gcc
    command -v $CC || (echo $CC must be available for static compilation && exit 1)
    STATIC_LD_FLAGS=' -extldflags "-static" -linkmode external '
fi

# Set the CGO flags to use the portable version of BLST
#
# We use "export" here instead of just setting a bash variable because we need
# to pass this flag to all child processes spawned by the shell.
export CGO_CFLAGS="-O2 -D__BLST_PORTABLE__"
