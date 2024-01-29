use wasmlanche_sdk::{params, program::Program, public, state_keys, types::Address};

#[state_keys]
enum StateKeys {
    /// The count of this program. Key prefix 0x0 + address
    Counter(Address),
}

/// Initializes the program address a count of 0.
#[public]
fn initialize_address(program: Program, address: Address) -> bool {
    if program
        .state()
        .get::<i64, _>(StateKeys::Counter(address))
        .is_ok()
    {
        panic!("counter already initialized for address")
    }

    program
        .state()
        .store(StateKeys::Counter(address), &0_i64)
        .expect("failed to store counter");

    true
}

/// Increments the count at the address by the amount.
#[public]
fn inc(program: Program, to: Address, amount: i64) -> bool {
    let counter = amount + get_value(program, to);

    program
        .state()
        .store(StateKeys::Counter(to), &counter)
        .expect("failed to store counter");

    true
}

/// Increments the count at the address by the amount for an external program.
#[public]
fn inc_external(_: Program, target: Program, max_units: i64, of: Address, amount: i64) -> i64 {
    target
        .call_function("inc", params!(&of, &amount), max_units)
        .unwrap()
}

/// multiply is an example of a reentrant function. It will call inc with value `x`, `y` times.
/// This function will fail because it is not marked as reentrant.
#[public]
fn multiply(program: Program, address: Address, x: i64, y: i64, max_units: i64) {
    if y > 0 && inc(program, address, x) {
        program
            .call_function(
                "multiply",
                params!(&address, &x, &(y - 1), &(max_units / 10)),
                max_units,
            )
            .expect("failed to call multiply");
    }
}

/// multiply_reentrant is an example of a reentrant function. It will call inc with value `x`, `y` times.
/// This function will succeed because it is marked as reentrant.
#[public(reentrant)]
fn multiply_reentrant(program: Program, address: Address, x: i64, y: i64, max_units: i64) {
    if y > 0 && inc(program, address, x) {
        program
            .call_function(
                "multiply_reentrant",
                params!(&address, &x, &(y - 1), &(max_units / 10)),
                max_units,
            )
            .expect("failed to call multiply");
    }
}

/// Gets the count at the address.
#[public]
fn get_value(program: Program, of: Address) -> i64 {
    program
        .state()
        .get(StateKeys::Counter(of))
        .expect("failed to get counter")
}

/// Gets the count at the address for an external program.
#[public]
fn get_value_external(_: Program, target: Program, max_units: i64, of: Address) -> i64 {
    target
        .call_function("get_value", params!(&of), max_units)
        .unwrap()
}
