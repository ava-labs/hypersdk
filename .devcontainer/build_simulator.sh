#!/usr/bin/env bash

export GOPATH=/go
export PATH=$PATH:$GOPATH/bin

# Build simulator
cd /go/src/github.com/ava-labs/hypersdk/x/programs/examples/simulator
go build simulator.go

rustup target add wasm32-wasi

# Clone simulator UI
git clone https://github.com/samliok/JackJackIDE.git /simulator-ui
cd /simulator-ui
npm install

cd /go/src/github.com/ava-labs/hypersdk/x/programs/examples/simulator/server
go run server.go
