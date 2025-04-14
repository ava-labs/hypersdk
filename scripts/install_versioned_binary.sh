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

  local binary_file="${REPO_ROOT}/build/${binary_name}"
  local version_file="${REPO_ROOT}/build/${binary_name}.version"

  # Check if the binary exists and was built for the current version
  local build_required=1
  if [[ -f "${binary_file}" && -f "${version_file}" ]]; then
    if [[ "$(cat "${version_file}")" == "${AVALANCHE_VERSION}" ]]; then
      build_required=
    fi
  fi

  if [[ -n "${build_required}" ]]; then
    echo "installing ${binary_name} @ ${AVALANCHE_VERSION}"
    GOBIN="${REPO_ROOT}"/build go install "${binary_url}@${AVALANCHE_VERSION}"
    local package_name
    package_name="$(basename "${binary_url}")"
    if [[ "${package_name}" != "${binary_name}" ]]; then
      mv ./build/"${package_name}" "${binary_file}"
    fi
    echo "${AVALANCHE_VERSION}" > "${version_file}"
  fi
}
