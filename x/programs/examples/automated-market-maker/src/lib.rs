// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use std::cmp;
use token::Units;
use wasmlanche::{public, state_schema, Address, Context, Gas, ProgramId};

mod math;

state_schema! {
    // Tokens in the Pool
    TokenX => Address,
    TokenY => Address,
    // Liquidity Token
    LiquidityToken => Address,
}

const MAX_GAS: Gas = 10000000;

/// Initializes the pool with the two tokens and the liquidity token
#[public]
pub fn init(context: &mut Context, token_x: Address, token_y: Address, liquidity_token: ProgramId) {
    let lt_program = context.deploy(liquidity_token, &[0, 1]);
    let liquidity_context = context.to_extern(lt_program, MAX_GAS, 0);
    token::init(
        &liquidity_context,
        String::from("liquidity token"),
        String::from("LT"),
    );

    context
        .store((
            (TokenX, token_x),
            (TokenY, token_y),
            (LiquidityToken, lt_program),
        ))
        .expect("failed to set state");
}

/// Swaps 'amount' of `token_program_in` with the other token in the pool
/// Returns the amount of tokens received from the swap
/// Requires `amount` of `token_program_in` to be approved by the actor beforehand
#[public]
pub fn swap(context: &mut Context, token_program_in: Address, amount: Units) -> Units {
    // ensure the token_program_in is one of the tokens
    internal::check_token(context, token_program_in);

    let (token_x, token_y) = external_token_contracts(context);

    // make sure token_in matches the token_program_in
    let (token_in, token_out) = if token_program_in == token_x.contract_address() {
        (token_x, token_y)
    } else {
        (token_y, token_x)
    };

    // calculate the amount of tokens in the pool
    let (reserve_token_in, reserve_token_out) = reserves(context, &token_in, &token_out);
    assert!(reserve_token_out > 0, "insufficient liquidity");

    // x * y = k
    // (x + dx) * (y - dy) = k
    // dy = (y * dx) / (x + dx)
    let amount_out = (reserve_token_out * amount) / (reserve_token_in + amount);
    let actor = context.actor();
    let account = context.contract_address();

    // transfer tokens fropm actor to the pool
    // this will fail if the actor has not approved the tokens or if the actor does not have enough tokens
    token::transfer_from(&token_in, actor, account, amount);

    // transfer the amount_out to the actor
    // we use transfer_from to update the allowance automatically
    token::transfer_from(&token_out, account, actor, amount_out);

    // update the allowance for token_in
    token::approve(&token_in, account, reserve_token_in + amount);

    amount_out
}

/// Adds 'amount_x' of token_x and 'amount_y' of token_y to the pool.
/// The ratio of the tokens must be the same as the ratio of the tokens
/// in the pool, otherwise the function will fail.
/// Both tokens must be approved by the actor before calling this function
/// Returns the amount of LP shares minted
#[public]
pub fn add_liquidity(context: &mut Context, amount_x: Units, amount_y: Units) -> Units {
    let (token_x, token_y) = external_token_contracts(context);
    let lp_token = external_liquidity_token(context);

    // calculate the amount of tokens in the pool
    let (reserve_x, reserve_y) = reserves(context, &token_x, &token_y);

    // ensure the proper ratio
    assert_eq!(
        reserve_x * amount_y,
        reserve_y * amount_x,
        "invalid token amounts provided"
    );

    let actor = context.actor();
    let account = context.contract_address();

    // transfer tokens from the actor to the pool
    token::transfer_from(&token_x, actor, account, amount_x);
    token::transfer_from(&token_y, actor, account, amount_y);

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
    token::mint(&lp_token, actor, shares);

    // update the amm's allowances
    token::approve(&token_x, account, reserve_x + amount_x);
    token::approve(&token_y, account, reserve_y + amount_y);

    shares
}

