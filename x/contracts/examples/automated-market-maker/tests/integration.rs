// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use std::{path::PathBuf, process::Command};
use token::Units;
use wasmlanche::{
    simulator::{Error, SimpleState, Simulator},
    Address,
};
const CONTRACT_PATH: &str = env!("CONTRACT_PATH");
const MAX_GAS: u64 = 1000000000;

#[test]
fn init_contract() -> Result<(), Error> {
    let mut state = SimpleState::new();
    let simulator = Simulator::new(&mut state);

    simulator.create_contract(CONTRACT_PATH)?;

    Ok(())
}

#[test]
fn amm_init() {
    let mut state = SimpleState::new();
    let mut simulator = Simulator::new(&mut state);
    let alice = Address::new([1; 33]);
    simulator.set_actor(alice);

    init_amm(&mut simulator);
}

#[test]
fn add_liquidity_same_ratio() {
    let mut state = SimpleState::new();
    let mut simulator = Simulator::new(&mut state);
    let alice = Address::new([1; 33]);
    simulator.set_actor(alice);

    let (token_x, token_y, lt, amm) = init_amm(&mut simulator);
    let amount: u64 = 100;

    simulator
        .call_contract::<(), _>(token_x, "mint", (alice, amount), MAX_GAS)
        .unwrap();
    simulator
        .call_contract::<(), _>(token_y, "mint", (alice, amount), MAX_GAS)
        .unwrap();

    let balance: u64 = simulator
        .call_contract(lt, "balance_of", alice, MAX_GAS)
        .unwrap();
    assert_eq!(balance, 0, "Balance of liquidity token is incorrect");

    simulator
        .call_contract::<(), _>(token_x, "approve", (amm, amount), MAX_GAS)
        .unwrap();
    simulator
        .call_contract::<(), _>(token_y, "approve", (amm, amount), MAX_GAS)
        .unwrap();

    let result =
        simulator.call_contract::<Units, _>(amm, "add_liquidity", (amount, amount), MAX_GAS);
    assert!(
        result.is_ok(),
        "Add liquidity errored {:?}",
        result.unwrap_err()
    );

    let balance: u64 = simulator
        .call_contract(lt, "balance_of", alice, MAX_GAS)
        .unwrap();
    assert!(balance > 0, "Balance of liquidity token is incorrect");
}

#[test]
fn add_liquidity_without_approval() {
    let mut state = SimpleState::new();
    let mut simulator = Simulator::new(&mut state);
    let alice = Address::new([1; 33]);
    simulator.set_actor(alice);

    let (token_x, token_y, lt, amm) = init_amm(&mut simulator);
    let amount: u64 = 100;

    simulator
        .call_contract::<(), _>(token_x, "mint", (alice, amount), MAX_GAS)
        .unwrap();
    simulator
        .call_contract::<(), _>(token_y, "mint", (alice, amount), MAX_GAS)
        .unwrap();

    let balance: u64 = simulator
        .call_contract(lt, "balance_of", alice, MAX_GAS)
        .unwrap();
    assert_eq!(balance, 0, "Balance of liquidity token is incorrect");

    let result =
        simulator.call_contract::<Units, _>(amm, "add_liquidity", (amount, amount), MAX_GAS);
    assert!(result.is_err(), "Add liquidity did not error");
}

#[test]
fn swap_changes_ratio() {
    let mut state = SimpleState::new();
    let mut simulator = Simulator::new(&mut state);
    let alice = Address::new([1; 33]);
    simulator.set_actor(alice);

    let (token_x, token_y, _lt, amm) = init_amm(&mut simulator);
    let amount: u64 = 100;
    let amount_x_swap: u64 = 50;

    simulator
        .call_contract::<(), _>(token_x, "mint", (alice, amount + amount_x_swap), MAX_GAS)
        .unwrap();
    simulator
        .call_contract::<(), _>(token_y, "mint", (alice, amount), MAX_GAS)
        .unwrap();

    simulator
        .call_contract::<(), _>(token_x, "approve", (amm, amount + amount_x_swap), MAX_GAS)
        .unwrap();
    simulator
        .call_contract::<(), _>(token_y, "approve", (amm, amount), MAX_GAS)
        .unwrap();

    let result =
        simulator.call_contract::<Units, _>(amm, "add_liquidity", (amount, amount), MAX_GAS);
    assert!(
        result.is_ok(),
        "Add liquidity errored {:?}",
        result.unwrap_err()
    );

    let balance_y: u64 = simulator
        .call_contract(token_y, "balance_of", alice, MAX_GAS)
        .unwrap();
    assert_eq!(balance_y, 0, "Balance of token y is incorrect");

    let swap: u64 = simulator
        .call_contract(amm, "swap", (token_x, amount_x_swap), MAX_GAS)
        .unwrap();

    assert!(swap > 0, "Swap did not return any tokens");

    let balance_x: Units = simulator
        .call_contract(token_x, "balance_of", alice, MAX_GAS)
        .unwrap();
    assert_eq!(balance_x, 0, "Balance of token x is incorrect");

    let balance_y: u64 = simulator
        .call_contract(token_y, "balance_of", alice, MAX_GAS)
        .unwrap();
    assert!(balance_y > 0, "Balance of token y is incorrect");
}

