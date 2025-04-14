#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -euo pipefail

function install_versioned_binary() {
  local repo_root="${1}"
  local binary_name="${2}"
  local binary_url="${3}"
  local version="${4}"

  local binary_file="${repo_root}/build/${binary_name}"
  local version_file="${repo_root}/build/${binary_name}.version"

  # Check if the binary exists and was built for the current version
  if [[ -f "${binary_file}" && -f "${version_file}" ]]; then
    if [[ "$(cat "${version_file}")" == "${version}" ]]; then
      return
    fi
  fi

  echo "installing ${binary_name} @ ${version}"
  GOBIN="${repo_root}"/build go install "${binary_url}@${version}"

  # Rename the binary if the package name doesn't match
  local package_name
  package_name="$(basename "${binary_url}")"
  if [[ "${package_name}" != "${binary_name}" ]]; then
    mv ./build/"${package_name}" "${binary_file}"
  fi

  echo "${version}" > "${version_file}"
}
