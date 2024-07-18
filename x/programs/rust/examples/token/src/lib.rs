use wasmlanche_sdk::Context;
use wasmlanche_sdk::{public, state_keys, Address};

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
    /// The allowance of the token by owner and spender. Key prefix 0x4 + owner + spender.
    Allowance(Address, Address),
    // Original owner of the token
    Owner,
}

/// Initializes the program with a name, symbol, and total supply.
#[public]
pub fn init(context: Context<StateKeys>, name: String, symbol: String) {
    let actor = context.actor();

    context
        .store_by_key(StateKeys::Owner, &actor)
        .expect("failed to store owner");

    context
        .store([(StateKeys::Name, &name), (StateKeys::Symbol, &symbol)])
        .expect("failed to store owner");
}

/// Returns the total supply of the token.
#[public]
pub fn total_supply(context: Context<StateKeys>) -> u64 {
    _total_supply(&context)
}

/// Transfers balance from the token owner to the recipient.
#[public]
pub fn mint(context: Context<StateKeys>, recipient: Address, amount: u64) -> bool {
    let actor = context.actor();

    check_owner(&context, actor);
    let balance = _balance_of(&context, recipient);
    let total_supply = _total_supply(&context);
    context
        .store([
            (StateKeys::Balance(recipient), &(balance + amount)),
            (StateKeys::TotalSupply, &(total_supply + amount)),
        ])
        .expect("failed to store balance");

    true
}

/// Burn the token from the recipient.
#[public]
pub fn burn(context: Context<StateKeys>, recipient: Address, value: u64) -> u64 {
    let actor = context.actor();

    check_owner(&context, actor);
    let total = _balance_of(&context, recipient);

    assert!(value <= total, "address doesn't have enough tokens to burn");
    let new_amount = total - value;

    context
        .store_by_key::<u64>(StateKeys::Balance(recipient), &new_amount)
        .expect("failed to burn recipient tokens");

    new_amount
}

/// Gets the balance of the recipient.
#[public]
pub fn balance_of(context: Context<StateKeys>, account: Address) -> u64 {
    _balance_of(&context, account)
}

fn _balance_of(context: &Context<StateKeys>, account: Address) -> u64 {
    context
        .get(StateKeys::Balance(account))
        .expect("failed to get balance")
        .unwrap_or_default()
}

/// Returns the allowance of the spender for the owner's tokens.
#[public]
pub fn allowance(context: Context<StateKeys>, owner: Address, spender: Address) -> u64 {
    _allowance(&context, owner, spender)
}

/// Approves the spender to spend the owner's tokens.
/// Returns true if the approval was successful.
#[public]
pub fn approve(context: Context<StateKeys>, spender: Address, amount: u64) -> bool {
    let actor = context.actor();

    context
        .store_by_key(StateKeys::Allowance(actor, spender), &amount)
        .expect("failed to store allowance");
    true
}

/// Transfers balance from the sender to the the recipient.
#[public]
pub fn transfer(context: Context<StateKeys>, recipient: Address, amount: u64) -> bool {
    let actor = context.actor();
    _transfer(&context, actor, recipient, amount)
}

/// Transfers balance from the sender to the recipient.
/// The caller must have an allowance to spend the senders tokens.
#[public]
pub fn transfer_from(
    context: Context<StateKeys>,
    sender: Address,
    recipient: Address,
    amount: u64,
) -> bool {
    assert_ne!(sender, recipient, "sender and recipient must be different");

    let actor = context.actor();

    let total_allowance = _allowance(&context, sender, actor);
    assert!(total_allowance >= amount, "insufficient allowance");

    context
        .store_by_key(
            StateKeys::Allowance(sender, actor),
            &(total_allowance - amount),
        )
        .expect("failed to store allowance");

    _transfer(&context, sender, recipient, amount)
}

#[public]
pub fn transfer_ownership(context: Context<StateKeys>, new_owner: Address) -> bool {
    let actor = context.actor();

    check_owner(&context, actor);
    context
        .store_by_key(StateKeys::Owner, &new_owner)
        .expect("failed to store owner");

    true
}

#[public]
// grab the symbol of the token
pub fn symbol(context: Context<StateKeys>) -> String {
    context
        .get::<String>(StateKeys::Symbol)
        .expect("failed to get symbol")
        .expect("symbol not initialized")
}

#[public]
// grab the name of the token
pub fn name(context: Context<StateKeys>) -> String {
    context
        .get::<String>(StateKeys::Name)
        .expect("failed to get name")
        .expect("name not initialized")
}

// Checks if the caller is the owner of the token
// If the caller is not the owner, the program will panic
#[cfg(not(feature = "bindings"))]
fn check_owner(context: &Context<StateKeys>, actor: Address) {
    assert_eq!(get_owner(context), actor, "caller is required to be owner")
}

// Returns the owner of the token
#[cfg(not(feature = "bindings"))]
fn get_owner(context: &Context<StateKeys>) -> Address {
    context
        .get::<Address>(StateKeys::Owner)
        .expect("failed to get owner")
        .expect("owner not initialized")
}

