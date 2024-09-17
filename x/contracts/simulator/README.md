# Rust Smart Contract Simulator

## Overview

The Rust Smart Contract Simulator emulates a Virtual Machine (VM) environment for testing and debugging WASM smart-contracts. It provides a lightweight implementation that simulates smart-contract execution throughout its lifecycle.

The simulator consists of two main components:

1. `State`: Persists smart-contract state across multiple calls
2. `Simulator`: Manages smart-contract creation, execution, and VM control

## Key Components

### State

The `State` struct encapsulates a simple key-value store, representing the VM's state:

```rust
let state = SimpleState::new();
```

### Simulator

The `Simulator` serves as the primary interface for interacting with the simulated VM environment. It offers the following core functionalities:

#### 1. Contract Creation

Create a new smart-contract from a WASM binary:

```rust
pub fn create_contract(&self, smart-contract_path: &str) -> CreateContractResponse
```

The `CreateContractResponse` provides:

- `contract()`: Returns the smart-contract's address
- `contract_id()`: Returns a unique identifier for the smart-contract's bytecode storage
- `error()` or `has_error()`: Indicates potential errors during creation

#### 2. Contract Execution

Call a smart-contract with specified method, parameters, and gas limit:

```rust
pub fn call_contract<T: wasmlanche::borsh::BorshSerialize>(
    &self,
    smart-contract: Address,
    method: &str,
    params: T,
    gas: u64,
) -> CallContractResponse
```

The `CallContractResponse` offers:

- `result<R>()`: Returns the call result (specify expected return type `R`)
- `error()` or `has_error()`: Provides error information if applicable

#### 3. Balances & Actor Control

Interact and set native account balances via:

```rust
pub fn get_balance(&self, address: Address) -> u64
pub fn set_balance(&mut self, address: Address, balance: u64)
```

Set and get the current actor's address:

```rust
pub fn set_actor(&mut self, actor: Address)
pub fn get_actor(&self) -> Address
```

The current actor represents the account address making the call.

> **Note**: In the future, the simulator will also have the ability to toggle the block height and timestamp as well as other VM related functions needed for smart-contract execution.

## Usage

See the `rust/examples` directory.
