# # # # # # # # # # # # # # # # 
# Avalanchego builder
# # # # # # # # # # # # # # # # 

ARG INSTALL_GO_VERSION=1.22
FROM mcr.microsoft.com/devcontainers/go:1-${INSTALL_GO_VERSION}-bookworm AS avalanchego-builder

WORKDIR /build
RUN git clone https://github.com/ava-labs/avalanchego.git

ARG INSTALL_AVALANCHEGO_VERSION=d729e5c7ef9f008c3e89cd7131148ad3acda2e34

WORKDIR /build/avalanchego
RUN git checkout ${INSTALL_AVALANCHEGO_VERSION}
RUN ./scripts/build.sh

# # # # # # # # # # # # # # # # 
# Hawkeye builder
# # # # # # # # # # # # # # # # 

FROM rust:1.78.0 AS hawkeye-builder
ARG HAWKEYE_VERSION=5.7.0
WORKDIR /build
RUN curl -L https://github.com/korandoru/hawkeye/archive/refs/tags/v${HAWKEYE_VERSION}.zip -o file.zip
RUN unzip file.zip
WORKDIR /build/hawkeye-${HAWKEYE_VERSION}
RUN echo  "[toolchain]newLinechannel = \"1.78\"newLinecomponents = [\"cargo\", \"rustfmt\", \"clippy\", \"rust-analyzer\"]newLine" | sed "s|newLine|\n|g" > rust-toolchain.toml
RUN cargo build --release --bin hawkeye

RUN mv /build/hawkeye-${HAWKEYE_VERSION}/target/release/hawkeye /usr/local/bin/


# # # # # # # # # # # # # # # # 
# Final image
# # # # # # # # # # # # # # # # 

FROM mcr.microsoft.com/devcontainers/go:1-${INSTALL_GO_VERSION}-bookworm

ARG INSTALL_AVALANCHEGO_VERSION=d729e5c7ef9f008c3e89cd7131148ad3acda2e34

COPY --from=hawkeye-builder /usr/local/bin/hawkeye /usr/local/bin/hawkeye
COPY --from=avalanchego-builder /build/avalanchego/build/avalanchego /tmp/hypersdk/avalanchego-${INSTALL_AVALANCHEGO_VERSION}/avalanchego

