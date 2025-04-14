#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -euo pipefail

REPO_ROOT=$(cd "$( dirname "${BASH_SOURCE[0]}" )"; cd .. && pwd )

# Set AVALANCHE_VERSION and ensure CGO is configured
. "${REPO_ROOT}"/scripts/constants.sh

function install_versioned_binary() {
  local binary_name="${1}"
  local binary_url="${2}"

  # Check if the binary exists and was built for the current version
  local build_required=1
  if [[ -f "${REPO_ROOT}/build/${binary_name}" && -f "${REPO_ROOT}/build/${binary_name}.version" ]]; then
    if [[ "$(cat "${REPO_ROOT}/build/${binary_name}.version")" == "${AVALANCHE_VERSION}" ]]; then
      build_required=
    fi
  fi

  if [[ -n "${build_required}" ]]; then
    cd "${REPO_ROOT}"
    echo "installing ${binary_name} @ ${AVALANCHE_VERSION}"
    GOBIN="${REPO_ROOT}"/build go install "${binary_url}@${AVALANCHE_VERSION}"
    local package_name=$(basename "${binary_url}")
    if [[ "${package_name}" != "${binary_name}" ]]; then
      mv ./build/"${package_name}" ./build/"${binary_name}"
    fi
    echo "${AVALANCHE_VERSION}" > "${REPO_ROOT}"/build/"${binary_name}".version
  fi
}
