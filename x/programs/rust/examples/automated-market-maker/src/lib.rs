use std::cmp;
use wasmlanche_sdk::Context;
use wasmlanche_sdk::{
    public, state_keys, types::Address, ExternalCallContext, Program,
};
mod math;

#[state_keys]
pub enum StateKeys {
    // Tokens in the Pool
    TokenX,
    TokenY,

    // total LP shares
    TotalShares,
    // balances of LP shares
    Balances(Address),
}

const MAX_GAS: u64 = 10000000;

#[public]
// Swaps 'amount' of `token_program_in` with the other token in the pool.
// Returns the amount of tokens received from the swap
// Requires `amount` of `token_program_in` to be approved by the actor beforehand
pub fn swap(context: Context<StateKeys>, token_program_in: Program, amount: u64) -> u64 {
    let program = context.program();

    // ensure the token_program_in is one of the tokens
    _token_check(program, &token_program_in);

    let (token_in, token_out) = external_token_contracts(program, MAX_GAS, 1000000);

    // make sure token_in is the first token
    let (token_in, token_out) = if token_program_in.account() == token_in.program().account() {
        (token_in, token_out)
    } else {
        (token_out, token_in)
    };

    // calculate the amount of tokens in the pool
    let (reserve_token_in, reserve_token_out) = reserves(&token_in, &token_out);
    assert!(
        reserve_token_in > amount && reserve_token_out > 0,
        "insufficient liquidity"
    );

    // x * y = k
    // (x + dx) * (y - dy) = k
    // dy = (y * dx) / (x + dx)
    let amount_out = (reserve_token_out * amount) / (reserve_token_in + amount);

    // transfer tokens fropm actor to the pool
    // this will ensure the actor has enough tokens
    token::transfer_from(token_in, context.actor(), *program.account(), amount);

    // transfer the amount_out to the actor
    token::transfer(token_out, context.actor(), amount_out);

    // update the allowances
    assert!(
        token::approve(token_in, *program.account(), reserve_token_in + amount)
            && token::approve(token_out, *program.account(), reserve_token_out - amount)
    );

    amount_out
}

#[public]
// Adds 'amount_x' of token_x and 'amount_y' of token_y to the pool. 
// The ratio of the tokens must be the same as the ratio of the tokens 
// in the pool, otherwise the function will fail.
// Both tokens must be approved by the actor before calling this function
// Returns the amount of LP shares minted
pub fn add_liquidity(context: Context<StateKeys>, amount_x: u64, amount_y: u64) -> u64 {
    let program = context.program();
    let (token_x, token_y) = external_token_contracts(program, MAX_GAS, 1000000);

    // calculate the amount of tokens in the pool
    let (reserve_x, reserve_y) = reserves(&token_x, &token_y);

    // ensure the proper ratio
    assert_eq!(reserve_x * amount_y, reserve_y * amount_x, "invalid ratio");
    // transfer tokens from the actor to the pool
    token::transfer_from(token_x, context.actor(), *program.account(), amount_x);
    token::transfer_from(token_y, context.actor(), *program.account(), amount_y);

    // calculate the amount of shares to mint
    let total_shares = total_shares(program);
    let shares = if total_shares == 0 {
        // if the pool is empty, mint the shares
        math::sqrt(amount_x * amount_y)
    } else {
        // calculate the amount of shares to mint
        cmp::min(
            (amount_x * total_shares) / reserve_x,
            (amount_y * total_shares) / reserve_y,
        )
    };
    assert!(shares > 0, "insufficient liquidity");

    // mint the shares
    mint_lp(program, shares);
    let lp_balance : u64= program
        .state()
        .get(StateKeys::Balances(context.actor()))
        .unwrap()
        .unwrap_or_default();
    update_lp_balance(program, context.actor(), lp_balance + shares);

    // update the allowances
    assert!(
        token::approve(token_x, *program.account(), reserve_x + amount_x)
            && token::approve(token_y, *program.account(), reserve_y + amount_y)
    );

    shares
}

