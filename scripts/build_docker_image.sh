#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -euo pipefail

# e.g.,
# ./scripts/build_docker_image.sh                                                           # Build local image
# ./scripts/build_docker_image.sh --no-cache                                                # All script arguments are provided to `docker buildx build`
# DOCKER_IMAGE=mymorpheusvm ./scripts/build_docker_image.sh                                 # Build local image with a custom image name
# BUILD_MULTI_ARCH=1 DOCKER_IMAGE=avaplatform/morpheusvm ./scripts/build_docker_image.sh    # Build and push multi-arch image to docker hub
# FORCE_TAG_LATEST=1 DOCKER_IMAGE=localhost:5001/morpheusvm ./scripts/build_docker_image.sh # Build and push image to private registry with tag `latest`

# Directory above this script
HYPERSDK_PATH=$(
  cd "$(dirname "${BASH_SOURCE[0]}")"
  cd .. && pwd
)

# Force tagging as latest even if not the master branch
FORCE_TAG_LATEST="${FORCE_TAG_LATEST:-}"

source "$HYPERSDK_PATH"/scripts/constants.sh
source "$HYPERSDK_PATH"/scripts/git_commit.sh
source "$HYPERSDK_PATH"/scripts/image_tag.sh

# Configure image build for the specified VM.
VM_NAME="${VM_NAME:-${DEFAULT_VM_NAME}}"
VM_ID="${VM_ID:-${DEFAULT_VM_ID}}"
VM_PATH="${VM_PATH:-${HYPERSDK_PATH}/examples/${VM_NAME}}"

# The published name should be 'avaplatform/[vm name]', but to avoid unintentional pushes
# it is defaulted to '[vm name]' (without a registry or namespace) which will only build a
# local image.
#
# Notes:
# - The format for a docker image name is [registry/][namespace/]repository[:tag][@digest]
# - DOCKER_IMAGE is intended to include [registry/][namespace/]repository. The tag is
#   determined automatically based on the local git state.
DOCKER_IMAGE="${DOCKER_IMAGE:-${VM_NAME}}"

# If set to non-empty, prompts the building of a multi-arch image when the image
# name indicates use of a registry.
#
# A registry is required to build a multi-arch image since a multi-arch image is
# not really an image at all. A multi-arch image (also called a manifest) is
# basically a list of arch-specific images available from the same registry that
# hosts the manifest. Manifests are not supported for local images.
#
# Reference: https://docs.docker.com/build/building/multi-platform/
BUILD_MULTI_ARCH="${BUILD_MULTI_ARCH:-}"

# buildx (BuildKit) improves the speed and UI of builds over the legacy builder and
# simplifies creation of multi-arch images.
#
# Reference: https://docs.docker.com/build/buildkit/
DOCKER_CMD="docker buildx build ${*}" # Provide all arguments to the script to the build command

if [[ "${DOCKER_IMAGE}" == *"/"* ]]; then
  # Default to pushing when the image name includes a slash which indicates the
  # use of a registry e.g.
  #
  #  - dockerhub: [namespace]/[repo name]:[tag]
  #  - private registry: [private registry hostname]/[repo name]:[tag]
  DOCKER_CMD="${DOCKER_CMD} --push"

  # Build a multi-arch image if requested
  if [[ -n "${BUILD_MULTI_ARCH}" ]]; then
    DOCKER_CMD="${DOCKER_CMD} --platform=${PLATFORMS:-linux/amd64,linux/arm64}"
  fi

  # A populated DOCKER_USERNAME env var triggers login
  if [[ -n "${DOCKER_USERNAME:-}" ]]; then
    echo "${DOCKER_PASS}" | docker login --username "${DOCKER_USERNAME}" --password-stdin
  fi
else
  # Build a single-arch image since the image name does not include a slash which
  # indicates that a registry is not available.
  #
  # Building a single-arch image with buildx and having the resulting image show up
  # in the local store of docker images (ala 'docker build') requires explicitly
  # loading it from the buildx store with '--load'.
  DOCKER_CMD="${DOCKER_CMD} --load"
fi

# Default to the release image. Will need to be overridden when testing against unreleased versions.
AVALANCHEGO_NODE_IMAGE="${AVALANCHEGO_NODE_IMAGE:-avaplatform/avalanchego:${AVALANCHE_DOCKER_VERSION}}"

echo "Building Docker Image: ${DOCKER_IMAGE}:${IMAGE_TAG}, ${DOCKER_IMAGE}:${COMMIT_HASH} based off AvalancheGo@${AVALANCHE_DOCKER_VERSION}"
${DOCKER_CMD} -t "${DOCKER_IMAGE}:${IMAGE_TAG}" -t "${DOCKER_IMAGE}:${COMMIT_HASH}" \
  "${HYPERSDK_PATH}" -f "${HYPERSDK_PATH}/Dockerfile" \
  --build-arg AVALANCHEGO_NODE_IMAGE="${AVALANCHEGO_NODE_IMAGE}" \
  --build-arg VM_COMMIT="${GIT_COMMIT}" \
  --build-arg VM_ID="${VM_ID}" \
  --build-arg VM_NAME="${VM_NAME}"

# Only tag the latest image for the main branch when images are pushed to a registry
if [[ "${DOCKER_IMAGE}" == *"/"* && ("${IMAGE_TAG}" == "main" || -n "${FORCE_TAG_LATEST}") ]]; then
  echo "Tagging current image as ${DOCKER_IMAGE}:latest"
  docker buildx imagetools create -t "${DOCKER_IMAGE}:latest" "${DOCKER_IMAGE}:${COMMIT_HASH}"
fi
