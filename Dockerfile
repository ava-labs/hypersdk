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

# Copy avalanche dependencies first (intermediate docker image caching)
# Copy avalanchego directory if present (for manual CI case, which uses local dependency)
COPY go.mod go.sum avalanchego* ./

# Download avalanche dependencies using go mod
RUN go mod download && go mod tidy

# Copy the code into the container
COPY . .


# Ensure pre-existing builds are not available for inclusion in the final image
RUN [ -d ./build ] && rm -rf ./build/* || true

ARG VM_NAME

WORKDIR /build/examples/$VM_NAME/

RUN export VM_COMMIT=$VM_COMMIT && export CURRENT_BRANCH=$CURRENT_BRANCH && ./scripts/build.sh /build/build/vm

# ============= Cleanup Stage ================
FROM $AVALANCHEGO_NODE_IMAGE AS builtImage

# Copy the evm binary into the correct location in the container
ARG VM_ID
COPY --from=builder /build/build/vm /avalanchego/build/plugins/$VM_ID
