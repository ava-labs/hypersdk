// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use std::cmp;
use token::Units;
use wasmlanche::{
    public, state_schema, Address, Context, ContractId, ExternalCallArgs, ExternalCallContext, Gas,
};

mod math;

state_schema! {
    // Tokens in the Pool
    TokenX => Address,
    TokenY => Address,
    // Liquidity Token
    LiquidityToken => Address,
}

const MAX_GAS: Gas = 10_000_000;
const ZERO: u64 = 0;

fn call_args_from_address(address: Address) -> ExternalCallArgs {
    ExternalCallArgs {
        contract_address: address,
        max_units: MAX_GAS,
        value: ZERO,
    }
}

/// Initializes the pool with the two tokens and the liquidity token
#[public]
pub fn init(
    context: &mut Context,
    token_x: Address,
    token_y: Address,
    liquidity_token: ContractId,
) {
    let lt_contract = context.deploy(liquidity_token, &[0, 1]);

    let args = call_args_from_address(lt_contract);
    let liquidity_context = context.to_extern(args);

    token::init(
        liquidity_context,
        String::from("liquidity token"),
        String::from("LT"),
    );

    context
        .store((
            (TokenX, token_x),
            (TokenY, token_y),
            (LiquidityToken, lt_contract),
        ))
        .expect("failed to set state");
}

/// Swaps 'amount' of `token_contract_in` with the other token in the pool
/// Returns the amount of tokens received from the swap
/// Requires `amount` of `token_contract_in` to be approved by the actor beforehand
#[public]
pub fn swap(context: &mut Context, token_contract_in: Address, amount: Units) -> Units {
    // ensure the token_contract_in is one of the tokens
    internal::check_token(context, token_contract_in);

    let (token_x, token_y) = external_token_contracts(context);

    // make sure token_in matches the token_contract_in
    let (token_in, token_out) = if token_contract_in == token_x.contract_address {
        (token_x, token_y)
    } else {
        (token_y, token_x)
    };

    // calculate the amount of tokens in the pool
    let (reserve_token_in, reserve_token_out) = reserves(context, token_in, token_out);
    assert!(reserve_token_out > 0, "insufficient liquidity");

    // x * y = k
    // (x + dx) * (y - dy) = k
    // dy = (y * dx) / (x + dx)
    let amount_out = (reserve_token_out * amount) / (reserve_token_in + amount);
    let actor = context.actor();
    let account = context.contract_address();

    // transfer tokens fropm actor to the pool
    // this will fail if the actor has not approved the tokens or if the actor does not have enough tokens
    token::transfer_from(context.to_extern(token_in), actor, account, amount);

    // transfer the amount_out to the actor
    // we use transfer_from to update the allowance automatically
    token::transfer_from(context.to_extern(token_out), account, actor, amount_out);

    // update the allowance for token_in
    token::approve(
        context.to_extern(token_in),
        account,
        reserve_token_in + amount,
    );

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

    // calculate the amount of tokens in the pool
    let (reserve_x, reserve_y) = reserves(context, token_x, token_y);

    // ensure the proper ratio
    assert_eq!(
        reserve_x * amount_y,
        reserve_y * amount_x,
        "invalid token amounts provided"
    );

    let actor = context.actor();
    let account = context.contract_address();

    // transfer tokens from the actor to the pool
    token::transfer_from(context.to_extern(token_x), actor, account, amount_x);
    token::transfer_from(context.to_extern(token_y), actor, account, amount_y);

    // calculate the amount of shares to mint
    let total_shares = token::total_supply(lp_token(context));

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
    token::mint(lp_token(context), actor, shares);

    // update the amm's allowances
    token::approve(context.to_extern(token_x), account, reserve_x + amount_x);
    token::approve(context.to_extern(token_y), account, reserve_y + amount_y);

    shares
}

