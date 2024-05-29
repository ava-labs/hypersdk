use borsh::{BorshDeserialize, BorshSerialize};
use wasmlanche_sdk::Context;
use wasmlanche_sdk::{public, state_keys, types::Address};

const INITIAL_SUPPLY: u64 = 123456789;

/// The program state keys.
#[state_keys]
pub enum StateKeys {
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
pub fn init(context: Context<StateKeys>) {
    let Context { program, .. } = context;

    // set total supply
    program
        .state()
        .store(StateKeys::TotalSupply, &INITIAL_SUPPLY)
        .expect("failed to store total supply");

    // set token name
    program
        .state()
        .store(StateKeys::Name, b"WasmCoin")
        .expect("failed to store coin name");

    // set token symbol
    program
        .state()
        .store(StateKeys::Symbol, b"WACK")
        .expect("failed to store symbol");
}

/// Returns the total supply of the token.
#[public]
pub fn get_total_supply(context: Context<StateKeys>) -> u64 {
    let Context { program, .. } = context;

    program
        .state()
        .get(StateKeys::TotalSupply)
        .expect("failed to get total supply")
        .unwrap_or_default()
}

/// Transfers balance from the token owner to the recipient.
#[public]
pub fn mint_to(context: Context<StateKeys>, recipient: Address, amount: u64) -> bool {
    mint_to_internal(context, recipient, amount);
    true
}

fn mint_to_internal(
    context: Context<StateKeys>,
    recipient: Address,
    amount: u64,
) -> Context<StateKeys> {
    let program = &context.program;

    let balance = program
        .state()
        .get::<u64>(StateKeys::Balance(recipient))
        .expect("state corrupt")
        .unwrap_or_default();

    program
        .state()
        .store(StateKeys::Balance(recipient), &(balance + amount))
        .expect("failed to store balance");

    context
}

/// Burn the token from the recipient.
#[public]
pub fn burn_from(context: Context<StateKeys>, recipient: Address) -> u64 {
    let Context { program, .. } = context;
    program
        .state()
        .delete::<u64>(StateKeys::Balance(recipient))
        .expect("failed to burn recipient tokens")
        .expect("recipient balance not found")
}

/// Transfers balance from the sender to the recipient.
#[public]
pub fn transfer(
    context: Context<StateKeys>,
    sender: Address,
    recipient: Address,
    amount: u64,
) -> bool {
    let Context { program, .. } = context;
    assert_ne!(sender, recipient, "sender and recipient must be different");

    // ensure the sender has adequate balance
    let sender_balance = program
        .state()
        .get::<u64>(StateKeys::Balance(sender))
        .expect("state corrupt")
        .unwrap_or_default();

    assert!(sender_balance >= amount, "invalid input");

    let recipient_balance = program
        .state()
        .get::<u64>(StateKeys::Balance(recipient))
        .expect("state corrupt")
        .unwrap_or_default();

    // update balances
    program
        .state()
        .store(StateKeys::Balance(sender), &(sender_balance - amount))
        .expect("failed to store balance");

    program
        .state()
        .store(StateKeys::Balance(recipient), &(recipient_balance + amount))
        .expect("failed to store balance");

    true
}

#[derive(BorshDeserialize, BorshSerialize)]
pub struct Minter {
    to: Address,
    amount: u64,
}

/// Mints tokens to multiple recipients.
#[public]
pub fn mint_to_many(context: Context<StateKeys>, minters: Vec<Minter>) -> bool {
    minters.into_iter().fold(context, |context, minter| {
        mint_to_internal(context, minter.to, minter.amount)
    });
    true
}

/// Gets the balance of the recipient.
#[public]
pub fn get_balance(context: Context<StateKeys>, recipient: Address) -> i64 {
    let Context { program, .. } = context;
    program
        .state()
        .get(StateKeys::Balance(recipient))
        .expect("state corrupt")
        .unwrap_or_default()
}

#[cfg(test)]
mod tests {
    use crate::INITIAL_SUPPLY;
    use simulator::{Endpoint, Key, Param, Plan, Step};

    const PROGRAM_PATH: &str = env!("PROGRAM_PATH");

    #[test]
    fn create_program() {
        let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        let owner_key = String::from("owner");

        let mut plan = Plan::new(&owner_key);

        plan.add_step(Step::create_key(Key::Ed25519(owner_key.clone())));
        plan.add_step(Step::create_program(PROGRAM_PATH));

        let plan_responses = simulator.run_plan(plan).unwrap();

        assert!(
            plan_responses.iter().all(|resp| resp.base.error.is_none()),
            "error: {:?}",
            plan_responses
                .iter()
                .filter_map(|resp| resp.base.error.as_ref())
                .next()
        );
    }

    #[test]
    fn init_token() {
        let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        let owner_key_id = String::from("owner");
        let owner_key = Key::Ed25519(owner_key_id.clone());

        let mut plan = Plan::new(&owner_key_id);

        plan.add_step(Step::create_key(owner_key.clone()));
        let program_id = plan.add_step(Step {
            endpoint: Endpoint::Execute,
            method: "program_create".into(),
            max_units: 0,
            params: vec![Param::String(PROGRAM_PATH.into())],
        });

        plan.add_step(Step {
            endpoint: Endpoint::Execute,
            method: "init".into(),
            params: vec![program_id.into()],
            max_units: 1000000,
        });

        let plan_responses = simulator.run_plan(plan).unwrap();

        assert!(
            plan_responses.iter().all(|resp| resp.base.error.is_none()),
            "error: {:?}",
            plan_responses
                .iter()
                .filter_map(|resp| resp.base.error.as_ref())
                .next()
        );

        let supply = simulator
            .run_step::<u64>(
                &owner_key_id,
                &Step {
                    endpoint: Endpoint::ReadOnly,
                    method: "get_total_supply".into(),
                    max_units: 0,
                    params: vec![program_id.into()],
                },
            )
            .unwrap()
            .result
            .response;

        assert_eq!(supply, INITIAL_SUPPLY as u64);
    }

