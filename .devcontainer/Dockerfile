ARG INSTALL_AVALANCHEGO_VERSION_SHORT
FROM avaplatform/avalanchego:${INSTALL_AVALANCHEGO_VERSION_SHORT} AS avalanchego

FROM mcr.microsoft.com/devcontainers/go:1

ARG INSTALL_AVALANCHEGO_VERSION
COPY --from=avalanchego /avalanchego/build/avalanchego /tmp/hypersdk/avalanchego-${INSTALL_AVALANCHEGO_VERSION}/avalanchego
