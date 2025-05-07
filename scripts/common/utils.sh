#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

# Function to check if the script is run from the repository root
function check_repository_root() {
  if ! [[ "$0" =~ $1 ]]; then
    echo "must be run from repository root"
    exit 255
  fi
}

function rm_previous_cov_reports() {
    rm -f integration.coverage.out
    rm -f integration.coverage.html
}

function add_license_headers() {
  local license_file="license-header.txt"
  if [[ ! -f "$license_file" ]]; then
    license_file="../../license-header.txt"
  fi

  local args=("-f" "$license_file")
  local action="adding"
  if [[ "$1" == "-check" ]]; then
    args+=("-check")
    action="checking"
  fi

  echo "${action} license headers"

  local repo_root
  repo_root="$(cd "$( dirname "${BASH_SOURCE[0]}" )" || exit; cd ../.. && pwd )"

  # Find and process files with the specified extensions
  find . -type f \( -name "*.go" -o -name "*.rs" -o -name "*.sh" \) -not -path "./target/*" -print0 | xargs -0 "${repo_root}"/bin/addlicense "${args[@]}"
}
