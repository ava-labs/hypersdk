use wasmlanche_sdk::{program::Program, public, state, types::Address};

#[state]
enum StateKeys {
    /// The count of this program. Key prefix 0x0 + address
    Counter(Address),
}

/// Initializes the program address a count of 0.
#[public]
fn initialize_address(program: Program, address: Address) -> bool {
    if program
        .state()
        .get_value::<i64, _>(StateKeys::Counter(address).to_vec())
        .is_ok()
    {
        panic!("counter already initialized for address")
    }

    program
        .state()
        .store(StateKeys::Counter(address).to_vec(), &0_i64)
        .expect("failed to store counter");

    true
}

/// Increments the count at the address by the amount.
#[public]
fn inc(program: Program, to: Address, amount: i64) -> bool {
    let counter = amount + value(program, to);

    program
        .state()
        .store(StateKeys::Counter(to).to_vec(), &counter)
        .expect("failed to store counter");

    true
}

/// Gets the count at the address.
#[public]
fn value(program: Program, of: Address) -> i64 {
    program
        .state()
        .get_value(StateKeys::Counter(of).to_vec())
        .expect("failed to get counter")
}
