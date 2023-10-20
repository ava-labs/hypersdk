# Rust Program SDK

## Build Notes

- Install the wasm32-unknown-unknown target.
```sh
rustup target add wasm32-unknown-unknown
```

- Compile `Program`` to WebAssembly.
```sh
cargo build --target wasm32-unknown-unknown --target-dir $CARGO_TARGET_DIR --release
```

- Optionally use our build script.

```sh
./scripts/build.sh
```

## Debugging

While developing programs you can optionally compile the program with `Wasi`
support. This allows, for example, logging from your program. The compiled
`Wasm` will be automatically supported by the VM simulator, or if you explicitly
set `WithEnableTestingOnlyMode` for the runtime `Config`. 

**NOTE**: Once testing is complete, don't forget to remove all print statements
from your program and recompile without `DEBUG`.

```sh
DEBUG=1 ./scripts/build.sh
```

## Storage

Memory in WebAssembly is a linear buffer of unsigned bytes that can read and written to by the guest or host. 

## Serialization Between Host(Go) & Guest(Rust)

Just a couple things to note. Serialization is minimal, yet there are certain
aspects in the code that need to follow a specific format. Specifically, there
are currently only two places we modify the bytes coming in/out of rust.

1. The first is quite minimal. When storing a value in the host, the
SDK prepends a single byte representing the type of `ProgramValue`
being stored. This single byte is necessary to inform Rust about the variable
type when retrieving from the `host`.
2. The second area is a bit more complex and happens during a call to invoke
another program. In this case, we pass a byte array which contains the parameters
for the external function call. To construct this we marshal all the params with
their metadata into one final byte array. Each parameter is added in this order
  - length of the parameter in bytes(stored as a i64)
  - boolean, [1] if the parameter is an Int, [0] otherwise
  - the actual bytes of the parameter.

On the Go side, we unmarshal in the same order.

## Storage

A Program has 2 storage types.

- `Memory` in WebAssembly is a linear buffer of unsigned bytes that can be read and
written to by both the `Program` or host.

- `State` is a host module which is imported into the `Program` instance. It
exposes access to a database store. Unlike `Memory` state can be durable.

### Memory

In this example we share a byte slice between our host and a `Program`. To
achieve this, we can write our byte slice to the shared memory from the host using
an exported functions called `alloc` and `memory`. To do this, we would first
allocate a chunk of continuous memory of the length desired.  `Alloc` will
return the `offset` of that slice.

```go
runtime := // create new runtime and initialize
memory := runtime.Memory()

bytes := []byte("hello world")
bytesLen := uint64(len(bytes))

// create a memory allocation 
offset, _ := memory.Alloc(bytesLen)

// copy bytes into memory allocation
_ = memory.Write(offset, bytes)

// pass the offset and length as params to a function
// exported by the Program
resp, _ := runtime.Call("some_function", offset, bytesLen )
```

`hello world` converted to a byte slice will represent our Program's memory for
the example.
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
    let mem = Memory::new(Pointer::new(message));
    let msg = unsafe { mem.range(12) };
    // msg = [119 111 114]
}
```
          
Each `Memory` is tightly bound to the lifecycle of the module. For this reason,
`Memory` is non durable and can not be relied upon outside the scope of a `#[public]` function.

### State

- `state_keys` is the definition of the schema used to persist `Program` data.

```rust
type ProposalId = [u8; 8];

#[state_keys]
enum StateKeys {
    Proposal(ProposalId) // 0x0 + [u8; 8]
}

#[public]
pub fn vote(program: Program, proposal: i64, weight: i64) -> i64 {
  // check current votes
  let votes: i64 = program
        .state()
        .get(StateKey::Proposal(proposal.to_be_bytes()).to_vec())
        .expect("failed to get proposal")


  // update vote
  program
        .state()
        .store(StateKey::Proposal(proposal.to_be_bytes()).to_vec(), &(votes+weight))
        .expect("failed to store total supply");
   ...
}

```

### Rust Program SDK

These modules provides the necessary tools to build WASM programs using rust.

- `state` : Exposes methods with interacting with the host state.
- `types` : Defines types(currently just `Address`).
- `host` : Imports necessary functions from the host.

### Exporting Program Functions

A program exposes functions that can be called by the host named exports. To
export a function using the Rust SDK, we use the `#[public]` attribute above the signature
of the function we want to export.

The example below shows a basic function `add`, which is accessible by the host
using the `Call` method of the `Runtime` interface.

```rust
#[public]
pub fn add(_: Program, a: i64, b: i64) -> i64 {
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

To compile the examples see the Build Notes above.

### Token Contract

A simple implementation of a token contract that allows functionality for
`transferring` & `minting`. Can also query to retrieve `total_supply` and user
`balances`.

### Counter

A more rudimentary example of invoking an external program from another program.
Counter is a program that associates counts with addresses.
