#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -euo pipefail

# - Run e2e tests against nodes deployed to a kind cluster.
# - Expects a nix shell to ensure dependencies are available.
#
# TODO(marun)
# - Support testing against a remote cluster

# Ensure the morpheusvm root as the working directory
cd "$(dirname "${BASH_SOURCE[0]}")" && cd ..
MORPHEUS_ROOT="$(pwd)"
REPO_ROOT="$(cd ../../ && pwd)"

export KUBECONFIG="${KUBECONFIG:-$HOME/.kube/config}"

# Enable collector start if credentials are set in the env
if [[ -n "${PROMETHEUS_USERNAME:-}" ]]; then
  export TMPNET_START_COLLECTORS=true
fi

"${REPO_ROOT}"/bin/tmpnetctl start-kind-cluster

DOCKER_IMAGE="${DOCKER_IMAGE:-localhost:5001/morpheusvm}"
if [[ -z "${SKIP_BUILD_IMAGE:-}" ]]; then
  FORCE_TAG_LATEST=1 DOCKER_IMAGE="${DOCKER_IMAGE}" bash -x "${REPO_ROOT}"/scripts/build_docker_image.sh
fi

GINKGO_ARGS=()
# Reference: https://onsi.github.io/ginkgo/#spec-randomization
if [[ -n "${E2E_RANDOM_SEED:-}" ]]; then
  # Supply a specific seed to simplify reproduction of test failures
  GINKGO_ARGS+=(--seed="${E2E_RANDOM_SEED}")
else
  # Execute in random order to identify unwanted dependency
  GINKGO_ARGS+=(--randomize-all)
fi

"${REPO_ROOT}"/bin/ginkgo -v "${GINKGO_ARGS[@]}" "${MORPHEUS_ROOT}"/tests/e2e --\
  --runtime=kube --kube-image="${DOCKER_IMAGE}" "${@}"
