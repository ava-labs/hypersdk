use wasmlanche_sdk::{params, public, state_keys, types::Address, Context, Program};

#[state_keys]
enum StateKeys {
    /// The count of this program. Key prefix 0x0 + address
    Counter(Address),
}

/// Initializes the program address a count of 0.
#[public]
pub fn initialize_address(context: Context, address: Address) -> bool {
    let Context { program } = context;

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
pub fn inc(context: Context, to: Address, amount: i64) -> bool {
    let counter = amount + get_value(context, to);
    let Context { program } = context;

    program
        .state()
        .store(StateKeys::Counter(to), &counter)
        .expect("failed to store counter");

    true
}

/// Increments the count at the address by the amount for an external program.
#[public]
pub fn inc_external(_: Context, target: Program, max_units: i64, of: Address, amount: i64) -> i64 {
    let params = params!(&of, &amount).unwrap();
    target.call_function("inc", params, max_units).unwrap()
}

/// Gets the count at the address.
#[public]
pub fn get_value(context: Context, of: Address) -> i64 {
    let Context { program } = context;
    program
        .state()
        .get(StateKeys::Counter(of))
        .expect("failed to get counter")
}

/// Gets the count at the address for an external program.
#[public]
pub fn get_value_external(_: Context, target: Program, max_units: i64, of: Address) -> i64 {
    let params = params!(&of).unwrap();
    target
        .call_function("get_value", params, max_units)
        .unwrap()
}
