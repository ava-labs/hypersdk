# Wasmlanche_SDK
The `wasm_guest` folder contains  the `wasmlanche_sdk`, macros and example programs

### Wasmlanche_SDK
This folder provides the necessary tools to build WASM programs using rust. 
- `/store` : Exposes methods with interacting with the host environment
- `/types` : Defines types(currently just `Address`)
- `/host` :  Imports necessary functions from the host. 
- `/Program`: Defines the `ProgramValue` and `Progam` types.

### Expose Macro
A rust crate that contains an attribute procedural macro `expose` allowing program functions to be exposed to the host.

# Examples

### Token Contract
A simple implementation of a token contract that allows functionality for `transferring` & `minting`. Can also query to retrieve `total_supply` and user `balances`.

### Lottery
An example of invoking an external program(`token_contract`). The `lottery` contract randomly computes a number from 1-100 and calls the `token_contract` to perform the token transfer. 


### Counter + Even 
A more rudimentary example of invoking an external program from another program. Counter is a program that associates counts with addresses. Counter has an `inc` method that increments a given address's counts. `Even` calls `Counter`'s `inc` method with double the amount specified.
