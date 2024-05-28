use wasmlanche_sdk::Context;
use wasmlanche_sdk::{public, state_keys, types::Address, Program};

const INITIAL_SUPPLY: u64 = 123456789;

/// The program state keys.
#[state_keys]
pub enum StateKey {
    /// The total supply of the token. Key prefix 0x0.
    TotalSupply,
    /// The name of the token. Key prefix 0x1.
    Name,
    /// The symbol of the token. Key prefix 0x2.
    Symbol,
    /// The balance of the token by address. Key prefix 0x3 + address.
    Balance(Address),

    Allowance(Address, Address),

    Owner,
}

pub fn get_owner(program: &Program<StateKey>) -> Address {
    program
        .state()
        .get(StateKey::Owner)
        .expect("failed to get total supply")
}

pub fn owner_check(program: &Program<StateKey>, actor: Address) {
    assert!(
        get_owner(program) == actor,
        "caller is required to be owner"
    )
}

/// Initializes the program with a name, symbol, and total supply.
#[public]
pub fn init(context: Context<StateKey>, name: String, symbol: String, owner: Address) {
    let Context { program, actor } = context;

    program
        .state()
        .store(StateKey::Owner, &actor)
        .expect("failed to store total supply");

    // set total supply
    program
        .state()
        .store(StateKey::TotalSupply, &INITIAL_SUPPLY)
        .expect("failed to store total supply");

    // set token name
    program
        .state()
        .store(StateKey::Name, &name)
        .expect("failed to store coin name");

    // set token symbol
    program
        .state()
        .store(StateKey::Symbol, &symbol)
        .expect("failed to store symbol");
}

/// Returns the total supply of the token.
#[public]
pub fn total_supply(context: Context<StateKey>) -> u64 {
    let Context { program, .. } = context;
    program
        .state()
        .get(StateKey::TotalSupply)
        .unwrap_or_default()
}

/// Transfers balance from the token owner to the recipient.
#[public]
pub fn mint(context: Context<StateKey>, recipient: Address, amount: u64) -> bool {
    let Context { program, actor } = context;
    owner_check(&program, actor);
    let balance = program
        .state()
        .get::<u64>(StateKey::Balance(recipient))
        .unwrap_or_default();

    program
        .state()
        .store(StateKey::Balance(recipient), &(balance + amount))
        .expect("failed to store balance");

    true
}

/// Burn the token from the recipient.
#[public]
pub fn burn(context: Context<StateKey>, recipient: Address) -> u64 {
    let Context { program, actor } = context;
    owner_check(&program, actor);
    program
        .state()
        .delete::<u64>(StateKey::Balance(recipient))
        .expect("failed to burn recipient tokens")
        .expect("recipient balance not found")
}

/// Gets the balance of the recipient.
#[public]
pub fn balance_of(context: Context<StateKey>, account: Address) -> u64 {
    let Context { program, .. } = context;
    program
        .state()
        .get(StateKey::Balance(account))
        .unwrap_or_default()
}

#[public]
pub fn allowance(context: Context<StateKey>, owner: Address, spender: Address) -> u64 {
    let Context { program, .. } = context;
    program
        .state()
        .get::<u64>(StateKey::Allowance(owner, spender))
        .unwrap_or_default()
}

#[public]
pub fn approve(context: Context<StateKey>, spender: Address, amount: u64) -> bool {
    let Context { program, actor } = context;
    assert_ne!(actor, spender, "actor and spender must be different");
    program
        .state()
        .store(StateKey::Allowance(actor,spender), &amount)
        .expect("failed to store allowance");
    true
}

/// Transfers balance from the sender to the the recipient.
#[public]
pub fn transfer(context: Context<StateKey>, recipient: Address, amount: u64) -> bool {
    let Context { program, actor } = context;
    let sender = actor;
    assert_ne!(sender, recipient, "sender and recipient must be different");

    // ensure the sender has adequate balance
    let sender_balance = program
        .state()
        .get::<u64>(StateKey::Balance(sender))
        .expect("failed to update balance");

    assert!(amount >= 0 && sender_balance >= amount, "invalid input");

    let recipient_balance = program
        .state()
        .get::<u64>(StateKey::Balance(recipient))
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

#[public]
pub fn transfer_from(
    context: Context<StateKey>,
    sender: Address,
    recipient: Address,
    amount: u64,
) -> bool {
    let Context { program, actor } = context;
    let total_allowance = allowance(context, sender, actor);
    assert!(total_allowance>amount);
    program
        .state()
        .store(StateKey::Allowance(sender, actor), &(total_allowance - amount))
        .expect("failed to store allowance");
    transfer(context, recipient, amount);
    true
}
