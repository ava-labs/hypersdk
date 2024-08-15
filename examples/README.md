# Rust Documentation

## StateKeys
## External Calls
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

## Testing