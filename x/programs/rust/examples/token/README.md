# Token HyperSDK Program Example
This repository contains a simple example of a HyperSDK token program written in Rust.

## Overview
The token program provides the following features:

- Initialization of a token with a fixed supply
- Ability to transfer tokens between accounts
- Query balance of an account
- Mint coins to an address

## Testing
HyperSDK programs can be tested with the VM simulator. To run our simulation
tests, first compile the simulator binary which requires the latest version of
golang. Review the source code of the `test_token_plan` located in `lib.rs` and
use it as a template to build your own tests.

```sh
./scripts/tests.simulator.sh
Building Simulator...
Running Simulator Tests...
    Finished test [unoptimized + debuginfo] target(s) in 0.01s
     Running unittests src/lib.rs ()

running 1 test
test tests::test_token_plan ... ok

test result: ok. 1 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 1.04s
```
