#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -euo pipefail

REPO_ROOT=$(cd "$( dirname "${BASH_SOURCE[0]}" )"; cd .. && pwd )

function install_versioned_binary() {
  local repo_root="${1}"
  local binary_name="${2}"
  local binary_url="${3}"
  local version="${4}"

  # Install to central location to avoid having to rebuild across git worktrees or repos.
  local versioned_binary_root="${HOME}/.cache/versioned-binaries"

  local gobin="${versioned_binary_root}/${binary_url}/${version}"
  local binary_file="${gobin}/${binary_name}"

  # Check if the binary exists for the current version
  if [[ ! -f "${binary_file}" ]]; then
    echo "installing ${binary_name} @ ${version}"
    GOBIN="${gobin}" go install "${binary_url}@${version}"

    # Rename the binary if the package name doesn't match
    local package_name
    package_name="$(basename "${binary_url}")"
    if [[ "${package_name}" != "${binary_name}" ]]; then
      mv "${gobin}/${package_name}" "${binary_file}"
    fi
  fi

  # Symlink the binary to the local path to ensure a stable target for the caller
  mkdir -p "${repo_root}/build"
  ln -sf "${binary_file}" "${repo_root}/build/${binary_name}"
}

# The first 2 arguments are for this script, the rest are for invoking binary
BINARY_URL="${1}"
VERSION="${2}"
shift 2

if [[ -z "${BINARY_URL}" ]]; then
  echo "Usage: $0 <binary_url> <version> [<args>]"
  echo "binary_url is required"
  exit 1
fi

if [[ -z "${VERSION}" || "${VERSION}" == "latest" ]]; then
  echo "Usage: $0 <binary_url> <version> [<args>]"
  echo "version is required and cannot be 'latest'"
  exit 1
fi

# The binary name is the last part of the URL, or can be overridden by the caller
BINARY_NAME="${GOLANG_BINARY_NAME:-$(basename "${BINARY_URL}")}"

install_versioned_binary "${REPO_ROOT}" "${BINARY_NAME}" "${BINARY_URL}" "${VERSION}"

"${REPO_ROOT}/build/${BINARY_NAME}" "${@}"
