#[cfg(not(feature = "bindings"))]
use wasmlanche_sdk::Context;
use wasmlanche_sdk::{public, state_keys, types::Address, Program};

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

// Returns the owner of the token
pub fn get_owner(program: &Program<StateKeys>) -> Address {
    program
        .state()
        .get::<Address>(StateKeys::Owner)
        .expect("failed to get owner")
        .expect("owner not initialized")
}

// Checks if the caller is the owner of the token
// If the caller is not the owner, the program will panic
pub fn check_owner(program: &Program<StateKeys>, actor: Address) {
    assert_eq!(get_owner(program), actor, "caller is required to be owner")
}

/// Initializes the program with a name, symbol, and total supply.
#[public]
pub fn init(context: Context<StateKeys>, name: String, symbol: String) {
    let program = context.program();
    let actor = context.actor();

    program
        .state()
        .store_by_key(StateKeys::Owner, &actor)
        .expect("failed to store owner");
    program
        .state()
        .store([(StateKeys::Name, &name), (StateKeys::Symbol, &symbol)])
        .expect("failed to store owner");
}

/// Returns the total supply of the token.
#[public]
pub fn total_supply(context: Context<StateKeys>) -> u64 {
    let program = context.program();
    _total_supply(program)
}

fn _total_supply(program: &Program<StateKeys>) -> u64 {
    program
        .state()
        .get(StateKeys::TotalSupply)
        .expect("failed to get total supply")
        .unwrap_or_default()
}

/// Transfers balance from the token owner to the recipient.
#[public]
pub fn mint(context: Context<StateKeys>, recipient: Address, amount: u64) -> bool {
    let program = context.program();
    let actor = context.actor();

    check_owner(program, actor);
    let balance = _balance_of(program, recipient);
    let total_supply = _total_supply(program);
    program
        .state()
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
    let program = context.program();
    let actor = context.actor();

    check_owner(program, actor);
    let total = _balance_of(program, recipient);

    assert!(value <= total, "address doesn't have enough tokens to burn");
    let new_amount = total - value;

    program
        .state()
        .store_by_key::<u64>(StateKeys::Balance(recipient), &new_amount)
        .expect("failed to burn recipient tokens");

    new_amount
}

/// Gets the balance of the recipient.
#[public]
pub fn balance_of(context: Context<StateKeys>, account: Address) -> u64 {
    let program = context.program();
    _balance_of(program, account)
}

fn _balance_of(program: &Program<StateKeys>, account: Address) -> u64 {
    program
        .state()
        .get(StateKeys::Balance(account))
        .expect("failed to get balance")
        .unwrap_or_default()
}

/// Returns the allowance of the spender for the owner's tokens.
#[public]
pub fn allowance(context: Context<StateKeys>, owner: Address, spender: Address) -> u64 {
    let program = context.program();
    _allowance(program, owner, spender)
}

pub fn _allowance(program: &Program<StateKeys>, owner: Address, spender: Address) -> u64 {
    program
        .state()
        .get::<u64>(StateKeys::Allowance(owner, spender))
        .expect("failed to get allowance")
        .unwrap_or_default()
}

/// Approves the spender to spend the owner's tokens.
/// Returns true if the approval was successful.
#[public]
pub fn approve(context: Context<StateKeys>, spender: Address, amount: u64) -> bool {
    let program = context.program();
    let actor = context.actor();

    program
        .state()
        .store_by_key(StateKeys::Allowance(actor, spender), &amount)
        .expect("failed to store allowance");
    true
}

/// Transfers balance from the sender to the the recipient.
#[public]
pub fn transfer(context: Context<StateKeys>, recipient: Address, amount: u64) -> bool {
    let program = context.program();
    let actor = context.actor();
    _transfer(program, actor, recipient, amount)
}

fn _transfer(
    program: &Program<StateKeys>,
    sender: Address,
    recipient: Address,
    amount: u64,
) -> bool {
    assert_ne!(sender, recipient, "sender and recipient must be different");

    // ensure the sender has adequate balance
    let sender_balance = _balance_of(program, sender);

    assert!(sender_balance >= amount, "sender has insufficient balance");

    let recipient_balance = _balance_of(program, recipient);

    // update balances
    program
        .state()
        .store([
            (StateKeys::Balance(sender), &(sender_balance - amount)),
            (StateKeys::Balance(recipient), &(recipient_balance + amount)),
        ])
        .expect("failed to update balances");

    true
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

    let program = context.program();
    let actor = context.actor();

    let total_allowance = _allowance(program, sender, actor);
    assert!(total_allowance >= amount, "insufficient allowance");

    program
        .state()
        .store_by_key(
            StateKeys::Allowance(sender, actor),
            &(total_allowance - amount),
        )
        .expect("failed to store allowance");

    _transfer(program, sender, recipient, amount)
}

#[public]
pub fn transfer_ownership(context: Context<StateKeys>, new_owner: Address) -> bool {
    let program = context.program();
    let actor = context.actor();

    check_owner(program, actor);
    program
        .state()
        .store_by_key(StateKeys::Owner, &new_owner)
        .expect("failed to store owner");
    true
}

#[public]
// grab the symbol of the token
pub fn symbol(context: Context<StateKeys>) -> String {
    let program = context.program();
    program
        .state()
        .get::<String>(StateKeys::Symbol)
        .expect("failed to get symbol")
        .expect("symbol not initialized")
}

#[public]
// grab the name of the token
pub fn name(context: Context<StateKeys>) -> String {
    let program = context.program();
    program
        .state()
        .get::<String>(StateKeys::Name)
        .expect("failed to get name")
        .expect("name not initialized")
}