/// Removes 'shares' of LP shares from the pool and returns the amount of token_x and token_y received.
/// The actor must have enough LP shares before calling this function.
#[public]
pub fn remove_liquidity(context: &mut Context, shares: Units) -> (Units, Units) {
    let lp_token = external_liquidity_token(context);

    // assert that the actor has enough shares
    let actor_total_shares = token::balance_of(&lp_token, context.actor());
    assert!(actor_total_shares >= shares, "insufficient shares");

    let total_shares = token::total_supply(&lp_token);
    let (token_x, token_y) = external_token_contracts(context);
    let (reserve_x, reserve_y) = reserves(context, &token_x, &token_y);

    let amount_x = (shares * reserve_x) / total_shares;
    let amount_y = (shares * reserve_y) / total_shares;

    assert!(
        amount_x > 0 && amount_y > 0,
        "amounts must be greater than 0"
    );

    // burn the shares
    token::burn(&lp_token, context.actor(), shares);

    let actor = context.actor();
    let account = context.contract_address();
    // update the reserves
    token::transfer_from(&token_x, account, actor, amount_x);
    token::transfer_from(&token_y, account, actor, amount_y);

    (amount_x, amount_y)
}

/// Removes all LP shares from the pool and returns the amount of token_x and token_y received.
#[public]
pub fn remove_all_liquidity(context: &mut Context) -> (Units, Units) {
    let lp_token = external_liquidity_token(context);
    let lp_balance = token::balance_of(&lp_token, context.actor());
    remove_liquidity(context, lp_balance)
}

#[public]
pub fn get_liquidity_token(context: &mut Context) -> Address {
    context.get(LiquidityToken).unwrap().unwrap()
}

/// Returns the token reserves in the pool
fn reserves(context: &Context, token_x: &Context, token_y: &Context) -> (Units, Units) {
    let balance_x = token::allowance(
        token_x,
        context.contract_address(),
        context.contract_address(),
    );
    let balance_y = token::allowance(
        token_y,
        context.contract_address(),
        context.contract_address(),
    );

    (balance_x, balance_y)
}

/// Returns the tokens in the pool
fn token_programs(context: &mut Context) -> (Address, Address) {
    (
        context
            .get(TokenX)
            .unwrap()
            .expect("token x not initialized"),
        context
            .get(TokenY)
            .unwrap()
            .expect("token y not initialized"),
    )
}
/// Returns the external call contexts for the tokens in the pool
fn external_token_contracts(context: &mut Context) -> (Context, Context) {
    let (token_x, token_y) = token_programs(context);
    (
        context.to_extern(token_x, MAX_GAS, 0),
        context.to_extern(token_y, MAX_GAS, 0),
    )
}

/// Returns the external call context for the liquidity token
fn external_liquidity_token(context: &mut Context) -> Context {
    let token = context.get(LiquidityToken).unwrap().unwrap();
    context.to_extern(token, MAX_GAS, 0)
}

mod internal {
    use super::*;

    /// Checks if `token_program` is one of the tokens supported by the pool
    pub fn check_token(context: &mut Context, token_program: Address) {
        let (token_x, token_y) = token_programs(context);
        let supported = token_program == token_x || token_program == token_y;
        assert!(
            supported,
            "token program is not one of the tokens supported by this pool"
        );
    }
}

#[cfg(test)]
mod tests {
    #[test]
    fn init() {
        // TODO
    }

    #[test]
    fn add_liquidity() {
        // TODO
    }

    #[test]
    fn remove_liquidity() {
        // TODO
    }

    #[test]
    fn remove_liquidity_insufficient_shares() {
        // TODO
    }

    #[test]
    fn remove_all_liquidity() {
        // TODO
    }

    #[test]
    fn swap() {
        // TODO
    }

    #[test]
    fn swap_no_liquidity() {
        // TODO
    }

    #[test]
    fn swap_invalid_token() {
        // TODO
    }

    #[test]
    fn swap_not_approved() {
        // TODO
    }

    #[test]
    fn swap_insufficient_balance() {
        // TODO
    }
}
