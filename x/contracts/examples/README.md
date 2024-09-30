# Examples

### amm

- TODO

### counter

- smart-contract that stores an Address -> Count mapping. Contains two functions `inc` which increments a value by a count, and `get_value`a value at an Address.

### counter-external

- example smart-contract showing external smart-contract invocation. Calls the counter smart-contracts `inc` and `get_value` functions.

### token

- A simple ERC-20 replica

## Installation

To run examples locally, you will need to install the following dependencies:

- Rus. You can do this by following these instructions:

  - https://www.rust-lang.org/tools/install

- Wasm Target. You can do this by running the following command:
  - `rustup target add wasm32-unknown-unknown`
