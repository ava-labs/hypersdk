# WASM Based Programs

## Status
`Alpha` not ready for production use.

## TODOs

- [ ] Harden metering and add prefetch support.
- [ ] Analyze and improve performance.
- [ ] Implement a fully functional example VM.

## Introduction

`Programs` are metered on-chain compiled `WebAssembly` that can be executed
very similarly to a smart contract. This allows for developers to extend the
functionality of a HyperSDK VM without the need to change its code.

#### Examples

- Execution - See the test examples defined in the `examples/` directory.

- Building - Checkout the `rust/` directory examples of programs written in Rust. 

## Design choices

### wazero WASM runtime engine

wazero is a WebAssembly runtime written in Go which has zero dependencies which
does not rely on CGO.

### Metering

Program metering support currently utilizes the `Interpreter` based
implementation of the WASM virtual machine. By using the interpreter the meter
can perform cost lookups of WebAssembly opcodes in realtime and in the future
AOT.

### Rust Program SDK

Although many languages provide WebAssembly support, Rust was chosen because of
its strong native WASM support and fantastic community.
