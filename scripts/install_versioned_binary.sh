#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -euo pipefail

function install_versioned_binary() {
  local repo_root="${1}"
  local binary_name="${2}"
  local binary_url="${3}"
  local version="${4}"

  # Install to central location to avoid having to rebuild across git worktrees or repos.
  local versioned_binary_root="${HOME}/.cache/versioned-binaries"

  local gobin="${versioned_binary_root}/${binary_name}/${version}"
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
