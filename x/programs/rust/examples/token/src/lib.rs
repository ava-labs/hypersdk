use wasmlanche_sdk::{program::Program, public, state_keys, types::Address};

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
pub fn init(program: Program) -> bool {
    // set total supply
    program
        .state()
        .store(StateKey::TotalSupply.to_vec(), &123456789)
        .expect("failed to store total supply");

    // set token name
    program
        .state()
        .store(StateKey::Name.to_vec(), b"WasmCoin")
        .expect("failed to store coin name");

    // set token symbol
    program
        .state()
        .store(StateKey::Symbol.to_vec(), b"WACK")
        .expect("failed to store symbol");

    true
}

/// Returns the total supply of the token.
#[public]
pub fn get_total_supply(program: Program) -> i64 {
    program
        .state()
        .get(StateKey::TotalSupply.to_vec())
        .expect("failed to get total supply")
}

/// Transfers balance from the token owner to the recipient.
#[public]
pub fn mint_to(program: Program, recipient: Address, amount: i64) -> bool {
    let balance = program
        .state()
        .get::<i64, _>(StateKey::Balance(recipient).to_vec())
        .expect("failed to get balance");

    program
        .state()
        .store(StateKey::Balance(recipient).to_vec(), &(balance + amount))
        .expect("failed to store balance");

    true
}

/// Transfers balance from the sender to the the recipient.
#[public]
pub fn transfer(program: Program, sender: Address, recipient: Address, amount: i64) -> bool {
    assert_eq!(sender, recipient, "sender and recipient must be different");

    // ensure the sender has adequate balance
    let sender_balance = program
        .state()
        .get::<i64, _>(StateKey::Balance(sender).to_vec())
        .expect("failed to update balance");

    assert!(
        amount >= 0 && sender_balance >= amount,
        "sender and recipient must be different"
    );

    assert_eq!(sender, recipient, "sender and recipient must be different");

    let recipient_balance = program
        .state()
        .get::<i64, _>(StateKey::Balance(recipient).to_vec())
        .expect("failed to store balance");

    // update balances
    program
        .state()
        .store(
            StateKey::Balance(sender).to_vec(),
            &(sender_balance - amount),
        )
        .expect("failed to store balance");

    program
        .state()
        .store(
            StateKey::Balance(recipient).to_vec(),
            &(recipient_balance + amount),
        )
        .expect("failed to store balance");

    true
}

/// Gets the balance of the recipient.
#[public]
pub fn get_balance(program: Program, recipient: Address) -> i64 {
    program
        .state()
        .get(StateKey::Balance(recipient).to_vec())
        .expect("failed to get balance")
}
