// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use simulator::{SimpleState, Simulator};
use wasmlanche_sdk::Address;
const PROGRAM_PATH: &str = env!("PROGRAM_PATH");

#[test]
fn init_program() {
    let mut state = SimpleState::new();
    let simulator = Simulator::new(&mut state);

    let error = simulator.create_program(PROGRAM_PATH).has_error();
    assert!(!error, "Create program errored")
}

#[test]
fn amm_init() {
    let mut state = SimpleState::new();
    let mut simulator = Simulator::new(&mut state);
    let alice = Address::new([1; 33]);
    simulator.set_actor(alice);
    let gas = 1000000000;

    init_amm(&mut simulator, gas);
}

#[test]
fn add_liquidity_same_ratio() {
    let mut state = SimpleState::new();
    let mut simulator = Simulator::new(&mut state);
    let alice = Address::new([1; 33]);
    simulator.set_actor(alice);
    let gas = 1000000000;

    let (token_x, token_y, lt, amm) = init_amm(&mut simulator, gas);
    let amount: u64 = 100;

    simulator
        .call_program(token_x, "mint", (alice, amount), gas)
        .unwrap();
    simulator
        .call_program(token_y, "mint", (alice, amount), gas)
        .unwrap();

    let balance = simulator
        .call_program(lt, "balance_of", alice, gas)
        .result::<u64>()
        .unwrap();
    assert_eq!(balance, 0, "Balance of liquidity token is incorrect");

    simulator
        .call_program(token_x, "approve", (amm, amount), gas)
        .unwrap();
    simulator
        .call_program(token_y, "approve", (amm, amount), gas)
        .unwrap();

    let result = simulator.call_program(amm, "add_liquidity", (amount, amount), gas);
    assert!(
        !result.has_error(),
        "Add liquidity errored {:?}",
        result.error()
    );

    let balance = simulator
        .call_program(lt, "balance_of", alice, gas)
        .result::<u64>()
        .unwrap();
    assert!(balance > 0, "Balance of liquidity token is incorrect");
}

#[test]
fn add_liquidity_without_approval() {
    let mut state = SimpleState::new();
    let mut simulator = Simulator::new(&mut state);
    let alice = Address::new([1; 33]);
    simulator.set_actor(alice);
    let gas = 1000000000;

    let (token_x, token_y, lt, amm) = init_amm(&mut simulator, gas);
    let amount: u64 = 100;

    simulator
        .call_program(token_x, "mint", (alice, amount), gas)
        .unwrap();
    simulator
        .call_program(token_y, "mint", (alice, amount), gas)
        .unwrap();

    let balance = simulator
        .call_program(lt, "balance_of", alice, gas)
        .result::<u64>()
        .unwrap();
    assert_eq!(balance, 0, "Balance of liquidity token is incorrect");

    let result = simulator.call_program(amm, "add_liquidity", (amount, amount), gas);
    assert!(result.has_error(), "Add liquidity did not error");
}

#[test]
fn swap_changes_ratio() {
    let mut state = SimpleState::new();
    let mut simulator = Simulator::new(&mut state);
    let alice = Address::new([1; 33]);
    simulator.set_actor(alice);
    let gas = 1000000000;

    let (token_x, token_y, _lt, amm) = init_amm(&mut simulator, gas);
    let amount: u64 = 100;
    let amount_x_swap: u64 = 50;

    simulator
        .call_program(token_x, "mint", (alice, amount + amount_x_swap), gas)
        .unwrap();
    simulator
        .call_program(token_y, "mint", (alice, amount), gas)
        .unwrap();

    simulator
        .call_program(token_x, "approve", (amm, amount + amount_x_swap), gas)
        .unwrap();
    simulator
        .call_program(token_y, "approve", (amm, amount), gas)
        .unwrap();

    let result = simulator.call_program(amm, "add_liquidity", (amount, amount), gas);
    assert!(
        !result.has_error(),
        "Add liquidity errored {:?}",
        result.error()
    );

    let balance_y = simulator
        .call_program(token_y, "balance_of", alice, gas)
        .result::<u64>()
        .unwrap();
    assert_eq!(balance_y, 0, "Balance of token y is incorrect");

    let swap = simulator.call_program(amm, "swap", (token_x, amount_x_swap), gas);
    assert!(!swap.has_error(), "Swap errored, {:?}", swap.error());

    let swap = swap.result::<u64>().unwrap();
    assert!(swap > 0, "Swap did not return any tokens");

    let balance_x = simulator
        .call_program(token_x, "balance_of", alice, gas)
        .result::<u64>()
        .unwrap();
    assert_eq!(balance_x, 0, "Balance of token x is incorrect");

    let balance_y = simulator
        .call_program(token_y, "balance_of", alice, gas)
        .result::<u64>()
        .unwrap();
    assert!(balance_y > 0, "Balance of token y is incorrect");
}

