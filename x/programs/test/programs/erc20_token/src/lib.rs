#[cfg(not(feature = "bindings"))]
use wasmlanche_sdk::Context;
use wasmlanche_sdk::{public, state_keys, types::Address, Program};

#[cfg(not(feature = "bindings"))]
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

pub fn get_owner(program: &Program<StateKey>) -> Option<Address> {
    program
        .state()
        .get::<Address>(StateKey::Owner)
        .expect("failure")
}

pub fn owner_check(program: &Program<StateKey>, actor: Address) {
    assert!(
        match get_owner(program) {
            None => true,
            Some(owner) => owner == actor,
        },
        "caller is required to be owner"
    )
}

/// Initializes the program with a name, symbol, and total supply.
#[public]
pub fn init(context: Context<StateKey>, name: String, symbol: String) {
    let Context { program, actor, .. } = context;

    program
        .state()
        .store(StateKey::Owner, &actor)
        .expect("failed to store owner");

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
    _total_supply(&program)
}

fn _total_supply(program: &Program<StateKey>) -> u64 {
    program
        .state()
        .get(StateKey::TotalSupply)
        .expect("failure")
        .unwrap_or_default()
}

/// Transfers balance from the token owner to the recipient.
#[public]
pub fn mint(context: Context<StateKey>, recipient: Address, amount: u64) -> bool {
    let Context { program, actor, .. } = context;
    owner_check(&program, actor);
    let balance = program
        .state()
        .get::<u64>(StateKey::Balance(recipient))
        .expect("failed to get balance")
        .unwrap_or_default();

    program
        .state()
        .store(StateKey::Balance(recipient), &(balance + amount))
        .expect("failed to store balance");

    true
}

/// Burn the token from the recipient.
#[public]
pub fn burn(context: Context<StateKey>, recipient: Address, value: u64) -> u64 {
    let Context { program, actor, .. } = context;
    owner_check(&program, actor);
    let total = _balance_of(&program, recipient);

    assert!(value < total, "address doesn't have enough tokens to burn");
    let new_amount = total - value;

    program
        .state()
        .store::<u64>(StateKey::Balance(recipient), &new_amount)
        .expect("failed to burn recipient tokens");

    new_amount
}

/// Gets the balance of the recipient.
#[public]
pub fn balance_of(context: Context<StateKey>, account: Address) -> u64 {
    let Context { program, .. } = context;
    _balance_of(&program, account)
}

fn _balance_of(program: &Program<StateKey>, account: Address) -> u64 {
    program
        .state()
        .get(StateKey::Balance(account))
        .expect("failure")
        .unwrap_or_default()
}

#[public]
pub fn allowance(context: Context<StateKey>, owner: Address, spender: Address) -> u64 {
    let Context { program, .. } = context;
    _allowance(&program, owner, spender)
}

pub fn _allowance(program: &Program<StateKey>, owner: Address, spender: Address) -> u64 {
    program
        .state()
        .get::<u64>(StateKey::Allowance(owner, spender))
        .expect("failure")
        .unwrap_or_default()
}

#[public]
pub fn approve(context: Context<StateKey>, spender: Address, amount: u64) -> bool {
    let Context { program, actor, .. } = context;
    assert_ne!(actor, spender, "actor and spender must be different");
    program
        .state()
        .store(StateKey::Allowance(actor, spender), &amount)
        .expect("failed to store allowance");
    true
}

/// Transfers balance from the sender to the the recipient.
#[public]
pub fn transfer(context: Context<StateKey>, recipient: Address, amount: u64) -> bool {
    let Context { program, actor, .. } = context;
    _transfer(&program, actor, recipient, amount)
}

fn _transfer(
    program: &Program<StateKey>,
    sender: Address,
    recipient: Address,
    amount: u64,
) -> bool {
    assert_ne!(sender, recipient, "sender and recipient must be different");

    // ensure the sender has adequate balance
    let sender_balance = program
        .state()
        .get::<u64>(StateKey::Balance(sender))
        .expect("failed to get sender balance")
        .unwrap_or_default();

    assert!(sender_balance >= amount, "invalid input");

    let recipient_balance = program
        .state()
        .get::<u64>(StateKey::Balance(recipient))
        .expect("failed to store recipient balance")
        .unwrap_or_default();

    // update balances
    program
        .state()
        .store(StateKey::Balance(sender), &(sender_balance - amount))
        .expect("failed to store sender balance");

    program
        .state()
        .store(StateKey::Balance(recipient), &(recipient_balance + amount))
        .expect("failed to store recipient balance");

    true
}

#[public]
pub fn transfer_from(
    context: Context<StateKey>,
    sender: Address,
    recipient: Address,
    amount: u64,
) -> bool {
    let Context {
        ref program, actor, ..
    } = context;
    let total_allowance = _allowance(program, sender, actor);
    assert!(total_allowance >= amount);
    program
        .state()
        .store(
            StateKey::Allowance(sender, actor),
            &(total_allowance - amount),
        )
        .expect("failed to store allowance");
    _transfer(program, sender, recipient, amount)
}
