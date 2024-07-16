# Rust Program SDK

## Build Notes

- Install the wasm32-unknown-unknown target.

```sh
rustup target add wasm32-unknown-unknown
```

- Compile `Program` to WebAssembly.

```sh
cargo build --target wasm32-unknown-unknown --target-dir $CARGO_TARGET_DIR --release
```

## Debugging

While developing programs you can optionally compile the program with `Wasi`
support. This allows, for example, logging from your program. The compiled
`Wasm` will be automatically supported by the VM simulator, or if you explicitly
set `WithEnableTestingOnlyMode` for the runtime `Config`.

**NOTE**: Once testing is complete, don't forget to remove all print statements
from your program and recompile without `DEBUG`.

```sh
cargo build --target wasm32-wasi
```

## Storage

Memory in WebAssembly is a linear buffer of unsigned bytes that can read and written to by the guest or host.

## Serialization Between Host(Go) & Guest(Rust)

Objects are serialized and passed to/from WASM using [borsh](https://github.com/near/borsh).

### Memory

Runtime memory is cleared once the program function returns. The host vm can allocate `len` bytes into the WASM runtime memory by calling `pub extern "C" fn alloc(len: usize) -> HostPtr {` defined in `wasmlanche-sdk/memory`. `alloc` will return a ptr that the host can then write at.

### StateKeys

`#[state_keys]` macro define global state variables that persist in storage. `#[state_keys]` can define enum values with multiple variants.

See [here](https://github.com/ava-labs/hypersdk/blob/main/x/programs/test/programs/state_access/src/lib.rs) for more info about StateKey interactions.

```rust
#[state_keys]
pub enum StateKeys {
    /// The total supply of the token. Key prefix 0x0.
    TotalSupply,
    /// The symbol of the token. Key prefix 0x1.
    Symbol,
    /// The balance of the token by address. Key prefix 0x2 + address.
    Balance(Address),
}

```

In this StateKeys enum, `TotalSupply` and `Symbol` represent single-value storage slots, while `Balance(Address)` represents a key-value mapping, analogous to a Solidity mapping where Address serves as the key. The associated data in these enum variants is not subject to compile-time type checking. Therefore, it is crucial to maintain consistency in your program when reading from or writing to these keys.

### Exporting Program Functions

A program exposes functions that can be called by the host named exports. To
export a function using the Rust SDK, we use the `#[public]` attribute above the signature
of the function we want to export.

# Examples

To compile the examples see the Build Notes above.
