// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

#[cfg(not(feature = "bindings"))]
use wasmlanche_sdk::Context;
use wasmlanche_sdk::{public, state_schema, Address};

pub type Units = u64;

state_schema! {
    /// The total supply of the token. Key prefix 0x0.
    TotalSupply => Units,
    /// The name of the token. Key prefix 0x1.
    Name => String,
    /// The symbol of the token. Key prefix 0x2.
    Symbol => String,
    /// The balance of the token by address. Key prefix 0x3 + address.
    Balance(Address) => Units,
    /// The allowance of the token by owner and spender. Key prefix 0x4 + owner + spender.
    Allowance(Address, Address) => Units,
    // Original owner of the token
    Owner => Address,
}

/// Initializes the program with a name, symbol, and total supply.
#[public]
pub fn init(context: &mut Context, name: String, symbol: String) {
    let actor = context.actor();

    context
        .store_by_key(Owner, actor)
        .expect("failed to store owner");

    context
        .store(((Name, name), (Symbol, symbol)))
        .expect("failed to store owner");
}

/// Returns the total supply of the token.
#[public]
pub fn total_supply(context: &mut Context) -> Units {
    context
        .get(TotalSupply)
        .expect("failed to get total supply")
        .unwrap_or_default()
}

/// Transfers balance from the token owner to the recipient.
#[public]
pub fn mint(context: &mut Context, recipient: Address, amount: Units) {
    let actor = context.actor();

    check_owner(context, actor);

    let balance = balance_of(context, recipient);
    let total_supply = total_supply(context);

    context
        .store((
            (Balance(recipient), (balance + amount)),
            (TotalSupply, (total_supply + amount)),
        ))
        .expect("failed to store balance");
}

/// Burn the token from the recipient.
#[public]
pub fn burn(context: &mut Context, recipient: Address, value: Units) -> Units {
    let actor = context.actor();

    check_owner(context, actor);

    let total = balance_of(context, recipient);

    assert!(value <= total, "address doesn't have enough tokens to burn");

    let new_amount = total - value;

    context
        .store_by_key(Balance(recipient), new_amount)
        .expect("failed to burn recipient tokens");

    new_amount
}

/// Gets the balance of the recipient.
#[public]
pub fn balance_of(context: &mut Context, account: Address) -> Units {
    context
        .get(Balance(account))
        .expect("failed to get balance")
        .unwrap_or_default()
}

/// Returns the allowance of the spender for the owner's tokens.
#[public]
pub fn allowance(context: &mut Context, owner: Address, spender: Address) -> Units {
    context
        .get(Allowance(owner, spender))
        .expect("failed to get allowance")
        .unwrap_or_default()
}

/// Approves the spender to spend the owner's tokens.
#[public]
pub fn approve(context: &mut Context, spender: Address, amount: Units) {
    let actor = context.actor();

    context
        .store_by_key(Allowance(actor, spender), amount)
        .expect("failed to store allowance");
}

/// Transfers balance from the sender to the the recipient.
#[public]
pub fn transfer(context: &mut Context, recipient: Address, amount: Units) {
    let sender = context.actor();

    internal::transfer(context, sender, recipient, amount);
}

/// Transfers balance from the sender to the recipient.
/// The caller must have an allowance to spend the senders tokens.
#[public]
pub fn transfer_from(context: &mut Context, sender: Address, recipient: Address, amount: Units) {
    assert_ne!(sender, recipient, "sender and recipient must be different");

    let actor = context.actor();

    let total_allowance = allowance(context, sender, actor);
    assert!(total_allowance >= amount, "insufficient allowance");

    context
        .store_by_key(Allowance(sender, actor), total_allowance - amount)
        .expect("failed to store allowance");

    internal::transfer(context, sender, recipient, amount);
}

#[public]
pub fn transfer_ownership(context: &mut Context, new_owner: Address) {
    check_owner(context, context.actor());

    context
        .store_by_key(Owner, new_owner)
        .expect("failed to store owner");
}

#[public]
// grab the symbol of the token
pub fn symbol(context: &mut Context) -> String {
    context
        .get(Symbol)
        .expect("failed to get symbol")
        .expect("symbol not initialized")
}

#[public]
// grab the name of the token
pub fn name(context: &mut Context) -> String {
    context
        .get(Name)
        .expect("failed to get name")
        .expect("name not initialized")
}