#[public]
// Removes 'shares' of LP shares from the pool and returns the amount of token_x and token_y received.
// The actor must have enough LP shares before calling this function.
pub fn remove_liquidity(context: Context<StateKeys>, shares: u64) -> (u64, u64) {
    let program = context.program();

    // assert that the actor has enough shares
    let lp_balance : u64 = program
        .state()
        .get(StateKeys::Balances(context.actor()))
        .unwrap()
        .unwrap_or_default();
    assert!(lp_balance >= shares, "insufficient shares");

    let total_shares = total_shares(program);
    let (token_x, token_y) = external_token_contracts(program, MAX_GAS, 1000000);
    let (reserve_x, reserve_y) = reserves(&token_x, &token_y);

    let amount_x = (shares * reserve_x) / total_shares;
    let amount_y = (shares * reserve_y) / total_shares;

    assert!(amount_x > 0 && amount_y > 0, "insufficient liquidity");

    // burn the shares
    burn_lp(program, shares);

    // update the reserves
    token::transfer(token_x, context.actor(), amount_x);
    token::transfer(token_y, context.actor(), amount_y);

    // update the allowances
    assert!(
        token::approve(token_x, *program.account(), reserve_x - amount_x)
            && token::approve(token_y, *program.account(), reserve_y - amount_y)
    );

    (amount_x, amount_y)
}

#[public]
// Removes all LP shares from the pool and returns the amount of token_x and token_y received.
pub fn remove_all_liquidity(context: Context<StateKeys>) -> (u64, u64) {
    let program = context.program();
    let lp_balance = program
        .state()
        .get(StateKeys::Balances(context.actor()))
        .unwrap()
        .unwrap_or_default();
    remove_liquidity(context, lp_balance)
}

// Calculates the balances of the tokens in the pool
fn reserves(
    token_x: &ExternalCallContext,
    token_y: &ExternalCallContext,
) -> (u64, u64) {
    let balance_x = token::allowance(
        token_x,
        *token_x.program().account(),
        *token_x.program().account(),
    );
    let balance_y = token::allowance(
        token_y,
        *token_y.program().account(),
        *token_y.program().account(),
    );

    (balance_x, balance_y)
}

// Returns the total amount of LP shares in circulation
fn total_shares(program: &Program<StateKeys>) -> u64 {
    program
        .state()
        .get(StateKeys::TotalShares)
        .unwrap()
        .unwrap_or_default()
}

// Checks if `token_program` is one of the tokens supported by the pool
fn _token_check(program: &Program<StateKeys>, token_program: &Program) {
    let (token_x, token_y) = tokens(program);

    assert!(
        token_program.account() == token_x.account()
            || token_program.account() == token_y.account(),
        "token program is not one of the tokens supported by this pool"
    );
}

// Returns the tokens in the pool
fn tokens(program: &Program<StateKeys>) -> (Program, Program) {
    (
        program.state().get(StateKeys::TokenX).unwrap().unwrap(),
        program.state().get(StateKeys::TokenY).unwrap().unwrap(),
    )
}
// Returns the external call contexts for the tokens in the pool
fn external_token_contracts(
    program: &Program<StateKeys>,
    max_gas_x: u64,
    max_gas_y: u64,
) -> (ExternalCallContext, ExternalCallContext) {
    let (token_x, token_y) = tokens(program);

    (
        ExternalCallContext::new(token_x, max_gas_x, 0),
        ExternalCallContext::new(token_y, max_gas_y, 0),
    )
}

// Updates the LP balance of the actor to `new_balance`
fn update_lp_balance(program: &Program<StateKeys>, actor: Address, new_balance: u64) {
    program
        .state()
        .store_by_key(StateKeys::Balances(actor), &(new_balance))
        .unwrap();
}

// Increases the amount of LP shares in circulation by 'amount'
fn mint_lp(program: &Program<StateKeys>, amount: u64) {
    let total_shares = total_shares(program);

    program
        .state()
        .store_by_key(StateKeys::TotalShares, &(total_shares + amount))
        .unwrap();
}