#[cfg(test)]
mod tests {
    use simulator::{Endpoint, Key, Param, Step, TestContext};

    const PROGRAM_PATH: &str = env!("PROGRAM_PATH");

    #[test]
    fn create_program() {
        let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        let owner_key = String::from("owner");

        simulator
            .run_step(&Step::create_key(Key::Ed25519(owner_key.clone())))
            .unwrap();
        simulator
            .run_step(&Step::create_program(PROGRAM_PATH))
            .unwrap();
    }

    #[test]
    // initialize the token, check that the statekeys are set to the correct values
    fn init_token() {
        let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        let owner_key = String::from("owner");

        simulator
            .run_step(&Step::create_key(Key::Ed25519(owner_key.clone())))
            .unwrap();
        let program_id = simulator
            .run_step(&Step::create_program(PROGRAM_PATH))
            .unwrap()
            .id;

        let test_context = TestContext::from(program_id);

        simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "init".into(),
                params: vec![
                    test_context.clone().into(),
                    Param::String("Test".into()),
                    Param::String("TST".into()),
                ],
                max_units: 1000000,
            })
            .unwrap();

        let supply = simulator
            .run_step(&Step {
                endpoint: Endpoint::ReadOnly,
                method: "total_supply".into(),
                max_units: 0,
                params: vec![test_context.clone().into()],
            })
            .unwrap()
            .result
            .response::<u64>()
            .unwrap();
        assert_eq!(supply, 0);

        let symbol = simulator
            .run_step(&Step {
                endpoint: Endpoint::ReadOnly,
                method: "symbol".into(),
                max_units: 0,
                params: vec![test_context.clone().into()],
            })
            .unwrap()
            .result
            .response::<String>()
            .unwrap();
        assert_eq!(symbol, "TST");

        let name = simulator
            .run_step(&Step {
                endpoint: Endpoint::ReadOnly,
                method: "name".into(),
                max_units: 0,
                params: vec![test_context.into()],
            })
            .unwrap()
            .result
            .response::<String>()
            .unwrap();
        assert_eq!(name, "Test");
    }

    #[test]
    fn mint() {
        let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        let owner_key = String::from("owner");
        let alice_key = Key::Ed25519(String::from("alice"));
        let alice_key_param = Param::Key(alice_key.clone());
        let alice_initial_balance = 1000;

        simulator
            .run_step(&Step::create_key(Key::Ed25519(owner_key.clone())))
            .unwrap();

        let program_id = simulator
            .run_step(&Step::create_program(PROGRAM_PATH))
            .unwrap()
            .id;

        simulator.run_step(&Step::create_key(alice_key)).unwrap();

        let test_context = TestContext::from(program_id);

        simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "init".into(),
                params: vec![
                    test_context.clone().into(),
                    Param::String("Test".into()),
                    Param::String("TST".into()),
                ],
                max_units: 1000000,
            })
            .unwrap();

        simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "mint".into(),
                params: vec![
                    test_context.clone().into(),
                    alice_key_param.clone(),
                    Param::U64(alice_initial_balance),
                ],
                max_units: 1000000,
            })
            .unwrap();

        let balance = simulator
            .run_step(&Step {
                endpoint: Endpoint::ReadOnly,
                method: "balance_of".into(),
                max_units: 0,
                params: vec![test_context.clone().into(), alice_key_param],
            })
            .unwrap()
            .result
            .response::<u64>()
            .unwrap();

        assert_eq!(balance, alice_initial_balance);

        let total_supply = simulator
            .run_step(&Step {
                endpoint: Endpoint::ReadOnly,
                method: "total_supply".into(),
                max_units: 0,
                params: vec![test_context.into()],
            })
            .unwrap()
            .result
            .response::<u64>()
            .unwrap();
        assert_eq!(total_supply, alice_initial_balance);
    }

    #[test]
    fn burn() {
        let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        let owner_key = String::from("owner");
        let alice_key = Key::Ed25519(String::from("alice"));
        let alice_key_param = Param::Key(alice_key.clone());
        let alice_initial_balance = 1000;
        let alice_burn_amount = 100;

        simulator
            .run_step(&Step::create_key(Key::Ed25519(owner_key.clone())))
            .unwrap();

        let program_id = simulator
            .run_step(&Step::create_program(PROGRAM_PATH))
            .unwrap()
            .id;

        simulator.run_step(&Step::create_key(alice_key)).unwrap();

        let test_context = TestContext::from(program_id);

        simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "init".into(),
                params: vec![
                    test_context.clone().into(),
                    Param::String("Test".into()),
                    Param::String("TST".into()),
                ],
                max_units: 1000000,
            })
            .unwrap();

        simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "mint".into(),
                params: vec![
                    test_context.clone().into(),
                    alice_key_param.clone(),
                    Param::U64(alice_initial_balance),
                ],
                max_units: 1000000,
            })
            .unwrap();

        simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "burn".into(),
                max_units: 1000000,
                params: vec![
                    test_context.clone().into(),
                    alice_key_param.clone(),
                    Param::U64(alice_burn_amount),
                ],
            })
            .unwrap();

        let balance = simulator
            .run_step(&Step {
                endpoint: Endpoint::ReadOnly,
                method: "balance_of".into(),
                max_units: 0,
                params: vec![test_context.clone().into(), alice_key_param],
            })
            .unwrap()
            .result
            .response::<u64>()
            .unwrap();

        assert_eq!(balance, alice_initial_balance - alice_burn_amount);
    }
}