#[test]
fn swap_insufficient_funds() {
    let mut state = SimpleState::new();
    let mut simulator = Simulator::new(&mut state);
    let alice = Address::new([1; 33]);
    simulator.set_actor(alice);
    let gas = 1000000000;

    let (token_x, token_y, _lt, amm) = init_amm(&mut simulator, gas);
    let amount: u64 = 100;
    let amount_x_swap: u64 = 50;

    simulator
        .call_program(token_x, "mint", (alice, amount), gas)
        .unwrap();
    simulator
        .call_program(token_y, "mint", (alice, amount), gas)
        .unwrap();

    simulator
        .call_program(
            token_x,
            "approve",
            (amm, amount + amount_x_swap + amount_x_swap),
            gas,
        )
        .unwrap();
    simulator
        .call_program(token_y, "approve", (amm, amount), gas)
        .unwrap();

    let result = simulator.call_program(amm, "add_liquidity", (amount, amount), gas);
    assert!(
        !result.has_error(),
        "Add liquidity errored {:?}",
        result.error()
    );

    let balance_y = simulator
        .call_program(token_y, "balance_of", alice, gas)
        .result::<u64>()
        .unwrap();
    assert_eq!(balance_y, 0, "Balance of token y is incorrect");

    let swap = simulator.call_program(amm, "swap", (token_x, amount_x_swap), gas);
    assert!(swap.has_error(), "Swap succeeded with insufficient funds");
}

#[test]
fn swap_incorrect_token() {
    let mut state = SimpleState::new();
    let mut simulator = Simulator::new(&mut state);
    let alice = Address::new([1; 33]);
    simulator.set_actor(alice);
    let gas = 1000000000;

    let (token_x, token_y, _lt, amm) = init_amm(&mut simulator, gas);

    let token_path = PROGRAM_PATH
        .replace("automated_market_maker", "token")
        .replace("automated-market-maker", "token");
    let wrong_token = simulator.create_program(&token_path).program().unwrap();

    let amount: u64 = 100;
    let amount_x_swap: u64 = 50;

    simulator
        .call_program(token_x, "mint", (alice, amount + amount_x_swap), gas)
        .unwrap();
    simulator
        .call_program(token_y, "mint", (alice, amount), gas)
        .unwrap();

    simulator
        .call_program(token_x, "approve", (amm, amount + amount_x_swap), gas)
        .unwrap();
    simulator
        .call_program(token_y, "approve", (amm, amount), gas)
        .unwrap();

    let result = simulator.call_program(amm, "add_liquidity", (amount, amount), gas);
    assert!(
        !result.has_error(),
        "Add liquidity errored {:?}",
        result.error()
    );

    let swap = simulator.call_program(amm, "swap", (wrong_token, amount_x_swap), gas);
    assert!(swap.has_error(), "Swap succeeded with incorrect token");
}

