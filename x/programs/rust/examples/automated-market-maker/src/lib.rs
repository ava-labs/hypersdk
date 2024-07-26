use std::cmp;
use wasmlanche_sdk::{public, state_keys, ExternalCallContext, Program, Context};

mod math;

#[state_keys]
pub enum StateKeys {
    // Tokens in the Pool as Program type
    TokenX,
    TokenY,
    // Liquidity Token as a Program type
    LiquidityToken,
}

const MAX_GAS: u64 = 10000000;

/// Initializes the pool with the two tokens and the liquidity token
#[public]
pub fn init(
    context: Context<StateKeys>,
    token_x: Program,
    token_y: Program,
    liquidity_token: Program,
) {
    context
        .store([
            (StateKeys::TokenX, &token_x),
            (StateKeys::TokenY, &token_y),
            (StateKeys::LiquidityToken, &liquidity_token),
        ])
        .expect("failed to set state");

    let liquidity_context = ExternalCallContext::new(liquidity_token, MAX_GAS, 0);

    let transfer_reuslt = token::transfer_ownership(&liquidity_context, *context.program().account());
    // TODO: the init function should spin up a new token contract instead
    // of requiring the caller to pass in the liquidity token
    assert!(
        transfer_reuslt,
        "failed to transfer ownership of the liquidity token"
    );
}

/// Swaps 'amount' of `token_program_in` with the other token in the pool
/// Returns the amount of tokens received from the swap
/// Requires `amount` of `token_program_in` to be approved by the actor beforehand
#[public]
pub fn swap(context: Context<StateKeys>, token_program_in: Program, amount: u64) -> u64 {
    let program = context.program();

    // ensure the token_program_in is one of the tokens
    internal::check_token(&context, &token_program_in);

    let (token_in, token_out) = external_token_contracts(&context);

    // make sure token_in matches the token_program_in
    let (token_in, token_out) = if token_program_in.account() == token_in.program().account() {
        (token_in, token_out)
    } else {
        (token_out, token_in)
    };

    // calculate the amount of tokens in the pool
    let (reserve_token_in, reserve_token_out) = reserves(&token_in, &token_out);
    assert!(reserve_token_out > 0, "insufficient liquidity");

    // x * y = k
    // (x + dx) * (y - dy) = k
    // dy = (y * dx) / (x + dx)
    let amount_out = (reserve_token_out * amount) / (reserve_token_in + amount);

    // transfer tokens fropm actor to the pool
    // this will fail if the actor has not approved the tokens or if the actor does not have enough tokens
    token::transfer_from(&token_in, context.actor(), *program.account(), amount);

    // transfer the amount_out to the actor
    // we use transfer_from to update the allowance automatically
    token::transfer_from(&token_out, *program.account(), context.actor(), amount_out);

    // update the allowance for token_in
    let token_approved = token::approve(
        &token_in,
        *program.account(),
        reserve_token_in + amount
    );
    assert!(
        token_approved,
        "failed to update the allowance for the token"
    );

    amount_out
}

/// Adds 'amount_x' of token_x and 'amount_y' of token_y to the pool.
/// The ratio of the tokens must be the same as the ratio of the tokens
/// in the pool, otherwise the function will fail.
/// Both tokens must be approved by the actor before calling this function
/// Returns the amount of LP shares minted
#[public]
pub fn add_liquidity(context: Context<StateKeys>, amount_x: u64, amount_y: u64) -> u64 {
    let program = context.program();
    let (token_x, token_y) = external_token_contracts(&context);
    let lp_token = external_liquidity_token(&context);

    // calculate the amount of tokens in the pool
    let (reserve_x, reserve_y) = reserves(&token_x, &token_y);

    // ensure the proper ratio
    assert_eq!(
        reserve_x * amount_y,
        reserve_y * amount_x,
        "invalid token amounts provided"
    );


    // transfer tokens from the actor to the pool
    token::transfer_from(&token_x, context.actor(), *program.account(), amount_x);
    token::transfer_from(&token_y, context.actor(), *program.account(), amount_y);

    // calculate the amount of shares to mint
    let total_shares = token::total_supply(&lp_token);

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

    assert!(shares > 0, "number of shares minted must be greater than 0");

    // mint the shares
    token::mint(&lp_token, context.actor(), shares);

    // update the amm's allowances
    let approved =  token::approve(&token_x, *program.account(), reserve_x + amount_x)
    && token::approve(&token_y, *program.account(), reserve_y + amount_y);
    assert!(
       approved,
         "failed to update the allowances for the tokens"
    );

    shares
}

