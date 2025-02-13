#!/usr/bin/env bash

set -euo pipefail

# This test script is intended to execute successfully on a ubuntu host with either the amd64
# or arm64 arches. Recent docker (with buildx support) and qemu are required.

# TODO(marun) Maybe perform more extensive validation (e.g. e2e testing) against one or more images?

# Directory above this script
HYPERSDK_PATH=$(
  cd "$(dirname "${BASH_SOURCE[0]}")"
  cd .. && pwd
)

source "$HYPERSDK_PATH"/scripts/constants.sh
source "$HYPERSDK_PATH"/scripts/git_commit.sh
source "$HYPERSDK_PATH"/scripts/image_tag.sh

build_and_test() {
  local image_name="${1}"
  local vm_name="${2}"
  local vm_id="${3}"

  if [[ "$image_name" == *"/"* ]]; then
    # Assume a registry image is a multi-arch image
    local arches=("amd64" "arm64")
    BUILD_MULTI_ARCH=1
  else
    # Test only the host platform for non-registry/single arch builds
    local host_arch
    host_arch="$(go env GOARCH)"
    local arches=("$host_arch")
    BUILD_MULTI_ARCH=
  fi

  BUILD_MULTI_ARCH="${BUILD_MULTI_ARCH}"\
    DOCKER_IMAGE="${image_name}"\
    VM_NAME="${vm_name}"\
    VM_ID=$"${vm_id}"\
   ./scripts/build_docker_image.sh

  echo "listing images"
  docker images

  # Check all of the images expected to have been built
  local target_images=(
    "$image_name:$COMMIT_HASH"
    "$image_name:$IMAGE_TAG"
  )
  for arch in "${arches[@]}"; do
    for target_image in "${target_images[@]}"; do
      echo "checking sanity of image $target_image for $arch by running '${VM_ID} version'"
      docker run  -t --rm --platform "linux/$arch" "$target_image" /avalanchego/build/plugins/"${VM_ID}" version
    done
  done
}

# The name of the VM to build. Defaults to build morpheusvm in examples/morpheusvm/
VM_NAME="${VM_NAME:-${DEFAULT_VM_NAME}}"
VM_ID="${VM_ID:-${DEFAULT_VM_ID}}"

echo "checking build of single-arch image"
build_and_test "${VM_NAME}" "${VM_NAME}" "${VM_ID}"

echo "starting local docker registry to allow verification of multi-arch image builds"
REGISTRY_CONTAINER_ID="$(docker run --rm -d -P registry:2)"
REGISTRY_PORT="$(docker port "$REGISTRY_CONTAINER_ID" 5000/tcp | grep -v "::" | awk -F: '{print $NF}')"

echo "starting docker builder that supports multiplatform builds"
# - creating a new builder enables multiplatform builds
# - '--driver-opt network=host' enables the builder to use the local registry
docker buildx create --use --name ci-builder --driver-opt network=host

# Ensure registry and builder cleanup on teardown
function cleanup {
  echo "stopping local docker registry"
  docker stop "${REGISTRY_CONTAINER_ID}"
  echo "removing multiplatform builder"
  docker buildx rm ci-builder
}
trap cleanup EXIT

echo "checking build of multi-arch images"
build_and_test "localhost:${REGISTRY_PORT}/${VM_NAME}" "${VM_NAME}" "${VM_ID}"