/// Removes 'shares' of LP shares from the pool and returns the amount of token_x and token_y received.
/// The actor must have enough LP shares before calling this function.
#[public]
pub fn remove_liquidity(context: &mut Context, shares: Units) -> (Units, Units) {
    let actor = context.actor();
    // assert that the actor has enough shares
    let actor_total_shares = token::balance_of(lp_token(context), actor);
    assert!(actor_total_shares >= shares, "insufficient shares");

    let total_shares = token::total_supply(lp_token(context));
    let (token_x, token_y) = external_token_contracts(context);
    let (reserve_x, reserve_y) = reserves(context, token_x, token_y);

    let amount_x = (shares * reserve_x) / total_shares;
    let amount_y = (shares * reserve_y) / total_shares;

    assert!(
        amount_x > 0 && amount_y > 0,
        "amounts must be greater than 0"
    );

    let actor = context.actor();
    // burn the shares
    token::burn(lp_token(context), actor, shares);

    let actor = context.actor();
    let account = context.contract_address();

    // update the reserves
    token::transfer_from(context.to_extern(token_x), account, actor, amount_x);
    token::transfer_from(context.to_extern(token_y), account, actor, amount_y);

    (amount_x, amount_y)
}

/// Removes all LP shares from the pool and returns the amount of token_x and token_y received.
#[public]
pub fn remove_all_liquidity(context: &mut Context) -> (Units, Units) {
    let actor = context.actor();
    let lp_balance = token::balance_of(lp_token(context), actor);
    remove_liquidity(context, lp_balance)
}

#[public]
pub fn get_liquidity_token(context: &mut Context) -> Address {
    context.get(LiquidityToken).unwrap().unwrap()
}

/// Returns the token reserves in the pool
fn reserves(
    context: &mut Context,
    token_x: ExternalCallArgs,
    token_y: ExternalCallArgs,
) -> (Units, Units) {
    let contract_address = context.contract_address();
    let balance_x = token::allowance(
        context.to_extern(token_x),
        contract_address,
        contract_address,
    );
    let balance_y = token::allowance(
        context.to_extern(token_y),
        contract_address,
        contract_address,
    );

    (balance_x, balance_y)
}

/// Returns the tokens in the pool
fn token_contracts(context: &mut Context) -> (Address, Address) {
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
fn external_token_contracts(context: &mut Context) -> (ExternalCallArgs, ExternalCallArgs) {
    let (token_x, token_y) = token_contracts(context);
    (
        call_args_from_address(token_x),
        call_args_from_address(token_y),
    )
}

/// Returns the external call context for the liquidity token
fn lp_token(context: &mut Context) -> ExternalCallContext {
    let token = context.get(LiquidityToken).unwrap().unwrap();
    let args = call_args_from_address(token);
    context.to_extern(args)
}

mod internal {
    use super::*;

    /// Checks if `token_contract` is one of the tokens supported by the pool
    pub fn check_token(context: &mut Context, token_contract: Address) {
        let (token_x, token_y) = token_contracts(context);
        let supported = token_contract == token_x || token_contract == token_y;
        assert!(
            supported,
            "token contract is not one of the tokens supported by this pool"
        );
    }
}

#[cfg(test)]
mod tests {

    #[ignore]
    #[test]
    fn init() {
        // TODO
    }

    #[ignore]
    #[test]
    fn add_liquidity() {
        // TODO
    }

    #[ignore]
    #[test]
    fn remove_liquidity() {
        // TODO
    }

    #[ignore]
    #[test]
    fn remove_liquidity_insufficient_shares() {
        // TODO
    }

    #[ignore]
    #[test]
    fn remove_all_liquidity() {
        // TODO
    }

    #[ignore]
    #[test]
    fn swap() {
        // TODO
    }

    #[ignore]
    #[test]
    fn swap_no_liquidity() {
        // TODO
    }

    #[ignore]
    #[test]
    fn swap_invalid_token() {
        // TODO
    }

    #[ignore]
    #[test]
    fn swap_not_approved() {
        // TODO
    }

    #[ignore]
    #[test]
    fn swap_insufficient_balance() {
        // TODO
    }
}
