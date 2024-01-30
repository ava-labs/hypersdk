use std::cmp::Ordering;
use wasmlanche_sdk::{params, program::Program, public, state_keys, types::Address};

// Implementation of a CSAMM. This assumes a 1:1 peg between the two assets or one of them is going to be missing from the pool.

/// The program state keys.
#[state_keys]
enum StateKeys {
    // TODO this is also an LP token, could we reuse parts of the "token" program ?
    // TotalSupply,
    // Balance(Address),
    /// The vec of coins. Key prefix 0x0 + addresses
    // TODO Vec doesn't implement Copy, so can we not define dynamic array in the program storage ?
    // Coins([Address; 2]),
    CoinX,
    CoinY,
}

/// Initializes the program address a count of 0.
#[public]
fn constructor(program: Program, coin_x: Program, coin_y: Program) -> bool {
    if program.state().get::<Program, _>(StateKeys::CoinX).is_ok()
        || program.state().get::<Program, _>(StateKeys::CoinY).is_ok()
    {
        panic!("program already initialized")
    }

    program
        .state()
        .store(StateKeys::CoinX, &coin_x)
        .expect("failed to store coin_x");

    program
        .state()
        .store(StateKeys::CoinY, &coin_y)
        .expect("failed to store coin_y");

    true
}

#[public]
fn coins(program: Program, i: u8) -> *const u8 {
    let coin = match i {
        0 => StateKeys::CoinX,
        1 => StateKeys::CoinY,
        _ => panic!("this coin does not exist"),
    };

    program
        .state()
        .get::<Program, _>(coin)
        .unwrap()
        .id()
        .as_ptr()
}

fn token_x(program: Program) -> Program {
    program.state().get::<Program, _>(StateKeys::CoinX).unwrap()
}

fn token_y(program: Program) -> Program {
    program.state().get::<Program, _>(StateKeys::CoinY).unwrap()
}

fn balances(program: Program) -> (i64, i64) {
    let this = Address::new(*program.id());

    (
        program
            .state()
            .get::<Program, _>(StateKeys::CoinX)
            .unwrap()
            .call_function("get_balance", params!(&this), 10000)
            .unwrap(),
        program
            .state()
            .get::<Program, _>(StateKeys::CoinY)
            .unwrap()
            .call_function("get_balance", params!(&this), 10000)
            .unwrap(),
    )
}

/// exchange `amount` of token T for x amount of token T'.
/// amount > 0 sends tokens to the pool while amount < 0 pulls them.
#[public]
fn exchange(program: Program, amount: i64) {
    let sender = Address::new([0; 32]); // TODO how to get the program caller ?
    let this = Address::new(*program.id());

    // x + y = k
    // x + dx + y - dy = k
    // dy = x + dx + y - k
    // dy = x + dx + y - (x + y)
    // dy = dx

    // NOTE the extra granularity is good but can we avoid having to put a fixed max_units everytime ?
    // NOTE how to get a balance that is more than 32 bits over 0 ? For USD variants with 6 decimals, it's alright.
    // NOTE can we really have negative balances ?
    let (x, y) = balances(program);

    let dx = amount;
    let dy = dx;

    if dx > x {
        panic!("not enough x tokens in the pool!");
    }
    if dy > y {
        panic!("not enough y tokens in the pool!");
    }

    // NOTE rust makes it quite annoying to write constants for non-primitive types. It's not looking good for using pow operations
    match amount.cmp(&0) {
        Ordering::Greater => {
            token_x(program)
                .call_function("transfer", params!(&sender, &this, &dx), 10000)
                .unwrap();
            token_y(program)
                .call_function("transfer", params!(&this, &sender, &dy), 10000)
                .unwrap();
        }
        Ordering::Less => {
            token_y(program)
                .call_function("transfer", params!(&sender, &this, &dx), 10000)
                .unwrap();
            token_x(program)
                .call_function("transfer", params!(&this, &sender, &dy), 10000)
                .unwrap();
        }
        _ => panic!("amount == 0"),
    }
}

/// Add or remove liquidity. Both asset amounts should conform with the CSAMM properties.
#[public]
fn manage_liquidity(program: Program, dx: i64, dy: i64) {
    let sender = Address::new([0; 32]);
    let this = Address::new(*program.id());

    let (x, y) = balances(program);
    let k = x + y;

    assert_eq!(dx.signum(), dy.signum(), "either add or remove liquidity");
    match dx.cmp(&0) {
        Ordering::Greater => {
            token_x(program)
                .call_function("transfer", params!(&sender, &this, &dx), 10000)
                .unwrap();
            token_y(program)
                .call_function("transfer", params!(&sender, &this, &dy), 10000)
                .unwrap();
        }
        Ordering::Less => {
            token_x(program)
                .call_function("transfer", params!(&this, &sender, &-dx), 10000)
                .unwrap();
            token_y(program)
                .call_function("transfer", params!(&this, &sender, &-dy), 10000)
                .unwrap();
        }
        Ordering::Equal => panic!("amounts are negative"),
    };

    // post: is the equation still standing ?
    let (x, y) = balances(program);
    if x + y != k {
        panic!("CSAMM x + y = k property violated");
    }
}
