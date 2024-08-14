# Rust Documentation
## StateKeys
## External Calls
When a contract wants to call a function on another contract, it can be done two different ways.
### The Type Safe Way
If you have access to the code of the other contract, you can reference it directly in your code by including it in your Cargo.toml file.
```toml
otherContract = { path = "../someOtherContract", features = ["bindings"] }
```
Adding the reference will provide you with a crate that contains all of the public functions exposed by that contract.  See the example
```rust
// create a call context that can be used to call the program at programAddress
let ctx = ExternalCallContext::new(programAddress, fuelUsedInCall, amountOfNativeToSend)

// call the function foobar defined in the otherContract crate with param someValue
let result = otherContract::foobar(&ctx, someValue);
```

Calling it this way ensures the types of your parameters and the return are correct.
        
### The Type Unsafe Way
If you don't have access to the other contract's rust code, you can also make calls using the unsafe way.  If you have a variable of type Program you can call functions on it: 
```rust
    // call the balance function of the target Program with no parameters, a specific amount of fuel allowed and some amount of native currency
    let result = target.call_function("balance", &[], fuelUsedInCall, amountOfNativeToSend)
```

## Host Functions

## Testing