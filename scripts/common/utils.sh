#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

function check_command() {
  if ! which "$1" &> /dev/null
  then
      echo -e "\033[0;31myour golang environment is misconfigued...please ensure the golang bin folder is in your PATH\033[0m"
      echo -e "\033[0;31myou can set this for the current terminal session by running \"export PATH=\$PATH:\$(go env GOPATH)/bin\"\033[0m"
      exit
  fi
}

function install_if_not_exists() {
  if ! command -v "$1" &> /dev/null
  then
    echo "$1 not found, installing..."
    go install -v "$2"
  fi

  # alert the user if they do not have $GOPATH properly configured
  check_command "$1"
}

# Function to check if the script is run from the repository root
function check_repository_root() {
  if ! [[ "$0" =~ $1 ]]; then
    echo "must be run from repository root"
    exit 255
  fi
}

function prepare_ginkgo() {
  set -e

  install_if_not_exists ginkgo github.com/onsi/ginkgo/v2/ginkgo@v2.13.1
}

function rm_previous_cov_reports() {
    rm -f integration.coverage.out
    rm -f integration.coverage.html
}

function add_license_headers() {
  install_if_not_exists addlicense github.com/google/addlicense@latest
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

  # Find and process files with the specified extensions
  find . -type f \( -name "*.go" -o -name "*.rs" -o -name "*.sh" \) -print0 | xargs -0 addlicense "${args[@]}"
}
