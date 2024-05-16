use borsh::{BorshDeserialize, BorshSerialize};
use wasmlanche_sdk::Context;
use wasmlanche_sdk::{public, state_keys, types::Address};

const INITIAL_SUPPLY: i64 = 123456789;

/// The program state keys.
#[state_keys]
enum StateKey {
    /// The total supply of the token. Key prefix 0x0.
    TotalSupply,
    /// The name of the token. Key prefix 0x1.
    Name,
    /// The symbol of the token. Key prefix 0x2.
    Symbol,
    /// The balance of the token by address. Key prefix 0x3 + address.
    Balance(Address),
}

/// Initializes the program with a name, symbol, and total supply.
#[public]
pub fn init(context: Context) {
    let Context { program, .. } = context;

    // set total supply
    program
        .state()
        .store(StateKey::TotalSupply, &INITIAL_SUPPLY)
        .expect("failed to store total supply");

    // set token name
    program
        .state()
        .store(StateKey::Name, b"WasmCoin")
        .expect("failed to store coin name");

    // set token symbol
    program
        .state()
        .store(StateKey::Symbol, b"WACK")
        .expect("failed to store symbol");
}

/// Returns the total supply of the token.
#[public]
pub fn get_total_supply(context: Context) -> i64 {
    let Context { program, .. } = context;
    program
        .state()
        .get(StateKey::TotalSupply)
        .expect("failed to get total supply")
}

/// Transfers balance from the token owner to the recipient.
#[public]
pub fn mint_to(context: Context, recipient: Address, amount: i64) -> bool {
    let Context { program, .. } = context;
    let balance = program
        .state()
        .get::<i64>(StateKey::Balance(recipient))
        .unwrap_or_default();

    program
        .state()
        .store(StateKey::Balance(recipient), &(balance + amount))
        .expect("failed to store balance");

    true
}

/// Burn the token from the recipient.
#[public]
pub fn burn_from(context: Context, recipient: Address) -> i64 {
    let Context { program, .. } = context;
    program
        .state()
        .delete::<i64>(StateKey::Balance(recipient))
        .expect("failed to burn recipient tokens")
        .expect("recipient balance not found")
}

/// Transfers balance from the sender to the the recipient.
#[public]
pub fn transfer(context: Context, sender: Address, recipient: Address, amount: i64) -> bool {
    let Context { program, .. } = context;
    assert_ne!(sender, recipient, "sender and recipient must be different");

    // ensure the sender has adequate balance
    let sender_balance = program
        .state()
        .get::<i64>(StateKey::Balance(sender))
        .expect("failed to update balance");

    assert!(amount >= 0 && sender_balance >= amount, "invalid input");

    let recipient_balance = program
        .state()
        .get::<i64>(StateKey::Balance(recipient))
        .unwrap_or_default();

    // update balances
    program
        .state()
        .store(StateKey::Balance(sender), &(sender_balance - amount))
        .expect("failed to store balance");

    program
        .state()
        .store(StateKey::Balance(recipient), &(recipient_balance + amount))
        .expect("failed to store balance");

    true
}

#[derive(BorshDeserialize, BorshSerialize)]
pub struct Minter {
    to: Address,
    amount: i32,
}

/// Mints tokens to multiple recipients.
#[public]
pub fn mint_to_many(context: Context, minters: Vec<Minter>) -> bool {
    for minter in minters.iter() {
        mint_to(context, minter.to, minter.amount as i64);
    }

    true
}

/// Gets the balance of the recipient.
#[public]
pub fn get_balance(context: Context, recipient: Address) -> i64 {
    let Context { program, .. } = context;
    program
        .state()
        .get(StateKey::Balance(recipient))
        .unwrap_or_default()
}

#[cfg(test)]
mod tests {
    use simulator::{Endpoint, Key, Param, Plan, Require, ResultAssertion, Step};

    use crate::INITIAL_SUPPLY;

    const PROGRAM_PATH: &str = env!("PROGRAM_PATH");

    #[test]
    fn create_program() {
        let simulator = simulator::Client::new();

        let owner_key = String::from("owner");

        let mut plan = Plan::new(owner_key.clone());

        plan.add_step(Step::create_key(Key::Ed25519(owner_key)));
        plan.add_step(Step::create_program(PROGRAM_PATH));

        // run plan
        let plan_responses = simulator.run_plan(plan).unwrap();

        // ensure no errors
        assert!(
            plan_responses.iter().all(|resp| resp.error.is_none()),
            "error: {:?}",
            plan_responses
                .iter()
                .filter_map(|resp| resp.error.as_ref())
                .next()
        );
    }

    #[test]
    #[ignore]
    fn init_token() {
        let simulator = simulator::Client::new();

        let owner_key_id = String::from("owner");
        let owner_key = Key::Ed25519(owner_key_id.clone());

        let mut plan = Plan::new(owner_key_id);

        plan.add_step(Step::create_key(owner_key.clone()));
        let program_id = plan.add_step(Step {
            endpoint: Endpoint::Execute,
            method: "program_create".into(),
            max_units: 0,
            params: vec![Param::String(PROGRAM_PATH.into())],
            require: None,
        });

        plan.add_step(Step {
            endpoint: Endpoint::Execute,
            method: "init".into(),
            params: vec![program_id.into()],
            max_units: 1000000,
            require: None,
        });

        plan.add_step(Step {
            endpoint: Endpoint::ReadOnly,
            method: "get_total_supply".into(),
            max_units: 0,
            params: vec![program_id.into()],
            require: Some(Require {
                result: ResultAssertion::NumericEq(INITIAL_SUPPLY as u64),
            }),
        });

        let plan_responses = simulator.run_plan(plan).unwrap();

        assert!(
            plan_responses.iter().all(|resp| resp.error.is_none()),
            "error: {:?}",
            plan_responses
                .iter()
                .filter_map(|resp| resp.error.as_ref())
                .next()
        );
    }

