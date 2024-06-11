use std::cmp::Ordering;

use borsh::{BorshDeserialize, BorshSerialize};
#[cfg(not(feature = "bindings"))]
use wasmlanche_sdk::Context;
use wasmlanche_sdk::{public, state_keys, types::Address, Program};

const INITIAL_SUPPLY: u64 = 123456789;

#[derive(BorshSerialize)]
pub enum TokenError {
    InsufficientBalance,
}

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
    /// The allowance given from an address to an address. Key prefix 0x4 + address + address.
    Allowance(Address, Address),
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

#[cfg(not(feature = "bindings"))]
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

#[public]
pub fn burn(context: Context<StateKeys>, amount: u64) -> Result<(), TokenError> {
    let Context { program, actor, .. } = context;
    _burn_from(&program, actor, amount)
}

#[public]
pub fn burn_from(
    context: Context<StateKeys>,
    from: Address,
    amount: u64,
) -> Result<(), TokenError> {
    let Context { program, actor, .. } = context;
    let allowance = _allowance(&program, from, actor);
    if allowance >= amount {
        _set_allowance(&program, from, actor, allowance - amount);
        _burn_from(&program, from, amount)?;
    }
    Ok(())
}

fn _burn_from(program: &Program<StateKeys>, from: Address, amount: u64) -> Result<(), TokenError> {
    let balance = _balance_of(program, from);
    match balance.cmp(&amount) {
        Ordering::Less => return Err(TokenError::InsufficientBalance),
        Ordering::Equal => {
            program
                .state()
                .delete::<u64>(StateKeys::Balance(from))
                .expect("state corrupt");
        }
        Ordering::Greater => program
            .state()
            .store(StateKeys::Balance(from), &(balance - amount))
            .expect("state corrupt"),
    }
    Ok(())
}

#[public]
pub fn transfer(context: Context<StateKeys>, to: Address, amount: u64) -> Result<(), TokenError> {
    let Context { program, actor, .. } = context;
    _transfer_from(&program, actor, to, amount)
}

/// Transfers balance from the sender to the recipient.
#[public]
pub fn transfer_from(
    context: Context<StateKeys>,
    from: Address,
    to: Address,
    amount: u64,
) -> Result<(), TokenError> {
    let Context { program, .. } = context;
    let allowance = _allowance(&program, from, to);
    if allowance >= amount {
        _set_allowance(&program, from, to, allowance - amount);
        _transfer_from(&program, from, to, amount)?;
    }
    Ok(())
}

