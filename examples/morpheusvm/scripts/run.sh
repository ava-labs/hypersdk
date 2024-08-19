#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -e

# to run E2E tests (terminates cluster afterwards)
# MODE=test ./scripts/run.sh
MODE=${MODE:-run}

WORK_DIR="${GOPATH}"/src/github.com/ava-labs/hypersdk
MORPHEUSVM_DIR=${WORK_DIR}/examples/morpheusvm

cd "$MORPHEUSVM_DIR"

############################
# from constants.sh
export CGO_CFLAGS="-O -D__BLST_PORTABLE__"

# from utils.sh
function check_command() {
	if ! which "$1" &>/dev/null; then
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

############################

VERSION=d729e5c7ef9f008c3e89cd7131148ad3acda2e34

############################
# build avalanchego
# https://github.com/ava-labs/avalanchego/releases
TMPDIR=/tmp/hypersdk

echo "working directory: $TMPDIR"

AVALANCHEGO_PATH=${TMPDIR}/avalanchego-${VERSION}/avalanchego
AVALANCHEGO_PLUGIN_DIR=${TMPDIR}/avalanchego-${VERSION}/plugins

if [ ! -f "$AVALANCHEGO_PATH" ]; then
	echo "building avalanchego"
	CWD=$(pwd)

	# Clear old folders
	rm -rf "${TMPDIR}"/avalanchego-"${VERSION}"
	mkdir -p "${TMPDIR}"/avalanchego-"${VERSION}"
	rm -rf "${TMPDIR}"/avalanchego-src
	mkdir -p "${TMPDIR}"/avalanchego-src

	# Download src
	cd "${TMPDIR}"/avalanchego-src
	git clone https://github.com/ava-labs/avalanchego.git
	cd avalanchego
	git checkout "${VERSION}"

	# Build avalanchego
	./scripts/build.sh
	mv build/avalanchego "${TMPDIR}"/avalanchego-"${VERSION}"

	cd "${CWD}"
else
	echo "using previously built avalanchego"
fi

############################

echo "building morpheusvm"

# delete previous (if exists)
rm -f "${TMPDIR}"/avalanchego-"${VERSION}"/plugins/qCNyZHrs3rZX458wPJXPJJypPf6w423A84jnfbdP2TPEmEE9u

# rebuild with latest code
go build \
	-o "${TMPDIR}"/avalanchego-"${VERSION}"/plugins/qCNyZHrs3rZX458wPJXPJJypPf6w423A84jnfbdP2TPEmEE9u \
	"${MORPHEUSVM_DIR}"/cmd/morpheusvm

############################
echo "building e2e.test"

prepare_ginkgo

ACK_GINKGO_RC=true ginkgo build "${MORPHEUSVM_DIR}"/tests/e2e
"${MORPHEUSVM_DIR}"/tests/e2e/e2e.test --help

additional_args=("$@")

if [[ ${MODE} == "run" ]]; then
	echo "applying ginkgo.focus=Ping and --reuse-network to setup local network"
	additional_args+=("--ginkgo.focus=Ping")
	additional_args+=("--reuse-network")
fi

echo "running e2e tests"
"${MORPHEUSVM_DIR}"/tests/e2e/e2e.test \
	--ginkgo.v \
	--avalanchego-path="${AVALANCHEGO_PATH}" \
	--plugin-dir="${AVALANCHEGO_PLUGIN_DIR}" \
	"${additional_args[@]}"