fn _transfer(
    context: &Context<StateKeys>,
    sender: Address,
    recipient: Address,
    amount: u64,
) -> bool {
    assert_ne!(sender, recipient, "sender and recipient must be different");

    // ensure the sender has adequate balance
    let sender_balance = _balance_of(context, sender);

    assert!(sender_balance >= amount, "sender has insufficient balance");

    let recipient_balance = _balance_of(context, recipient);

    // update balances
    context
        .store([
            (StateKeys::Balance(sender), &(sender_balance - amount)),
            (StateKeys::Balance(recipient), &(recipient_balance + amount)),
        ])
        .expect("failed to update balances");

    true
}

pub fn _allowance(context: &Context<StateKeys>, owner: Address, spender: Address) -> u64 {
    context
        .get::<u64>(StateKeys::Allowance(owner, spender))
        .expect("failed to get allowance")
        .unwrap_or_default()
}

fn _total_supply(context: &Context<StateKeys>) -> u64 {
    context
        .get(StateKeys::TotalSupply)
        .expect("failed to get total supply")
        .unwrap_or_default()
}

#[cfg(test)]
mod tests {
    use simulator::{build_simulator, Param, TestContext};
    use wasmlanche_sdk::Address;

    const PROGRAM_PATH: &str = env!("PROGRAM_PATH");
    const MAX_UNITS: u64 = 1000000;
    #[test]
    fn create_program() {
        let mut simulator = build_simulator();

        simulator.create_program(PROGRAM_PATH).unwrap();
    }

    #[test]
    // initialize the token, check that the statekeys are set to the correct values
    fn init_token() {
        let mut simulator = build_simulator();

        let program_id = simulator.create_program(PROGRAM_PATH).unwrap().id;
        let test_context = TestContext::from(program_id);

        simulator
            .execute(
                "init".into(),
                vec![
                    test_context.clone().into(),
                    Param::String("Test".into()),
                    Param::String("TST".into()),
                ],
                MAX_UNITS,
            )
            .unwrap();

        let supply = simulator
            .read("total_supply".into(), vec![test_context.clone().into()])
            .unwrap()
            .result
            .response::<u64>()
            .unwrap();
        assert_eq!(supply, 0);

        let symbol = simulator
            .read("symbol".into(), vec![test_context.clone().into()])
            .unwrap()
            .result
            .response::<String>()
            .unwrap();

        assert_eq!(symbol, "TST");

        let name = simulator
            .read("name".into(), vec![test_context.into()])
            .unwrap()
            .result
            .response::<String>()
            .unwrap();

        assert_eq!(name, "Test");
    }

    #[test]
    fn mint() {
        let mut simulator = build_simulator();

        let alice = Address::new([1; 33]);
        let alice_initial_balance = 1000;

        let program_id = simulator.create_program(PROGRAM_PATH).unwrap().id;

        let test_context = TestContext::from(program_id);

        simulator
            .execute(
                "init".into(),
                vec![
                    test_context.clone().into(),
                    Param::String("Test".into()),
                    Param::String("TST".into()),
                ],
                MAX_UNITS,
            )
            .unwrap();

        simulator
            .execute(
                "mint".into(),
                vec![
                    test_context.clone().into(),
                    alice.into(),
                    Param::U64(alice_initial_balance),
                ],
                MAX_UNITS,
            )
            .unwrap();

        let balance = simulator
            .read(
                "balance_of".into(),
                vec![test_context.clone().into(), alice.into()],
            )
            .unwrap()
            .result
            .response::<u64>()
            .unwrap();

        assert_eq!(balance, alice_initial_balance);

        let total_supply = simulator
            .read("total_supply".into(), vec![test_context.into()])
            .unwrap()
            .result
            .response::<u64>()
            .unwrap();

        assert_eq!(total_supply, alice_initial_balance);
    }

    #[test]
    fn burn() {
        let mut simulator = build_simulator();

        let alice = Address::new([1; 33]);
        let alice_initial_balance = 1000;
        let alice_burn_amount = 100;

        let program_id = simulator.create_program(PROGRAM_PATH).unwrap().id;

        let test_context = TestContext::from(program_id);

        simulator
            .execute(
                "init".into(),
                vec![
                    test_context.clone().into(),
                    Param::String("Test".into()),
                    Param::String("TST".into()),
                ],
                MAX_UNITS,
            )
            .unwrap();

        simulator
            .execute(
                "mint".into(),
                vec![
                    test_context.clone().into(),
                    alice.into(),
                    Param::U64(alice_initial_balance),
                ],
                MAX_UNITS,
            )
            .unwrap();

        simulator
            .execute(
                "burn".into(),
                vec![
                    test_context.clone().into(),
                    alice.into(),
                    Param::U64(alice_burn_amount),
                ],
                MAX_UNITS,
            )
            .unwrap();

        let balance = simulator
            .read(
                "balance_of".into(),
                vec![test_context.clone().into(), alice.into()],
            )
            .unwrap()
            .result
            .response::<u64>()
            .unwrap();

        assert_eq!(balance, alice_initial_balance - alice_burn_amount);
    }
}
