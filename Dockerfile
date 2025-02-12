# syntax=docker/dockerfile:experimental

# ============= Setting up base Stage ================
# AVALANCHEGO_NODE_IMAGE needs to identify an existing node image and should include the tag
ARG AVALANCHEGO_NODE_IMAGE
ARG VM_ID
ARG VM_COMMIT
ARG CURRENT_BRANCH

# ============= Compilation Stage ================
FROM --platform=$BUILDPLATFORM golang:1.22.8-bullseye AS builder

WORKDIR /build

# Copy the code into the container
COPY . .

ARG VM_NAME
ARG TARGETPLATFORM
ARG BUILDPLATFORM

WORKDIR /build/examples/$VM_NAME/

# Ensure pre-existing builds are not available for inclusion in the final image
RUN [ -d ./build ] && rm -rf ./build/* || true

# Configure a cross-compiler if the target platform differs from the build platform.
#
# build_env.sh is used to capture the environmental changes required by the build step since RUN
# environment state is not otherwise persistent.
RUN if [ "$TARGETPLATFORM" = "linux/arm64" ] && [ "$BUILDPLATFORM" != "linux/arm64" ]; then \
    apt-get update && apt-get install -y gcc-aarch64-linux-gnu && \
    echo "export CC=aarch64-linux-gnu-gcc" > ./build_env.sh \
    ; elif [ "$TARGETPLATFORM" = "linux/amd64" ] && [ "$BUILDPLATFORM" != "linux/amd64" ]; then \
    apt-get update && apt-get install -y gcc-x86-64-linux-gnu && \
    echo "export CC=x86_64-linux-gnu-gcc" > ./build_env.sh \
    ; else \
    echo "export CC=gcc" > ./build_env.sh \
    ; fi

# Build VM binary. The build environment is configured with build_env.sh from the step
# enabling cross-compilation.
RUN . ./build_env.sh && \
    echo "{CC=$CC, TARGETPLATFORM=$TARGETPLATFORM, BUILDPLATFORM=$BUILDPLATFORM}" && \
    export GOARCH=$(echo ${TARGETPLATFORM} | cut -d / -f2) && \
    export VM_COMMIT=$VM_COMMIT && \
    export CURRENT_BRANCH=$CURRENT_BRANCH && \
    ./scripts/build.sh /build/build/vm

# ============= Cleanup Stage ================
FROM $AVALANCHEGO_NODE_IMAGE AS builtImage

# Copy the VM binary into the correct location in the container
ARG VM_ID
COPY --from=builder /build/build/vm /avalanchego/build/plugins/$VM_ID