// Decreases the amount of LP shares in circulation by 'amount'
fn burn_lp(program: &Program<StateKeys>, amount: u64) {
    let total_shares = total_shares(program);

    assert!(total_shares >= amount, "not enough shares to burn");

    program
        .state()
        .store_by_key(StateKeys::TotalShares, &(total_shares - amount))
        .unwrap();
}

#[cfg(test)]
mod tests {
    use simulator::{Endpoint, Key, Step, StepResponseError, TestContext};
    use wasmlanche_sdk::ExternalCallError;

    const PROGRAM_PATH: &str = env!("PROGRAM_PATH");

    #[test]
    fn init_state() {
        let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        let owner = String::from("owner");

        let program_id = simulator
            .run_step(&Step::create_program(PROGRAM_PATH))
            .unwrap()
            .id;

        simulator
            .run_step(&Step::create_key(Key::Ed25519(owner)))
            .unwrap();

        let test_context = TestContext::from(program_id);

        let resp_err = simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "remove_liquidity".to_string(),
                max_units: u64::MAX,
                params: vec![test_context.clone().into(), 100000u64.into()],
            })
            .unwrap()
            .result
            .response::<(u64, u64)>()
            .unwrap_err();

        let StepResponseError::ExternalCall(call_err) = resp_err else {
            panic!("wrong error returned");
        };

        assert!(matches!(call_err, ExternalCallError::CallPanicked));

        let resp_err = simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "swap".to_string(),
                max_units: u64::MAX,
                params: vec![test_context.into(), 100000u64.into(), true.into()],
            })
            .unwrap()
            .result
            .response::<u64>()
            .unwrap_err();

        let StepResponseError::ExternalCall(call_err) = resp_err else {
            panic!("wrong error returned");
        };

        assert!(matches!(call_err, ExternalCallError::CallPanicked));
    }

    #[test]
    fn add_liquidity_same_ratio() {
        let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        let owner = String::from("owner");

        let program_id = simulator
            .run_step(&Step::create_program(PROGRAM_PATH))
            .unwrap()
            .id;

        simulator
            .run_step(&Step::create_key(Key::Ed25519(owner)))
            .unwrap();

        let test_context = TestContext::from(program_id);

        let resp = simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "add_liquidity".to_string(),
                max_units: u64::MAX,
                params: vec![test_context.clone().into(), 1000u64.into(), 1000u64.into()],
            })
            .unwrap()
            .result
            .response::<u64>()
            .unwrap();

        assert_eq!(resp, 1000);

        let resp = simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "add_liquidity".to_string(),
                max_units: u64::MAX,
                params: vec![test_context.into(), 1000u64.into(), 1001u64.into()],
            })
            .unwrap()
            .result
            .response::<u64>()
            .unwrap_err();

        let StepResponseError::ExternalCall(call_err) = resp else {
            panic!("unexpected error");
        };

        assert!(matches!(call_err, ExternalCallError::CallPanicked));
    }

    #[test]
    fn swap_changes_ratio() {
        let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        let owner = String::from("owner");

        let program_id = simulator
            .run_step(&Step::create_program(PROGRAM_PATH))
            .unwrap()
            .id;

        simulator
            .run_step(&Step::create_key(Key::Ed25519(owner)))
            .unwrap();

        let test_context = TestContext::from(program_id);

        let resp = simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "add_liquidity".to_string(),
                max_units: u64::MAX,
                params: vec![test_context.clone().into(), 1000u64.into(), 1000u64.into()],
            })
            .unwrap()
            .result
            .response::<u64>()
            .unwrap();

        assert_eq!(resp, 1000);

        simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "swap".to_string(),
                max_units: u64::MAX,
                params: vec![test_context.clone().into(), 10u64.into(), true.into()],
            })
            .unwrap();

        let (amount_x, amount_y) = simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "remove_liquidity".to_string(),
                max_units: u64::MAX,
                params: vec![test_context.into(), 1000.into()],
            })
            .unwrap()
            .result
            .response::<(u64, u64)>()
            .unwrap();

        assert!(amount_x > 1000);
        assert!(amount_y < 1000);
    }
}