// Checks if the caller is the owner of the token
// If the caller is not the owner, the program will panic
#[cfg(not(feature = "bindings"))]
fn check_owner(context: &mut Context, actor: Address) {
    assert_eq!(get_owner(context), actor, "caller is required to be owner")
}

// Returns the owner of the token
#[cfg(not(feature = "bindings"))]
fn get_owner(context: &mut Context) -> Address {
    context
        .get(Owner)
        .expect("failed to get owner")
        .expect("owner not initialized")
}

#[cfg(not(feature = "bindings"))]
mod internal {
    use super::*;

    pub fn transfer(context: &mut Context, sender: Address, recipient: Address, amount: Units) {
        // ensure the sender has adequate balance
        let sender_balance = balance_of(context, sender);

        assert!(sender_balance >= amount, "sender has insufficient balance");

        let recipient_balance = balance_of(context, recipient);

        context
            .store((
                (Balance(sender), (sender_balance - amount)),
                (Balance(recipient), (recipient_balance + amount)),
            ))
            .expect("failed to update balances");
    }
}

#[cfg(test)]
mod tests {
    use super::Units;
    use simulator::{SimpleState, Simulator};
    use wasmlanche_sdk::Address;

    const PROGRAM_PATH: &str = env!("PROGRAM_PATH");

    const MAX_UNITS: u64 = 1000000;
    #[test]
    fn create_program() {
        let mut state = SimpleState::new();
        let simulator = Simulator::new(&mut state);

        simulator.create_program(PROGRAM_PATH).unwrap()
    }

    #[test]
    // initialize the token, check that the statekeys are set to the correct values
    fn init_token() {
        let mut state = SimpleState::new();
        let simulator = Simulator::new(&mut state);

        let program_address = simulator.create_program(PROGRAM_PATH).program().unwrap();

        simulator
            .call_program(program_address, "init", ("Test", "TST"), MAX_UNITS)
            .unwrap();

        let supply = simulator
            .call_program(program_address, "total_supply", (), MAX_UNITS)
            .result::<Units>()
            .unwrap();
        assert_eq!(supply, 0);

        let symbol = simulator
            .call_program(program_address, "symbol", (), MAX_UNITS)
            .result::<String>()
            .unwrap();
        assert_eq!(symbol, "TST");

        let name = simulator
            .call_program(program_address, "name", (), MAX_UNITS)
            .result::<String>()
            .unwrap();
        assert_eq!(name, "Test");
    }

    #[test]
    fn mint() {
        let mut state = SimpleState::new();
        let simulator = Simulator::new(&mut state);

        let alice = Address::new([1; 33]);
        let alice_initial_balance = 1000;

        let program_address = simulator.create_program(PROGRAM_PATH).program().unwrap();

        simulator
            .call_program(program_address, "init", ("Test", "TST"), MAX_UNITS)
            .unwrap();

        simulator
            .call_program(
                program_address,
                "mint",
                (alice, alice_initial_balance),
                MAX_UNITS,
            )
            .unwrap();

        let balance = simulator
            .call_program(program_address, "balance_of", (alice,), MAX_UNITS)
            .result::<Units>()
            .unwrap();
        assert_eq!(balance, alice_initial_balance);

        let total_supply = simulator
            .call_program(program_address, "total_supply", (), MAX_UNITS)
            .result::<Units>()
            .unwrap();
        assert_eq!(total_supply, alice_initial_balance);
    }

    #[test]
    fn burn() {
        let mut state = SimpleState::new();
        let simulator = Simulator::new(&mut state);

        let alice = Address::new([1; 33]);
        let alice_initial_balance = 1000;
        let alice_burn_amount = 100;

        let program_address = simulator.create_program(PROGRAM_PATH).program().unwrap();

        simulator
            .call_program(program_address, "init", ("Test", "TST"), MAX_UNITS)
            .unwrap();

        simulator
            .call_program(
                program_address,
                "mint",
                (alice, alice_initial_balance),
                MAX_UNITS,
            )
            .unwrap();

        simulator
            .call_program(
                program_address,
                "burn",
                (alice, alice_burn_amount),
                MAX_UNITS,
            )
            .unwrap();

        let balance = simulator
            .call_program(program_address, "balance_of", (alice,), MAX_UNITS)
            .result::<Units>()
            .unwrap();
        assert_eq!(balance, alice_initial_balance - alice_burn_amount);
    }
}