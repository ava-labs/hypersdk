// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Here, we import the `public` attribute-macro, the `state_schema` macro,
// and the `Address` and `Context` types
use wasmlanche_sdk::{public, state_schema, Address, Context};

// Count is a type alias. It makes it easy to change the type of the counter
// without having to update multiple places. For instance, if you wanted to
// be more space-efficient, you could use a u32 instead of a u64.
type Count = u64;

// We use the `state_schema` macro to define the state-keys and value types.
// The macro will create types such as `struct Counter(Address);`. The type
// is what's called a newtype wrapper around an Address. Keys must be defined as
// unit-structs, `Key`, or tuple-structs `Key(u32, u64)`. Tuple-structs can wrap one
// or more values forming something akin to a composite key in a relational database.
// Keys must be a fixed size (stack value). A good rule of thumb is that if they
// implement the `Copy` trait, they are a good candidate for a key. Try using a
// Heap allocated Type like a `String` or a `Vec` and see what the compiler tells you.
state_schema! {
    /// Counter for each address.
    Counter(Address) => Count,
}

// NOTE: use `wasmlanche_sdk::dbg!` for debugging. It's works like the `std::dbg!` macro.

// The `///` syntax is used to document the function. This documentation
// will be visible when the documentation is generated with `cargo doc`
//
// Every function that is callable form the outside world must be marked with the
// `#[public]` attribute and must follow a specific signature. Try writing a function
// marked with the `#[public]` attribute without any arguemnts. See if the compiler can
// guide you to the correct signature.
//
/// Gets the count at the address.
#[public]
pub fn get_value(context: &mut Context, of: Address) -> Count {
    // `Context::get` returns a Result<Option<T>, Error>
    // where T is the value type associated with the key. In the example below
    // T is `Count` which is just an alias for a `u64` (unless you changed it).
    // Note:
    // `expect` will cause the code to panic if there's a deserialization error
    // otherwise it will take a `Result<T, E>` and return the `T`.`
    context
        .get(Counter(of))
        .expect("state corrupt")
        .unwrap_or_default()
}

/// Increments the count at the address by the amount.
#[public]
pub fn inc(context: &mut Context, to: Address, amount: Count) {
    let counter = amount + get_value(context, to);

    context
        .store_by_key(Counter(to), counter)
        .expect("serialization failed");
}

#[public]
pub fn inc_me_by_one(context: &mut Context) {
    let caller = context.actor();
    inc(context, caller, 1);
}

// If you aren't familiar with Rust, we write our unit tests in the same files as the
// code we're testing.
#[cfg(test)]
mod tests {
    // the line below imports everthing from the parent module (code above)
    use super::*;
    use simulator::{SimpleState, Simulator};
    use wasmlanche_sdk::Address;

    // This is a constant that is set by the build script. It's the path to the
    // .wasm file that's output when we compile.
    const PROGRAM_PATH: &str = env!("PROGRAM_PATH");

    // Let's not worry about how much gas things cost for now.
    const LARGE_AMOUNT_OF_GAS: u64 = 100_000_000;

    #[test]
    fn create_program() {
        let mut state = SimpleState::new();
        // The simulator needs mutable access to state.
        let simulator = Simulator::new(&mut state);

        let error = simulator.create_program(PROGRAM_PATH).has_error();
        assert!(!error, "Create program errored")
    }

    #[test]
    fn increment() {
        let bob = Address::new([1; 33]); // 1 repeated 33 times

        let mut state = SimpleState::new();
        let simulator = Simulator::new(&mut state);

        let counter_address = simulator.create_program(PROGRAM_PATH).program().unwrap();

        simulator.call_program(counter_address, "inc", (bob, 10u64), LARGE_AMOUNT_OF_GAS);

        let value: Count = simulator
            .call_program(counter_address, "get_value", bob, LARGE_AMOUNT_OF_GAS)
            .result()
            .unwrap();

        assert_eq!(value, 10);
    }

    #[test]
    fn inc_by_one() {
        let bob = Address::new([1; 33]);

        let mut state = SimpleState::new();
        let mut simulator = Simulator::new(&mut state);

        // the default actor is the 0-address instead of bob
        simulator.set_actor(bob);

        let counter_address = simulator.create_program(PROGRAM_PATH).program().unwrap();

        simulator.call_program(counter_address, "inc_me_by_one", (), LARGE_AMOUNT_OF_GAS);

        let value: Count = simulator
            .call_program(counter_address, "get_value", bob, LARGE_AMOUNT_OF_GAS)
            .result()
            .unwrap();

        assert_eq!(value, 1);
    }
}
