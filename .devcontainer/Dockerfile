FROM public.ecr.aws/docker/library/rust:1.76.0-alpine3.19 AS hawkeye-build
ENV RUSTFLAGS="-C target-feature=-crt-static"
WORKDIR /build

RUN apk fix && \
    apk --no-cache --update add git musl-dev

WORKDIR /build

RUN wget https://github.com/korandoru/hawkeye/archive/refs/tags/v5.7.0.zip && \
    unzip v5.7.0.zip -d /build && \
    mv /build/hawkeye-5.7.0/* /build/ && \
    rm -rf /build/hawkeye-5.7.0 && \
    rm v5.7.0.zip

RUN ls -ltha && cargo build --release --bin hawkeye

# # # # # # # # # # # # # # # # 
# Final image
# # # # # # # # # # # # # # # # 

FROM mcr.microsoft.com/devcontainers/go:1-1.22-bookworm

COPY --from=hawkeye-build /build/target/debug/hawkeye /usr/local/bin/
