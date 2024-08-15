ARG INSTALL_AVALANCHEGO_VERSION_SHORT
FROM avaplatform/avalanchego:${INSTALL_AVALANCHEGO_VERSION_SHORT} AS avalanchego

ARG INSTALL_GO_VERSION=1.22
FROM mcr.microsoft.com/devcontainers/go:1-bookworm

ARG INSTALL_AVALANCHEGO_VERSION
COPY --from=avalanchego /avalanchego/build/avalanchego /tmp/hypersdk/avalanchego-${INSTALL_AVALANCHEGO_VERSION}/avalanchego