#[test]
fn remove_liquidity() {
    let mut state = SimpleState::new();
    let mut simulator = Simulator::new(&mut state);
    let alice = Address::new([1; 33]);
    simulator.set_actor(alice);
    let gas = 1000000000;

    let (token_x, token_y, lt, amm) = init_amm(&mut simulator, gas);
    let amount: u64 = 100;

    simulator
        .call_program(token_x, "mint", (alice, amount), gas)
        .unwrap();
    simulator
        .call_program(token_y, "mint", (alice, amount), gas)
        .unwrap();

    let balance = simulator
        .call_program(lt, "balance_of", alice, gas)
        .result::<u64>()
        .unwrap();
    assert_eq!(balance, 0, "Balance of liquidity token is incorrect");

    simulator
        .call_program(token_x, "approve", (amm, amount), gas)
        .unwrap();
    simulator
        .call_program(token_y, "approve", (amm, amount), gas)
        .unwrap();

    let result = simulator.call_program(amm, "add_liquidity", (amount, amount), gas);
    assert!(
        !result.has_error(),
        "Add liquidity errored {:?}",
        result.error()
    );

    let balance = simulator
        .call_program(lt, "balance_of", alice, gas)
        .result::<u64>()
        .unwrap();
    assert!(balance > 0, "Balance of liquidity token is incorrect");

    let result = simulator.call_program(amm, "remove_liquidity", balance, gas);
    assert!(
        !result.has_error(),
        "Remove liquidity errored {:?}",
        result.error()
    );

    let (token_x_balance, token_y_balance) = result.result::<(u64, u64)>().unwrap();
    assert_eq!(token_x_balance, amount, "Token x balance is incorrect");
    assert_eq!(token_y_balance, amount, "Token y balance is incorrect");

    let balance = simulator
        .call_program(lt, "balance_of", alice, gas)
        .result::<u64>()
        .unwrap();
    assert_eq!(balance, 0, "Balance of liquidity token is incorrect");

    let balance_x = simulator
        .call_program(token_x, "balance_of", alice, gas)
        .result::<u64>()
        .unwrap();
    assert_eq!(balance_x, amount, "Balance of token x is incorrect");

    let balance_y = simulator
        .call_program(token_y, "balance_of", alice, gas)
        .result::<u64>()
        .unwrap();
    assert_eq!(balance_y, amount, "Balance of token y is incorrect");
}

fn init_amm(simulator: &mut Simulator, gas: u64) -> (Address, Address, Address, Address) {
    let token_path = PROGRAM_PATH
        .replace("automated_market_maker", "token")
        .replace("automated-market-maker", "token");
    let alice = Address::new([1; 33]);

    simulator.set_actor(alice);
    // Setup the tokens
    // TODO: would be a good simulator test if we check token_x and token_y ID to be the same
    let token_x = simulator.create_program(&token_path);
    let token_program_id = token_x.program_id().unwrap();
    let token_x = token_x.program().unwrap();

    let token_y = simulator.create_program(&token_path).program().unwrap();
    // initialize tokens
    simulator
        .call_program(token_x, "init", ("CoinX", "CX"), gas)
        .unwrap();
    simulator
        .call_program(token_y, "init", ("YCoin", "YC"), gas)
        .unwrap();
    let amm_program = simulator.create_program(PROGRAM_PATH).program().unwrap();

    let result = simulator.call_program(
        amm_program,
        "init",
        (token_x, token_y, token_program_id),
        gas,
    );
    assert!(!result.has_error(), "Init AMM errored");

    // Check if the liquidity token was created
    let lt = simulator.call_program(amm_program, "get_liquidity_token", (), gas);
    assert!(!lt.has_error(), "Get liquidity token errored");
    let lt = lt.result::<Address>().unwrap();
    // grab the name of the liquidity token
    let lt_name = simulator.call_program(lt, "symbol", (), gas);
    assert!(!lt_name.has_error(), "Get liquidity token name errored");
    let lt_name = lt_name.result::<String>().unwrap();
    assert_eq!(lt_name, "LT", "Liquidity token name is incorrect");

    (token_x, token_y, lt, amm_program)
}
