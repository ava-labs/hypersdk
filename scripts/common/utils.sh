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

# Function to check if the script is run from the repository root
function check_repository_root() {
  if ! [[ "$0" =~ $1 ]]; then
    echo "must be run from repository root"
    exit 255
  fi
}

function prepare_ginkgo() {
  set -e

  # to install the ginkgo binary (required for test build and run)
  go install -v github.com/onsi/ginkgo/v2/ginkgo@v2.13.1 || true

  # alert the user if they do not have $GOPATH properly configured
  check_command ginkgo
}

function rm_previous_cov_reports() {
    rm -f integration.coverage.out
    rm -f integration.coverage.html
}