/// Removes 'shares' of LP shares from the pool and returns the amount of token_x and token_y received.
/// The actor must have enough LP shares before calling this function.
#[public]
pub fn remove_liquidity(context: Context<StateKeys>, shares: u64) -> (u64, u64) {
    let program = context.program();
    let lp_token = external_liquidity_token(&context);
    // assert that the actor has enough shares
    let actor_total_shares = token::balance_of(&lp_token, context.actor());
    assert!(actor_total_shares >= shares, "insufficient shares");

    let total_shares = token::total_supply(&lp_token);
    let (token_x, token_y) = external_token_contracts(&context);
    let (reserve_x, reserve_y) = reserves(&token_x, &token_y);

    let amount_x = (shares * reserve_x) / total_shares;
    let amount_y = (shares * reserve_y) / total_shares;

    assert!(
        amount_x > 0 && amount_y > 0,
        "amounts must be greater than 0"
    );

    // burn the shares
    token::burn(&lp_token, context.actor(), shares);

    // update the reserves
    token::transfer_from(&token_x, *program.account(), context.actor(), amount_x);
    token::transfer_from(&token_y, *program.account(), context.actor(), amount_y);

    (amount_x, amount_y)
}

/// Removes all LP shares from the pool and returns the amount of token_x and token_y received.
#[public]
pub fn remove_all_liquidity(context: Context<StateKeys>) -> (u64, u64) {
    let lp_token = external_liquidity_token(&context);
    let lp_balance = token::balance_of(&lp_token, context.actor());
    remove_liquidity(context, lp_balance)
}

/// Returns the token reserves in the pool
fn reserves(token_x: &ExternalCallContext, token_y: &ExternalCallContext) -> (u64, u64) {
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

/// Returns the tokens in the pool
fn token_programs(context: &Context<StateKeys>) -> (Program, Program) {
    (
        context
            .get(StateKeys::TokenX)
            .unwrap()
            .expect("token x not initialized"),
        context
            .get(StateKeys::TokenY)
            .unwrap()
            .expect("token y not initialized"),
    )
}
/// Returns the external call contexts for the tokens in the pool
fn external_token_contracts(
    context: &Context<StateKeys>,
) -> (ExternalCallContext, ExternalCallContext) {
    let (token_x, token_y) = token_programs(context);

    (
        ExternalCallContext::new(token_x, MAX_GAS, 0),
        ExternalCallContext::new(token_y, MAX_GAS, 0),
    )
}

/// Returns the external call context for the liquidity token
fn external_liquidity_token(context: &Context<StateKeys>) -> ExternalCallContext {
    let liquidity_token = context.get(StateKeys::LiquidityToken).unwrap().expect("liquidity token not initialized");
    ExternalCallContext::new(liquidity_token, MAX_GAS, 0)
}

mod internal {
    use super::*;

    /// Checks if `token_program` is one of the tokens supported by the pool
    pub fn check_token(context: &Context<StateKeys>, token_program: &Program) {
        let (token_x, token_y) = token_programs(context);
        let supported = token_program.account() == token_x.account()
        || token_program.account() == token_y.account();
        assert!(
            supported,
            "token program is not one of the tokens supported by this pool"
        );
    }
}