    #[test]
    fn mint() {
        let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        let owner_key_id = String::from("owner");
        let [alice_key] = ["alice"]
            .map(String::from)
            .map(Key::Ed25519)
            .map(Param::Key);
        let alice_initial_balance = 1000;

        let mut plan = Plan::new(&owner_key_id);

        plan.add_step(Step::create_key(Key::Ed25519(owner_key_id.clone())));

        let program_id = plan.add_step(Step {
            endpoint: Endpoint::Execute,
            method: "program_create".into(),
            max_units: 0,
            params: vec![Param::String(PROGRAM_PATH.into())],
        });

        plan.add_step(Step {
            endpoint: Endpoint::Key,
            method: "key_create".into(),
            params: vec![alice_key.clone()],
            max_units: 0,
        });

        plan.add_step(Step {
            endpoint: Endpoint::Execute,
            method: "init".into(),
            params: vec![program_id.into()],
            max_units: 1000000,
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
        });

        let plan_responses = simulator.run_plan(plan).unwrap();

        assert!(
            plan_responses.iter().all(|resp| resp.base.error.is_none()),
            "error: {:?}",
            plan_responses
                .iter()
                .filter_map(|resp| resp.base.error.as_ref())
                .next()
        );

        let balance = simulator
            .run_step::<u64>(
                &owner_key_id,
                &Step {
                    endpoint: Endpoint::ReadOnly,
                    method: "get_balance".into(),
                    max_units: 0,
                    params: vec![program_id.into(), alice_key],
                },
            )
            .unwrap()
            .result
            .response;

        assert_eq!(balance, alice_initial_balance);
    }

    #[test]
    fn mint_and_transfer() {
        let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        let owner_key_id = String::from("owner");
        let [alice_key, bob_key] = ["alice", "bob"]
            .map(String::from)
            .map(Key::Ed25519)
            .map(Param::Key);
        let alice_initial_balance = 1000;
        let transfer_amount = 100;
        let post_transfer_balance = alice_initial_balance - transfer_amount;

        let mut plan = Plan::new(&owner_key_id);

        plan.add_step(Step::create_key(Key::Ed25519(owner_key_id.clone())));

        let program_id = plan.add_step(Step {
            endpoint: Endpoint::Execute,
            method: "program_create".into(),
            max_units: 0,
            params: vec![Param::String(PROGRAM_PATH.into())],
        });

        plan.add_step(Step {
            endpoint: Endpoint::Key,
            method: "key_create".into(),
            params: vec![alice_key.clone()],
            max_units: 0,
        });

        plan.add_step(Step {
            endpoint: Endpoint::Key,
            method: "key_create".into(),
            params: vec![bob_key.clone()],
            max_units: 0,
        });

        plan.add_step(Step {
            endpoint: Endpoint::Execute,
            method: "init".into(),
            params: vec![program_id.into()],
            max_units: 1000000,
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
        });

        let plan_responses = simulator.run_plan(plan).unwrap();

        assert!(
            plan_responses.iter().all(|resp| resp.base.error.is_none()),
            "error: {:?}",
            plan_responses
                .iter()
                .filter_map(|resp| resp.base.error.as_ref())
                .next()
        );

        let supply = simulator
            .run_step::<u64>(
                &owner_key_id,
                &Step {
                    endpoint: Endpoint::ReadOnly,
                    method: "get_total_supply".into(),
                    max_units: 0,
                    params: vec![program_id.into()],
                },
            )
            .unwrap()
            .result
            .response;
        assert_eq!(supply, INITIAL_SUPPLY as u64);

        let balance = simulator
            .run_step::<u64>(
                &owner_key_id,
                &Step {
                    endpoint: Endpoint::ReadOnly,
                    method: "get_balance".into(),
                    max_units: 0,
                    params: vec![program_id.into(), alice_key.clone()],
                },
            )
            .unwrap()
            .result
            .response;
        assert_eq!(balance, post_transfer_balance);

        let balance = simulator
            .run_step::<u64>(
                &owner_key_id,
                &Step {
                    endpoint: Endpoint::ReadOnly,
                    method: "get_balance".into(),
                    max_units: 0,
                    params: vec![program_id.into(), bob_key],
                },
            )
            .unwrap()
            .result
            .response;
        assert_eq!(balance, transfer_amount);

        let balance = simulator
            .run_step::<u64>(
                &owner_key_id,
                &Step {
                    endpoint: Endpoint::Execute,
                    method: "burn_from".into(),
                    params: vec![program_id.into(), alice_key.clone()],
                    max_units: 1000000,
                },
            )
            .unwrap()
            .result
            .response;
        assert_eq!(balance, post_transfer_balance);

        let balance = simulator
            .run_step::<u64>(
                &owner_key_id,
                &Step {
                    endpoint: Endpoint::ReadOnly,
                    method: "get_balance".into(),
                    max_units: 0,
                    params: vec![program_id.into(), alice_key],
                },
            )
            .unwrap()
            .result
            .response;
        assert_eq!(balance, 0);
    }
}