#[test]
fn swap_insufficient_funds() {
    let mut state = SimpleState::new();
    let mut simulator = Simulator::new(&mut state);
    let alice = Address::new([1; 33]);
    simulator.set_actor(alice);

    let (token_x, token_y, _lt, amm) = init_amm(&mut simulator);
    let amount: u64 = 100;
    let amount_x_swap: u64 = 50;

    simulator
        .call_contract::<(), _>(token_x, "mint", (alice, amount), MAX_GAS)
        .unwrap();
    simulator
        .call_contract::<(), _>(token_y, "mint", (alice, amount), MAX_GAS)
        .unwrap();

    simulator
        .call_contract::<(), _>(
            token_x,
            "approve",
            (amm, amount + amount_x_swap + amount_x_swap),
            MAX_GAS,
        )
        .unwrap();
    simulator
        .call_contract::<(), _>(token_y, "approve", (amm, amount), MAX_GAS)
        .unwrap();

    let _result: Units = simulator
        .call_contract(amm, "add_liquidity", (amount, amount), MAX_GAS)
        .unwrap();

    let balance_y: Units = simulator
        .call_contract(token_y, "balance_of", alice, MAX_GAS)
        .unwrap();
    assert_eq!(balance_y, 0, "Balance of token y is incorrect");

    let swap = simulator.call_contract::<Units, _>(amm, "swap", (token_x, amount_x_swap), MAX_GAS);
    assert!(swap.is_err(), "Swap succeeded with insufficient funds");
}

#[test]
fn swap_incorrect_token() {
    let mut state = SimpleState::new();
    let mut simulator = Simulator::new(&mut state);
    let alice = Address::new([1; 33]);
    simulator.set_actor(alice);

    let (token_x, token_y, _lt, amm) = init_amm(&mut simulator);

    let token_path = CONTRACT_PATH
        .replace("automated_market_maker", "token")
        .replace("automated-market-maker", "token");
    let wrong_token = simulator.create_contract(&token_path).unwrap().address;

    let amount: u64 = 100;
    let amount_x_swap: u64 = 50;

    simulator
        .call_contract::<(), _>(token_x, "mint", (alice, amount + amount_x_swap), MAX_GAS)
        .unwrap();
    simulator
        .call_contract::<(), _>(token_y, "mint", (alice, amount), MAX_GAS)
        .unwrap();

    simulator
        .call_contract::<(), _>(token_x, "approve", (amm, amount + amount_x_swap), MAX_GAS)
        .unwrap();
    simulator
        .call_contract::<(), _>(token_y, "approve", (amm, amount), MAX_GAS)
        .unwrap();

    let result =
        simulator.call_contract::<Units, _>(amm, "add_liquidity", (amount, amount), MAX_GAS);
    assert!(
        result.is_ok(),
        "Add liquidity errored {:?}",
        result.unwrap_err()
    );

    let swap =
        simulator.call_contract::<Units, _>(amm, "swap", (wrong_token, amount_x_swap), MAX_GAS);
    assert!(swap.is_err(), "Swap succeeded with incorrect token");
}