fn _transfer_from(
    program: &Program<StateKeys>,
    from: Address,
    to: Address,
    amount: u64,
) -> Result<(), TokenError> {
    assert_ne!(from, to, "sender and recipient must be different");

    // ensure the sender has adequate balance
    let sender_balance = program
        .state()
        .get::<u64>(StateKeys::Balance(from))
        .expect("state corrupt")
        .unwrap_or_default();

    if sender_balance < amount {
        return Err(TokenError::InsufficientBalance);
    }

    let recipient_balance = program
        .state()
        .get::<u64>(StateKeys::Balance(to))
        .expect("state corrupt")
        .unwrap_or_default();

    // update balances
    program
        .state()
        .store(StateKeys::Balance(from), &(sender_balance - amount))
        .expect("failed to store balance");

    program
        .state()
        .store(StateKeys::Balance(to), &(recipient_balance + amount))
        .expect("failed to store balance");

    Ok(())
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

#[public]
pub fn balance_of(context: Context<StateKeys>, recipient: Address) -> u64 {
    let Context { program, .. } = context;
    _balance_of(&program, recipient)
}

pub fn _balance_of(program: &Program<StateKeys>, recipient: Address) -> u64 {
    program
        .state()
        .get(StateKeys::Balance(recipient))
        .expect("state corrupt")
        .unwrap_or_default()
}

#[public]
pub fn approve(context: Context<StateKeys>, to: Address, amount: u64) {
    let Context { program, actor } = context;
    _set_allowance(&program, actor, to, amount);
}

#[public]
pub fn allowance(context: Context<StateKeys>, from: Address, to: Address) -> u64 {
    let Context { program, .. } = context;
    _allowance(&program, from, to)
}

fn _allowance(program: &Program<StateKeys>, from: Address, to: Address) -> u64 {
    program
        .state()
        .get(StateKeys::Allowance(from, to))
        .expect("state corrupt")
        .unwrap_or_default()
}

fn _set_allowance(program: &Program<StateKeys>, from: Address, to: Address, amount: u64) {
    program
        .state()
        .store(StateKeys::Allowance(from, to), &amount)
        .expect("state corrupt");
}

#[cfg(test)]
mod tests {
    use crate::INITIAL_SUPPLY;
    use simulator::{Endpoint, Key, Param, Step};

    const PROGRAM_PATH: &str = env!("PROGRAM_PATH");

    #[test]
    fn create_program() {
        let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        let owner_key = String::from("owner");

        simulator
            .run_step(
                &owner_key,
                &Step::create_key(Key::Ed25519(owner_key.clone())),
            )
            .unwrap();
        simulator
            .run_step(&owner_key, &Step::create_program(PROGRAM_PATH))
            .unwrap();
    }

    #[test]
    fn init_token() {
        let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        let owner_key = String::from("owner");

        simulator
            .run_step(
                &owner_key,
                &Step::create_key(Key::Ed25519(owner_key.clone())),
            )
            .unwrap();
        let program_id = simulator
            .run_step(
                &owner_key,
                &Step {
                    endpoint: Endpoint::Execute,
                    method: "program_create".into(),
                    max_units: 0,
                    params: vec![Param::String(PROGRAM_PATH.into())],
                },
            )
            .unwrap()
            .id;

        simulator
            .run_step(
                &owner_key,
                &Step {
                    endpoint: Endpoint::Execute,
                    method: "init".into(),
                    params: vec![program_id.into()],
                    max_units: 1000000,
                },
            )
            .unwrap();

        let supply = simulator
            .run_step(
                &owner_key,
                &Step {
                    endpoint: Endpoint::ReadOnly,
                    method: "get_total_supply".into(),
                    max_units: 0,
                    params: vec![program_id.into()],
                },
            )
            .unwrap()
            .result
            .response::<u64>()
            .unwrap();

        assert_eq!(supply, INITIAL_SUPPLY);
    }

    #[test]
    fn mint() {
        let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        let owner_key = String::from("owner");
        let alice_key = Param::Key(Key::Ed25519(String::from("alice")));
        let alice_initial_balance = 1000;

        simulator
            .run_step(
                &owner_key,
                &Step::create_key(Key::Ed25519(owner_key.clone())),
            )
            .unwrap();

        let program_id = simulator
            .run_step(
                &owner_key,
                &Step {
                    endpoint: Endpoint::Execute,
                    method: "program_create".into(),
                    max_units: 0,
                    params: vec![Param::String(PROGRAM_PATH.into())],
                },
            )
            .unwrap()
            .id;

        simulator
            .run_step(
                &owner_key,
                &Step {
                    endpoint: Endpoint::Key,
                    method: "key_create".into(),
                    params: vec![alice_key.clone()],
                    max_units: 0,
                },
            )
            .unwrap();

        simulator
            .run_step(
                &owner_key,
                &Step {
                    endpoint: Endpoint::Execute,
                    method: "init".into(),
                    params: vec![program_id.into()],
                    max_units: 1000000,
                },
            )
            .unwrap();

        simulator
            .run_step(
                &owner_key,
                &Step {
                    endpoint: Endpoint::Execute,
                    method: "mint_to".into(),
                    params: vec![
                        program_id.into(),
                        alice_key.clone(),
                        Param::U64(alice_initial_balance),
                    ],
                    max_units: 1000000,
                },
            )
            .unwrap();

        let balance = simulator
            .run_step(
                &owner_key,
                &Step {
                    endpoint: Endpoint::ReadOnly,
                    method: "get_balance".into(),
                    max_units: 0,
                    params: vec![program_id.into(), alice_key],
                },
            )
            .unwrap()
            .result
            .response::<u64>()
            .unwrap();

        assert_eq!(balance, alice_initial_balance);
    }

    #[test]
    fn mint_and_transfer() {
        let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        let owner_key = String::from("owner");
        let [alice_key, bob_key] = ["alice", "bob"]
            .map(String::from)
            .map(Key::Ed25519)
            .map(Param::Key);
        let alice_initial_balance = 1000;
        let transfer_amount = 100;
        let post_transfer_balance = alice_initial_balance - transfer_amount;

        simulator
            .run_step(
                &owner_key,
                &Step::create_key(Key::Ed25519(owner_key.clone())),
            )
            .unwrap();

        let program_id = simulator
            .run_step(
                &owner_key,
                &Step {
                    endpoint: Endpoint::Execute,
                    method: "program_create".into(),
                    max_units: 0,
                    params: vec![Param::String(PROGRAM_PATH.into())],
                },
            )
            .unwrap()
            .id;

        simulator
            .run_step(
                &owner_key,
                &Step {
                    endpoint: Endpoint::Key,
                    method: "key_create".into(),
                    params: vec![alice_key.clone()],
                    max_units: 0,
                },
            )
            .unwrap();

        simulator
            .run_step(
                &owner_key,
                &Step {
                    endpoint: Endpoint::Key,
                    method: "key_create".into(),
                    params: vec![bob_key.clone()],
                    max_units: 0,
                },
            )
            .unwrap();

        simulator
            .run_step(
                &owner_key,
                &Step {
                    endpoint: Endpoint::Execute,
                    method: "init".into(),
                    params: vec![program_id.into()],
                    max_units: 1000000,
                },
            )
            .unwrap();

        simulator
            .run_step(
                &owner_key,
                &Step {
                    endpoint: Endpoint::Execute,
                    method: "mint_to".into(),
                    params: vec![
                        program_id.into(),
                        alice_key.clone(),
                        Param::U64(alice_initial_balance),
                    ],
                    max_units: 1000000,
                },
            )
            .unwrap();

        simulator
            .run_step(
                &owner_key,
                &Step {
                    endpoint: Endpoint::Execute,
                    method: "transfer".into(),
                    params: vec![
                        program_id.into(),
                        alice_key.clone(),
                        bob_key.clone(),
                        Param::U64(transfer_amount),
                    ],
                    max_units: 1000000,
                },
            )
            .unwrap();

        let supply = simulator
            .run_step(
                &owner_key,
                &Step {
                    endpoint: Endpoint::ReadOnly,
                    method: "get_total_supply".into(),
                    max_units: 0,
                    params: vec![program_id.into()],
                },
            )
            .unwrap()
            .result
            .response::<u64>()
            .unwrap();
        assert_eq!(supply, INITIAL_SUPPLY);

        let balance = simulator
            .run_step(
                &owner_key,
                &Step {
                    endpoint: Endpoint::ReadOnly,
                    method: "get_balance".into(),
                    max_units: 0,
                    params: vec![program_id.into(), alice_key.clone()],
                },
            )
            .unwrap()
            .result
            .response::<u64>()
            .unwrap();
        assert_eq!(balance, post_transfer_balance);

        let balance = simulator
            .run_step(
                &owner_key,
                &Step {
                    endpoint: Endpoint::ReadOnly,
                    method: "get_balance".into(),
                    max_units: 0,
                    params: vec![program_id.into(), bob_key],
                },
            )
            .unwrap()
            .result
            .response::<u64>()
            .unwrap();
        assert_eq!(balance, transfer_amount);

        let balance = simulator
            .run_step(
                &owner_key,
                &Step {
                    endpoint: Endpoint::Execute,
                    method: "burn_from".into(),
                    params: vec![program_id.into(), alice_key.clone()],
                    max_units: 1000000,
                },
            )
            .unwrap()
            .result
            .response::<u64>()
            .unwrap();
        assert_eq!(balance, post_transfer_balance);

        let balance = simulator
            .run_step(
                &owner_key,
                &Step {
                    endpoint: Endpoint::ReadOnly,
                    method: "get_balance".into(),
                    max_units: 0,
                    params: vec![program_id.into(), alice_key],
                },
            )
            .unwrap()
            .result
            .response::<u64>()
            .unwrap();
        assert_eq!(balance, 0);
    }
}
