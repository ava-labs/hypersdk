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

## Storage

A Program has 2 storage types.

- `Memory` in WebAssembly is a linear buffer of unsigned bytes that can read and
written to by both the guest or host. Because of the very minimal types
supported by WebAssembly Memory is critical for communication between `Program`
and host.

- `State` is a host module which is imported into the `Program` instance, which
exposes access to a database store. Unlike `Memory` state can be durable.

### Memory

In this example we share a byte slice between our host and a `Program`. To
achieve this we can write our byte slice to the shared memory from he host using
an exported functions called `alloc` and `memory` To do this we would first
allocate  a chunk of continuous memory of the length desired.  `Alloc` will
return the `offset` of that slice.


```go
runtime := // create new runtime and initialize
memory := runtime.Memory()

bytes := []byte("hello world")
bytesLen := uint64(len(bytes))

// create a memory allocation 
offset, _ := memory.Alloc(bytesLen)

// copy bytes into memory allocation
offset, _ = memory.Write(offset, bytes)

// pass the offset and lenth as params to a function
// exported by the Program
resp, _ := runtime.Call("some_function", offset, bytesLen )
```

`hello world` converted to a byte slice will represent our Program's memory for
the example
```sh
[offset=6              ][length=3   ]
[104 101 108 108 111 32 119 111 114 108 100]
[119 111 114] // wor
```
The below shows an exported `Program` function `some_function` which accepts a
message of arbitrary length. Unlike fixed size length objects such as `Address`
the length can be variable. So the `length` of the memory along with the
`offset` are both required.

```rust
#[public]
pub fn some_function(program: Program, message: i64, message_length: i64) -> i64 {
  // read message bytes from memory
  let msg := program.memory().read(message, message_length)?;
  // msg = [119 111 114]
}
```
          
Each  Memory is tightly bound to the lifecycle of the module. For this reason
`Memory` is non durable and can not be relied upon. 

### State

- `Storage` is the definition of the schema used to persist Program data.

```rust
#[reps(u8)]
#[storage]
enum Storage {
  /// [program_prefix_byte][program_id_bytes][0x0][address_bytes] = amount
  Balance(Address) // 0x0
}

#[public]
pub fn transfer(program: Program, from: Address, to: Address, amount: i64) -> i64 {
  // check balances from state
  let to_balance := program.state().get(Storage::Balance(to).into().as_ref())?;
  let from_balance :=  program.state().get(Storage::Balance(from).into().as_ref())?;

  if from_balance.lt(amount) {
    return -1
  }

  // update balances to state
  program.state().
    put(Storage::Balance(to).into().as_ref(), &(to_balance + amount )).
    put(Storage::Balance(from).into().as_ref(), &(from_balance - amount ))?
  [...]
}

```

### Rust Program SDK

This folder provides the necessary tools to build WASM programs using rust.

- `/store` : Exposes methods with interacting with the host environment
- `/types` : Defines types(currently just `Address`)
- `/host` : Imports necessary functions from the host.
- `/Program`: Defines the `ProgramValue` and `Progam` types.

### Exporting Program Functions

A program exposes functions that can be called by the host called exports. To
export a function using the Rust SDK we use the `#[public]` attribute above the signature
of the function we want to export.

The example below shows a basic function `add` which is accessible by the host
using the `Call` method of the `Runtime` interface.

```rust
#[public]
pub fn add(_: State, a: i64, b: i64) -> i64 {
  a + b
}
```
Example of golang host calling a Program's `add` funciton.

```go
a := uint64(1)
b := uint64(2)
resp, _ := runtime.Call("add", a, b )
fmt.Printf("1 + 2 = %d", resp[0])
// 1 + 2 = 3
```

# Examples

Compile the examples using
`cargo build --target wasm32-wasi --target-dir $<your_target_directory> --release`

### Token Contract

A simple implementation of a token contract that allows functionality for `transferring` & `minting`. Can also query to retrieve `total_supply` and user `balances`.

### Lottery

An example of invoking an external program(`token_contract`). The `lottery` contract randomly computes a number from 1-100 and calls the `token_contract` to perform the token transfer.

### Counter + Even

A more rudimentary example of invoking an external program from another program. Counter is a program that associates counts with addresses. Counter has an `inc` method that increments a given address's counts. `Even` calls `Counter`'s `inc` method with double the amount specified.