    #[test]
    #[ignore]
    fn mint() {
        let simulator = simulator::Client::new();

        let owner_key_id = String::from("owner");
        let [alice_key] = ["alice"]
            .map(String::from)
            .map(Key::Ed25519)
            .map(Param::Key);
        let alice_initial_balance = 1000;

        let mut plan = Plan::new(owner_key_id.clone());

        plan.add_step(Step::create_key(Key::Ed25519(owner_key_id)));

        let program_id = plan.add_step(Step {
            endpoint: Endpoint::Execute,
            method: "program_create".into(),
            max_units: 0,
            params: vec![Param::String(PROGRAM_PATH.into())],
            require: None,
        });

        plan.add_step(Step {
            endpoint: Endpoint::Key,
            method: "key_create".into(),
            params: vec![alice_key.clone()],
            max_units: 0,
            require: None,
        });

        plan.add_step(Step {
            endpoint: Endpoint::Execute,
            method: "init".into(),
            params: vec![program_id.into()],
            max_units: 1000000,
            require: None,
        });

        plan.add_step(Step {
            endpoint: Endpoint::Execute,
            method: "mint_to".into(),
            params: vec![
                program_id.into(),
                alice_key.clone(),
                Param::U64(alice_initial_balance),
            ],
            max_units: 1000000,
            require: None,
        });

        plan.add_step(Step {
            endpoint: Endpoint::ReadOnly,
            method: "get_balance".into(),
            max_units: 0,
            params: vec![program_id.into(), alice_key],
            require: Some(Require {
                result: ResultAssertion::NumericEq(alice_initial_balance),
            }),
        });

        let plan_responses = simulator.run_plan(plan).unwrap();

        assert!(
            plan_responses.iter().all(|resp| resp.error.is_none()),
            "error: {:?}",
            plan_responses
                .iter()
                .filter_map(|resp| resp.error.as_ref())
                .next()
        );
    }

    #[test]
    #[ignore]
    fn mint_and_transfer() {
        let simulator = simulator::Client::new();

        let owner_key_id = String::from("owner");
        let [alice_key, bob_key] = ["alice", "bob"]
            .map(String::from)
            .map(Key::Ed25519)
            .map(Param::Key);
        let alice_initial_balance = 1000;
        let transfer_amount = 100;
        let post_transfer_balance = alice_initial_balance - transfer_amount;

        let mut plan = Plan::new(owner_key_id.clone());

        plan.add_step(Step::create_key(Key::Ed25519(owner_key_id)));

        let program_id = plan.add_step(Step {
            endpoint: Endpoint::Execute,
            method: "program_create".into(),
            max_units: 0,
            params: vec![Param::String(PROGRAM_PATH.into())],
            require: None,
        });

        plan.add_step(Step {
            endpoint: Endpoint::Key,
            method: "key_create".into(),
            params: vec![alice_key.clone()],
            max_units: 0,
            require: None,
        });

        plan.add_step(Step {
            endpoint: Endpoint::Key,
            method: "key_create".into(),
            params: vec![bob_key.clone()],
            max_units: 0,
            require: None,
        });

        plan.add_step(Step {
            endpoint: Endpoint::Execute,
            method: "init".into(),
            params: vec![program_id.into()],
            max_units: 1000000,
            require: None,
        });

        plan.add_step(Step {
            endpoint: Endpoint::Execute,
            method: "mint_to".into(),
            params: vec![
                program_id.into(),
                alice_key.clone(),
                Param::U64(alice_initial_balance),
            ],
            max_units: 1000000,
            require: None,
        });

        plan.add_step(Step {
            endpoint: Endpoint::Execute,
            method: "transfer".into(),
            params: vec![
                program_id.into(),
                alice_key.clone(),
                bob_key.clone(),
                Param::U64(transfer_amount),
            ],
            max_units: 1000000,
            require: None,
        });

        plan.add_step(Step {
            endpoint: Endpoint::ReadOnly,
            method: "get_total_supply".into(),
            max_units: 0,
            params: vec![program_id.into()],
            require: Some(Require {
                result: ResultAssertion::NumericEq(INITIAL_SUPPLY as u64),
            }),
        });

        plan.add_step(Step {
            endpoint: Endpoint::ReadOnly,
            method: "get_balance".into(),
            max_units: 0,
            params: vec![program_id.into(), alice_key.clone()],
            require: Some(Require {
                result: ResultAssertion::NumericEq(post_transfer_balance),
            }),
        });

        plan.add_step(Step {
            endpoint: Endpoint::ReadOnly,
            method: "get_balance".into(),
            max_units: 0,
            params: vec![program_id.into(), bob_key],
            require: Some(Require {
                result: ResultAssertion::NumericEq(transfer_amount),
            }),
        });

        plan.add_step(Step {
            endpoint: Endpoint::Execute,
            method: "burn_from".into(),
            params: vec![program_id.into(), alice_key.clone()],
            max_units: 1000000,
            require: Some(Require {
                result: ResultAssertion::NumericEq(post_transfer_balance),
            }),
        });

        plan.add_step(Step {
            endpoint: Endpoint::ReadOnly,
            method: "get_balance".into(),
            max_units: 0,
            params: vec![program_id.into(), alice_key],
            require: Some(Require {
                result: ResultAssertion::NumericEq(0),
            }),
        });

        let plan_responses = simulator.run_plan(plan).unwrap();

        assert!(
            plan_responses.iter().all(|resp| resp.error.is_none()),
            "error: {:?}",
            plan_responses
                .iter()
                .filter_map(|resp| resp.error.as_ref())
                .next()
        );
    }
}
