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
