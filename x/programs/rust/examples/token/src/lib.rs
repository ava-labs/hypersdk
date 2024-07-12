#[cfg(not(feature = "bindings"))]
use wasmlanche_sdk::Context;
use wasmlanche_sdk::{public, state_keys, types::Address, Program};

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
    /// The allowance of the token by owner and spender. Key prefix 0x4 + owner + spender.
    Allowance(Address, Address),
    // Original owner of the token
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
    let program = context.program();
    let actor = context.actor();

    program
        .state()
        .store_by_key(StateKey::Owner, &actor)
        .expect("failed to store owner");
    // initialize the owner, name, and symbol
    program
        .state()
        .store([(StateKey::Name, &name), (StateKey::Symbol, &symbol)])
        .expect("failed to store owner");
}

/// Returns the total supply of the token.
#[public]
pub fn total_supply(context: Context<StateKey>) -> u64 {
    let program = context.program();
    _total_supply(program)
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
    let program = context.program();
    let actor = context.actor();

    owner_check(&program, actor);
    let balance = program
        .state()
        .get::<u64>(StateKey::Balance(recipient))
        .expect("failed to get balance")
        .unwrap_or_default();
    let total_supply = _total_supply(&program);
    program
        .state()
        .store([
            (StateKey::Balance(recipient), &(balance + amount)),
            (StateKey::TotalSupply, &(total_supply + amount)),
        ])
        .expect("failed to store balance");

    true
}

/// Burn the token from the recipient.
#[public]
pub fn burn(context: Context<StateKey>, recipient: Address, value: u64) -> u64 {
    let program = context.program();
    let actor = context.actor();

    owner_check(&program, actor);
    let total = _balance_of(&program, recipient);

    assert!(value < total, "address doesn't have enough tokens to burn");
    let new_amount = total - value;

    program
        .state()
        .store_by_key::<u64>(StateKey::Balance(recipient), &new_amount)
        .expect("failed to burn recipient tokens");

    new_amount
}

/// Gets the balance of the recipient.
#[public]
pub fn balance_of(context: Context<StateKey>, account: Address) -> u64 {
    let program = context.program();
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
    let program = context.program();
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
    let program = context.program();
    let actor = context.actor();

    program
        .state()
        .store_by_key(StateKey::Allowance(actor, spender), &amount)
        .expect("failed to store allowance");
    true
}

/// Transfers balance from the sender to the the recipient.
#[public]
pub fn transfer(context: Context<StateKey>, recipient: Address, amount: u64) -> bool {
    let program = context.program();
    let actor = context.actor();
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
        .store([
            (StateKey::Balance(sender), &(sender_balance - amount)),
            (StateKey::Balance(recipient), &(recipient_balance + amount)),
        ])
        .expect("failed to store sender balance");

    true
}

#[public]
pub fn transfer_from(
    context: Context<StateKey>,
    sender: Address,
    recipient: Address,
    amount: u64,
) -> bool {
    let program = context.program();
    let actor = context.actor();

    let total_allowance = if sender == actor {
        _balance_of(program, actor)
    } else {
        _allowance(program, sender, actor)
    };
    println!("total allowance: {}", total_allowance);
    assert!(total_allowance >= amount);
    program
        .state()
        .store_by_key(
            StateKey::Allowance(sender, actor),
            &(total_allowance - amount),
        )
        .expect("failed to store allowance");
    _transfer(program, sender, recipient, amount)
}

#[public]
pub fn transfer_ownership(context: Context<StateKey>, new_owner: Address) -> bool {
    let program = context.program();
    let actor = context.actor();

    owner_check(&program, actor);
    program
        .state()
        .store_by_key(StateKey::Owner, &new_owner)
        .expect("failed to store owner");
    true
}


#[public]
// grab the symbol of the token
pub fn symbol(context: Context<StateKey>) -> String {
    let program = context.program();
    program
        .state()
        .get::<String>(StateKey::Symbol)
        .expect("failed to get symbol")
        .unwrap_or_default()
}

#[public]
// grab the name of the token
pub fn name(context: Context<StateKey>) -> String {
    let program = context.program();
    program
        .state()
        .get::<String>(StateKey::Name)
        .expect("failed to get name")
        .unwrap_or_default()
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

    // #[test]
    // fn transfer() {
    //     let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

    //     let owner_key = String::from("owner");
    //     let alice_key = Key::Ed25519(String::from("alice"));
    //     let alice_key_param = Param::Key(alice_key.clone());
    //     let alice_initial_balance = 1000;
    //     let alice_transfer_amount = 100;

    //     let bob_key = Key::Ed25519(String::from("gunkyman"));
    //     let bob_key_param = Param::Key(bob_key.clone());

    //     simulator
    //         .run_step(&Step::create_key(Key::Ed25519(owner_key.clone())))
    //         .unwrap();

    //     let program_id = simulator
    //         .run_step(&Step::create_program(PROGRAM_PATH))
    //         .unwrap()
    //         .id;

    //     simulator.run_step(&Step::create_key(alice_key)).unwrap();

    //     let test_context = TestContext::from(program_id);

    //     simulator
    //     .run_step(&Step {
    //         endpoint: Endpoint::Execute,
    //         method: "init".into(),
    //         params: vec![test_context.clone().into(), Param::String("Test".into()), Param::String("TST".into())],
    //         max_units: 1000000,
    //     })
    //     .unwrap();

    //     simulator
    //         .run_step(&Step {
    //             endpoint: Endpoint::Execute,
    //             method: "mint".into(),
    //             params: vec![
    //                 test_context.clone().into(),
    //                 alice_key_param.clone(),
    //                 Param::U64(alice_initial_balance),
    //             ],
    //             max_units: 1000000,
    //         })
    //         .unwrap();

    //     simulator.run_step(
    //         &Step {
    //             endpoint: Endpoint::Execute,
    //             method: "transfer_from".into(),
    //             max_units: 1000000,
    //             params: vec![test_context.clone().into(), alice_key_param.clone(), Param::Key(bob_key.clone()), Param::U64(alice_transfer_amount)],
    //                     }
    //     ).unwrap();

    //     let alice_balance = simulator
    //         .run_step(&Step {
    //             endpoint: Endpoint::ReadOnly,
    //             method: "balance_of".into(),
    //             max_units: 0,
    //             params: vec![test_context.clone().into(), alice_key_param],
    //         })
    //         .unwrap()
    //         .result
    //         .response::<u64>()
    //         .unwrap();
    //     assert_eq!(alice_balance, alice_initial_balance - alice_transfer_amount);

    //     let bob_balance = simulator
    //         .run_step(&Step {
    //             endpoint: Endpoint::ReadOnly,
    //             method: "balance_of".into(),
    //             max_units: 0,
    //             params: vec![test_context.clone().into(), bob_key_param],
    //         })
    //         .unwrap()
    //         .result
    //         .response::<u64>()
    //         .unwrap();
    //     assert_eq!(bob_balance, alice_transfer_amount);
    // }
}
