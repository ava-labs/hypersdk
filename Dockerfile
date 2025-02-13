# ============= Setting up base Stage ================
# AVALANCHEGO_NODE_IMAGE needs to identify an existing node image and should include the tag
ARG AVALANCHEGO_NODE_IMAGE="invalid-image" # This value isn't intended to be used but silences a warning

# ============= Compilation Stage ================
FROM --platform=$BUILDPLATFORM golang:1.22.8-bullseye AS builder

WORKDIR /build

ARG VM_NAME

# Download hypersdk deps
COPY go.mod go.sum ./
RUN go mod download

# Copy the code into the container
COPY . .

# Set the working directory to the VM's directory
WORKDIR /build/examples/$VM_NAME/

# Download VM deps
COPY examples/$VM_NAME/go.mod examples/$VM_NAME/go.sum ./
RUN go mod download

# Ensure pre-existing builds are not available for inclusion in the final image
RUN [ -d ./build ] && rm -rf ./build/* || true

ARG TARGETPLATFORM
ARG BUILDPLATFORM

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
ARG VM_COMMIT
RUN . ./build_env.sh && \
    echo "{CC=$CC, TARGETPLATFORM=$TARGETPLATFORM, BUILDPLATFORM=$BUILDPLATFORM}" && \
    export GOARCH=$(echo ${TARGETPLATFORM} | cut -d / -f2) && \
    export VM_COMMIT="${VM_COMMIT}" && \
    ./scripts/build.sh /build/build/vm

# ============= Cleanup Stage ================
FROM $AVALANCHEGO_NODE_IMAGE AS execution

# Copy the VM binary into the correct location in the container
ARG VM_ID
COPY --from=builder /build/build/vm /avalanchego/build/plugins/$VM_ID
