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

## Memory

Runtime memory is cleared once the program function returns. The host vm can allocate `len` bytes into the WASM runtime memory by calling `pub extern "C" fn alloc(len: usize) -> HostPtr {` defined in `wasmlanche-sdk/memory`. `alloc` will return a ptr that the host can then write at.

## StateKeys

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

## Exporting Program Functions

A program exposes functions that can be called by the host named exports. To
export a function using the Rust SDK, we use the `#[public]` attribute above the signature
of the function we want to export.

## Calling Other Contracts
There are two calling patterns available for when a contract wants to make a call to another contract.

### The Type Safe Way
This is the preferred way to call other contracts because it provides you with the full type signature of the function being called.  To access these signatures, simply reference the contract's crate with the "bindings" feature.  Once that is included, you will have all of the contracts public functions available to call.  If we had a contract that we wanted to call, named someOtherContract, then we would include it in our contract's Cargo.toml file.
```toml
otherContract = { path = "../someOtherContract", features = ["bindings"] }
```

Now that we have the other contract's public functions available, we can call them using the wasmlanche_sdk::ExternalCallContext.  The ExternalCallContext controls the amount of fuel allowed and the amount of native currency that will be sent along with the call.
```rusts
// create a call context that can be used to call the otherContractProgram
let otherContractContext = ExternalCallContext::new(otherContractProgram, maxFuelUsedInCall, amountOfNativeToSend)

// call the function foobar defined in the otherContract crate with param someValue
let result = otherContract::foobar(&otherContractContext, someValue);
```

### The Type Unsafe Way

If you don't have access to the other contract's rust code, then you can also make calls using the unsafe way.  If you have a re:
```rust
    #[public]
    pub fn call_foobar(_: &mut Context,  otherContract: Program, someValue: someType) -> u64 {
        // call the foobar function of the otherContract with someValue as the param.
        otherContract.call_function("foobar", &[someValue], maxFuelUsedInCall, amountOfNativeToSend)
    }
```

# Examples

To compile the examples see the Build Notes above.