#[test]
fn remove_liquidity() {
    let mut state = SimpleState::new();
    let mut simulator = Simulator::new(&mut state);
    let alice = Address::new([1; 33]);
    simulator.set_actor(alice);

    let (token_x, token_y, lt, amm) = init_amm(&mut simulator);
    let amount: u64 = 100;

    simulator
        .call_contract::<(), _>(token_x, "mint", (alice, amount), MAX_GAS)
        .unwrap();
    simulator
        .call_contract::<(), _>(token_y, "mint", (alice, amount), MAX_GAS)
        .unwrap();

    let balance: u64 = simulator
        .call_contract(lt, "balance_of", alice, MAX_GAS)
        .unwrap();
    assert_eq!(balance, 0, "Balance of liquidity token is incorrect");

    simulator
        .call_contract::<(), _>(token_x, "approve", (amm, amount), MAX_GAS)
        .unwrap();
    simulator
        .call_contract::<(), _>(token_y, "approve", (amm, amount), MAX_GAS)
        .unwrap();

    let result =
        simulator.call_contract::<Units, _>(amm, "add_liquidity", (amount, amount), MAX_GAS);
    assert!(
        result.is_ok(),
        "Add liquidity errored {:?}",
        result.unwrap_err()
    );

    let balance: Units = simulator
        .call_contract(lt, "balance_of", alice, MAX_GAS)
        .unwrap();
    assert!(balance > 0, "Balance of liquidity token is incorrect");

    let (token_x_balance, token_y_balance): (u64, u64) = simulator
        .call_contract(amm, "remove_liquidity", balance, MAX_GAS)
        .unwrap();

    assert_eq!(token_x_balance, amount, "Token x balance is incorrect");
    assert_eq!(token_y_balance, amount, "Token y balance is incorrect");

    let balance: Units = simulator
        .call_contract(lt, "balance_of", alice, MAX_GAS)
        .unwrap();
    assert_eq!(balance, 0, "Balance of liquidity token is incorrect");

    let balance_x: Units = simulator
        .call_contract(token_x, "balance_of", alice, MAX_GAS)
        .unwrap();
    assert_eq!(balance_x, amount, "Balance of token x is incorrect");

    let balance_y: Units = simulator
        .call_contract(token_y, "balance_of", alice, MAX_GAS)
        .unwrap();
    assert_eq!(balance_y, amount, "Balance of token y is incorrect");
}

fn init_amm(simulator: &mut Simulator) -> (Address, Address, Address, Address) {
    let token_path = token_path();
    let alice = Address::new([1; 33]);

    simulator.set_actor(alice);
    // Setup the tokens
    // TODO: would be a good simulator test if we check token_x and token_y ID to be the same
    let token_x = simulator.create_contract(token_path).unwrap();
    let token_contract_id = token_x.id;
    let token_x = token_x.address;

    let token_y = simulator.create_contract(token_path).unwrap().address;

    // initialize tokens
    simulator
        .call_contract::<(), _>(token_x, "init", ("CoinX", "CX"), MAX_GAS)
        .unwrap();
    simulator
        .call_contract::<(), _>(token_y, "init", ("YCoin", "YC"), MAX_GAS)
        .unwrap();
    let amm_contract = simulator.create_contract(CONTRACT_PATH).unwrap().address;

    simulator
        .call_contract::<(), _>(
            amm_contract,
            "init",
            (token_x, token_y, token_contract_id),
            MAX_GAS,
        )
        .unwrap();

    // Check if the liquidity token was created
    let lt: Address = simulator
        .call_contract(amm_contract, "get_liquidity_token", (), MAX_GAS)
        .unwrap();
    // grab the name of the liquidity token
    let lt_name: String = simulator.call_contract(lt, "symbol", (), MAX_GAS).unwrap();

    assert_eq!(lt_name, "LT", "Liquidity token name is incorrect");

    (token_x, token_y, lt, amm_contract)
}

fn token_path() -> &'static str {
    let token_path = CONTRACT_PATH
        .replace("automated_market_maker", "token")
        .replace("automated-market-maker", "token");

    let token_path = PathBuf::from(token_path);

    let token_path = match token_path.canonicalize() {
        Ok(path) => path.to_string_lossy().into_owned(),
        Err(_) => {
            let build_dir: PathBuf = PathBuf::from(CONTRACT_PATH)
                .iter()
                .take_while({
                    let mut before_build = true;
                    move |p| {
                        let result = before_build;

                        if *p == "build" {
                            before_build = false;
                        }

                        result
                    }
                })
                .map(PathBuf::from)
                .collect();

            let _ = Command::new("cargo")
                .arg("build")
                .arg("-p")
                .arg("token")
                .arg("--target-dir")
                .arg(build_dir)
                .arg("--target=wasm32-unknown-unknown")
                .output()
                .expect("Failed to build token contract")
                .status
                .success();

            token_path.to_string_lossy().into_owned()
        }
    };

    Box::leak(Box::new(token_path))
}
