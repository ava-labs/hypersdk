# Rust Program SDK

The `wasm_guest` folder contains the `wasmlanche_sdk`, macros and example programs

## Build Notes

first install the wasm32-wasi target
`rustup target add wasm32-wasi`

build using `cargo build --target wasm32-wasi --target-dir $CARGO_TARGET_DIR --release`

or, build into the testdata folder using the build script

`./scripts/build.sh`

## Serialization Between Host(Go) & Guest(Rust)

Just a couple things to note. Serialization is minimal, yet there are certain aspects in the code that need to follow a specific format. Specifically there are currently only two places we modify the bytes coming in/out of rust.

- The first is quite minimal. When storing a value in the host, the `wasmlanche_sdk` prepends a single byte representing the type of `ProgramValue` being stored. This single byte is necessary to inform Rust about the variable type when retrieving from the `host`.
- The second area is a bit more complex and happens during a call to invoke another program. In this case we pass a byte array which contains the parameters for the external function call. To construct this we marshal all the params with their metadata into one final byte array. Each parameter is added in this order
  - length of the parameter in bytes(stored as a i64)
  - boolean, [1] if the parameter is an Int, [0] otherwise
  - the actual bytes of the parameter.

On the Go side, we unmarshal in the same order.

### Rust Program SDK

This folder provides the necessary tools to build WASM programs using rust.

- `/store` : Exposes methods with interacting with the host environment
- `/types` : Defines types(currently just `Address`)
- `/host` : Imports necessary functions from the host.
- `/Program`: Defines the `ProgramValue` and `Progam` types.

### Expose Macro

A rust crate that contains an attribute procedural macro `expose` allowing program functions to be exposed to the host.

# Examples

Compile the examples using
`cargo build --target wasm32-wasi --target-dir $<your_target_directory> --release`

### Token Contract

A simple implementation of a token contract that allows functionality for `transferring` & `minting`. Can also query to retrieve `total_supply` and user `balances`.

### Lottery

An example of invoking an external program(`token_contract`). The `lottery` contract randomly computes a number from 1-100 and calls the `token_contract` to perform the token transfer.

### Counter + Even

A more rudimentary example of invoking an external program from another program. Counter is a program that associates counts with addresses. Counter has an `inc` method that increments a given address's counts. `Even` calls `Counter`'s `inc` method with double the amount specified.
