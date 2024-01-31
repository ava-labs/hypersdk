use wasmlanche_sdk::{params, program::Program, public, state_keys, types::Address};

// Implementation of a CSAMM. This assumes a 1:1 peg between the two assets or one of them is going to be missing from the pool.

/// The program state keys.
#[state_keys]
enum StateKeys {
    // TODO this is also an LP token, could we reuse parts of the "token" program ?
    // TotalSupply,
    // Balance(Address),
    // Coins(Program),
    CoinX,
    CoinY,
}

/// Initializes the program address a count of 0.
#[public]
fn constructor(program: Program, coin_x: Program, coin_y: Program) -> bool {
    program
        .state()
        .store(StateKeys::CoinX, &coin_x)
        .expect("failed to store coin_x");

    program
        .state()
        .store(StateKeys::CoinY, &coin_y)
        .expect("failed to store coin_x");

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

    if amount == 0 {
        panic!("amount == 0");
    }

    // x + y = k
    // x + dx + y - dy = k
    // dy = x + dx + y - k
    // dy = x + dx + y - (x + y)
    // dy = dx
    let (x, y) = balances(program);

    let dx = amount;
    let dy = dx;

    if dx > x {
        panic!("not enough x tokens in the pool!");
    }
    if dy > y {
        panic!("not enough y tokens in the pool!");
    }

    // NOTE the extra granularity is good but can we avoid having to put a fixed max_units everytime ?
    // NOTE how to get a balance that is more than 32 bits over 0 ? For USD variants with 6 decimals, it's alright.
    // NOTE can we really have negative balances ?
    // NOTE rust makes it quite annoying to write constants for non-primitive types. It's not looking good for using pow operations

    let (token_x_recipient, token_y_recipient) = if amount > 0 {
        (this, sender)
    } else {
        (sender, this)
    };

    token_x(program)
        .call_function(
            "transfer",
            params!(&token_y_recipient, &token_x_recipient, &dx),
            10000,
        )
        .unwrap();

    token_y(program)
        .call_function(
            "transfer",
            params!(&token_x_recipient, &token_y_recipient, &dy),
            10000,
        )
        .unwrap();
}

/// Add or remove liquidity. Both asset amounts should conform with the CSAMM properties.
#[public]
fn manage_liquidity(program: Program, dx: i64, dy: i64) {
    let sender = Address::new([0; 32]);
    let this = Address::new(*program.id());

    assert_eq!(dx, dy, "inconsistent liquidity token amounts");
    assert_ne!(dx, 0, "cannot add 0 liquidity");

    let (token_sender, token_recipient) = if dx > 0 {
        (sender, this)
    } else {
        (this, sender)
    };

    token_x(program)
        .call_function(
            "transfer",
            params!(&token_sender, &token_recipient, &dx),
            10000,
        )
        .unwrap();

    token_y(program)
        .call_function(
            "transfer",
            params!(&token_sender, &token_recipient, &dy),
            10000,
        )
        .unwrap();

    // post: is the equation still standing ?
    let (x, y) = balances(program);
    assert_eq!(x, y, "CSAMM x + y = k property violated");
}

#[cfg(test)]
mod tests {
    use serial_test::serial;
    use std::env;
    use wasmlanche_sdk::simulator::{
        self, id_from_step, Key, Operator, PlanResponse, Require, ResultAssertion,
    };

    #[test]
    #[serial]
    #[ignore = "requires SIMULATOR_PATH and PROGRAM_PATH to be set"]
    fn test_swap() {
        use wasmlanche_sdk::simulator::{self, Endpoint, Key, Param, ParamType, Plan, Step};

        let s_path = env::var(simulator::PATH_KEY).expect("SIMULATOR_PATH not set");
        let simulator = simulator::Client::new(s_path);

        let owner_key = "owner";
        // create owner key in single step
        let resp = simulator
            .key_create::<PlanResponse>(owner_key, Key::Ed25519)
            .unwrap();
        assert_eq!(resp.error, None);

        // create multiple step test plan
        let mut plan = Plan::new(owner_key);

        // step 0: create program
        let p_path = env::var("PROGRAM_PATH").expect("PROGRAM_PATH not set");
        plan.add_step(Step {
            endpoint: Endpoint::Execute,
            method: "program_create".into(),
            max_units: 0,
            params: vec![Param::new(ParamType::String, p_path.as_ref())],
            require: None,
        });
    }
}
