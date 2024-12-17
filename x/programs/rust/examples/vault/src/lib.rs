// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use borsh::{BorshDeserialize, BorshSerialize};
use wasmlanche_sdk::{public, state_keys, Address, Program};
#[cfg(not(feature = "bindings"))]
use wasmlanche_sdk::{Context, ExternalCallContext};

#[state_keys]
pub enum StateKeys {
    /// Address of the vault asset
    Asset,
    /// Shares of account
    BalanceOf(Address),
    /// Total amount of shares
    TotalSupply,
    /// Shares allowance
    Allowance(Address, Address),
}

#[derive(Debug, BorshSerialize, BorshDeserialize)]
pub enum VaultError {
    TotalAssetsOverflow,
    TotalAssetsUnderflow,
    TotalSupplyOverflow,
    TotalSupplyUnderflow,
    DivByZero,
}

#[cfg(not(feature = "bindings"))]
enum Rounding {
    Down,
    Up,
}

#[public]
pub fn init(ctx: Context<StateKeys>, asset: Program, lock_amount: u64) {
    if ctx
        .get::<Program>(StateKeys::Asset)
        .expect("failed to get asset")
        .is_some()
    {
        panic!("already initialized");
    }

    ctx.store_by_key(StateKeys::Asset, &asset)
        .expect("failed to store asset");

    // lock tokens for defining the exchange rate (starting at 1:1)
    let ext_ctx = ExternalCallContext::new(asset, 1000000, 0);
    token::transfer_from(&ext_ctx, ctx.actor(), *ctx.program().account(), lock_amount);

    internal::accounting::add(&ctx, lock_amount, ctx.actor()).expect("failed to lock up tokens");
}

/// Deposit assets in the vault and return the amount of shares minted
#[public]
pub fn deposit(ctx: Context<StateKeys>, assets: u64, receiver: Address) -> Result<u64, VaultError> {
    let shares = internal::accounting::convert_to_shares(&ctx, assets, Some(Rounding::Down))?;

    let ext_ctx = ExternalCallContext::new(internal::assets::asset(&ctx), 1000000, 0);
    token::transfer_from(&ext_ctx, ctx.actor(), *ctx.program().account(), assets);

    internal::accounting::add(&ctx, shares, receiver)?;

    Ok(shares)
}

/// Mint shares in the vault and return the assets minted
#[public]
pub fn mint(ctx: Context<StateKeys>, shares: u64, receiver: Address) -> Result<u64, VaultError> {
    let assets = internal::accounting::convert_to_assets(&ctx, shares, Some(Rounding::Down))?;

    let ext_ctx = ExternalCallContext::new(internal::assets::asset(&ctx), 1000000, 0);
    token::transfer_from(&ext_ctx, ctx.actor(), *ctx.program().account(), assets);

    internal::accounting::add(&ctx, shares, receiver)?;

    Ok(shares)
}

/// Burn shares from the vault and sends exactly `assets` to the receiver
#[public]
pub fn withdraw(
    ctx: Context<StateKeys>,
    assets: u64,
    receiver: Address,
    owner: Address,
) -> Result<u64, VaultError> {
    assert_eq!(owner, ctx.actor());
    assert_eq!(receiver, owner);

    let shares = internal::accounting::convert_to_shares(&ctx, assets, Some(Rounding::Up))?;

    internal::accounting::sub(&ctx, shares, owner)?;

    let ext_ctx = ExternalCallContext::new(internal::assets::asset(&ctx), 1000000, 0);
    token::transfer(&ext_ctx, receiver, assets);

    Ok(assets)
}

/// Burn exactly `shares` from the owner and sends assets to the receiver
#[public]
pub fn redeem(
    ctx: Context<StateKeys>,
    shares: u64,
    receiver: Address,
    owner: Address,
) -> Result<u64, VaultError> {
    assert_eq!(owner, ctx.actor());
    assert_eq!(receiver, owner);

    let assets = internal::accounting::convert_to_assets(&ctx, shares, Some(Rounding::Up))?;

    internal::accounting::sub(&ctx, shares, owner)?;

    let ext_ctx = ExternalCallContext::new(internal::assets::asset(&ctx), 1000000, 0);
    token::transfer(&ext_ctx, receiver, assets);

    Ok(assets)
}

#[public]
pub fn balance_of(ctx: Context<StateKeys>, who: Address) -> u64 {
    internal::assets::balance_of(&ctx, who)
}

#[cfg(not(feature = "bindings"))]
mod internal {
    use super::*;

    pub mod assets {
        use super::*;
        use wasmlanche_sdk::Program;

        pub fn total_assets(ctx: &Context<StateKeys>) -> u64 {
            let ext_ctx = ExternalCallContext::new(internal::assets::asset(ctx), 1000000, 0);
            token::balance_of(&ext_ctx, *ctx.program().account())
        }

        pub fn total_supply(ctx: &Context<StateKeys>) -> u64 {
            ctx.get(StateKeys::TotalSupply)
                .expect("failed to get total supply")
                .unwrap_or_default()
        }

        pub fn balance_of(ctx: &Context<StateKeys>, who: Address) -> u64 {
            ctx.get(StateKeys::BalanceOf(who))
                .expect("failed to get balance of")
                .unwrap_or_default()
        }

        pub fn asset(ctx: &Context<StateKeys>) -> Program {
            ctx.get(StateKeys::Asset)
                .expect("failed to get asset")
                .expect("asset was not registered")
        }
    }

    pub mod accounting {
        use super::*;

        pub fn convert_to_shares(
            ctx: &Context<StateKeys>,
            assets: u64,
            rounding: Option<Rounding>,
        ) -> Result<u64, VaultError> {
            let total_supply = self::assets::total_supply(ctx);
            let total_assets = self::assets::total_assets(ctx);

            muldiv(assets, total_supply, total_assets, rounding).map_err(|err| match err {
                MathError::NumOverflow => VaultError::TotalSupplyOverflow,
                MathError::NumUnderflow => VaultError::TotalSupplyUnderflow,
                MathError::DenOverflow => VaultError::TotalAssetsOverflow,
                MathError::DenUnderflow => VaultError::TotalAssetsUnderflow,
                MathError::DivByZero => VaultError::DivByZero,
            })
        }

        pub fn convert_to_assets(
            ctx: &Context<StateKeys>,
            shares: u64,
            rounding: Option<Rounding>,
        ) -> Result<u64, VaultError> {
            let total_supply = self::assets::total_supply(ctx);
            let total_assets = self::assets::total_assets(ctx);

            muldiv(shares, total_assets, total_supply, rounding).map_err(|err| match err {
                MathError::NumOverflow => VaultError::TotalAssetsOverflow,
                MathError::NumUnderflow => VaultError::TotalAssetsUnderflow,
                MathError::DenOverflow => VaultError::TotalSupplyOverflow,
                MathError::DenUnderflow => VaultError::TotalSupplyUnderflow,
                MathError::DivByZero => VaultError::DivByZero,
            })
        }

        enum MathError {
            NumOverflow,
            NumUnderflow,
            DenOverflow,
            DenUnderflow,
            DivByZero,
        }

        /// `a` * `b` / `den`
        fn muldiv(a: u64, b: u64, den: u64, rounding: Option<Rounding>) -> Result<u64, MathError> {
            let num = a as u128 * b as u128;
            let den = den as u128;

            let num = if let Some(rounding) = rounding {
                match rounding {
                    Rounding::Up => num
                        .checked_add(den.checked_sub(1).ok_or(MathError::DenUnderflow)?)
                        .ok_or(MathError::NumOverflow)?,
                    Rounding::Down => num
                        .checked_sub(den.checked_add(1).ok_or(MathError::DenOverflow)?)
                        .ok_or(MathError::NumUnderflow)?,
                }
            } else {
                num
            };

            num.checked_div(den)
                .ok_or(MathError::DivByZero)?
                .try_into()
                .map_err(|_| MathError::NumOverflow)
        }

        pub fn add(
            ctx: &Context<StateKeys>,
            shares: u64,
            receiver: Address,
        ) -> Result<(), VaultError> {
            let total_supply = internal::assets::total_supply(ctx);
            let new_total_supply = total_supply + shares;
            let balance = internal::assets::balance_of(ctx, receiver);
            let new_balance = balance + shares;

            ctx.store([
                (StateKeys::TotalSupply, &new_total_supply),
                (StateKeys::BalanceOf(receiver), &new_balance),
            ])
            .expect("failed to store balances increases");

            Ok(())
        }

        pub fn sub(
            ctx: &Context<StateKeys>,
            shares: u64,
            receiver: Address,
        ) -> Result<(), VaultError> {
            let total_supply = internal::assets::total_supply(ctx);
            let new_total_supply = total_supply - shares;
            let balance = internal::assets::balance_of(ctx, receiver);
            let new_balance = balance - shares;

            ctx.store([
                (StateKeys::TotalSupply, &new_total_supply),
                (StateKeys::BalanceOf(receiver), &new_balance),
            ])
            .expect("failed to store balances increases");

            Ok(())
        }
    }
}

#[cfg(test)]
mod tests {
    use super::VaultError;
    use simulator::{ClientBuilder, Endpoint, Step, TestContext};
    use wasmlanche_sdk::Address;

    const PROGRAM_PATH: &str = env!("PROGRAM_PATH");

    fn token_program_path() -> String {
        let (trimmed_program_path, _) = PROGRAM_PATH.rsplit_once('/').unwrap();
        let (_, profile) = trimmed_program_path.rsplit_once('/').unwrap();

        [
            trimmed_program_path,
            &format!("/../../../../token/build/wasm32-unknown-unknown/{profile}/token.wasm"),
        ]
        .concat()
    }

    #[test]
    fn cannot_initialize_twice() {
        let mut simulator = ClientBuilder::new().try_build().unwrap();

        let program_id = simulator
            .run_step(&Step::create_program(PROGRAM_PATH))
            .unwrap()
            .id;

        let asset = Address::new([0; 33]);

        let init_step = Step {
            endpoint: Endpoint::Execute,
            method: "init".to_string(),
            max_units: 1000000,
            params: vec![
                TestContext::from(program_id).into(),
                asset.into(),
                0u64.into(),
            ],
        };

        simulator.run_step(&init_step).unwrap();

        let res: Result<(), simulator::StepResponseError> =
            simulator.run_step(&init_step).unwrap().result.response();
        assert!(res.is_err());
    }

    #[test]
    fn deposit_assets() {
        let mut simulator = ClientBuilder::new().try_build().unwrap();

        let program_id = simulator
            .run_step(&Step::create_program(PROGRAM_PATH))
            .unwrap()
            .id;

        let token_program_path = token_program_path();

        let asset_id = simulator
            .run_step(&Step::create_program(token_program_path))
            .unwrap()
            .id;

        let token_context = TestContext::from(asset_id);
        simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "init".to_string(),
                max_units: 1000000,
                params: vec![
                    token_context.clone().into(),
                    "token".to_string().into(),
                    "TOK".to_string().into(),
                ],
            })
            .unwrap();

        let token_amount = 100_000;
        simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "mint".to_string(),
                max_units: 1000000,
                params: vec![
                    token_context.clone().into(),
                    Address::default().into(),
                    token_amount.into(),
                ],
            })
            .unwrap();

        let lock_amount = 1000;

        simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "approve".to_string(),
                max_units: 1000000,
                params: vec![
                    token_context.clone().into(),
                    program_id.into(),
                    token_amount.into(),
                ],
            })
            .unwrap();

        let vault_context = TestContext::from(program_id);
        simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "init".to_string(),
                max_units: u64::MAX,
                params: vec![
                    vault_context.clone().into(),
                    asset_id.into(),
                    lock_amount.into(),
                ],
            })
            .unwrap();

        let maybe_shares: Result<u64, VaultError> = simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "deposit".to_string(),
                max_units: u64::MAX,
                params: vec![
                    vault_context.into(),
                    (token_amount - lock_amount).into(),
                    Address::default().into(),
                ],
            })
            .unwrap()
            .result
            .response()
            .unwrap();
        assert!(maybe_shares.is_ok());
    }

    #[test]
    fn withdraw_all_assets() {
        let mut simulator = ClientBuilder::new().try_build().unwrap();

        let vault_id = simulator
            .run_step(&Step::create_program(PROGRAM_PATH))
            .unwrap()
            .id;

        let token_program_path = token_program_path();

        let asset_id = simulator
            .run_step(&Step::create_program(&token_program_path))
            .unwrap()
            .id;

        let deployer = Address::new([1; 33]);
        let depositor = Address::new([2; 33]);

        let mut token_context = TestContext::from(asset_id);
        token_context.actor = deployer;
        simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "init".to_string(),
                max_units: 1000000,
                params: vec![
                    token_context.clone().into(),
                    "token".to_string().into(),
                    "TOK".to_string().into(),
                ],
            })
            .unwrap();

        let token_amount = 100_000;
        simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "mint".to_string(),
                max_units: 1000000,
                params: vec![
                    token_context.clone().into(),
                    deployer.into(),
                    token_amount.into(),
                ],
            })
            .unwrap();

        let lock_amount = 1000;
        let assets = token_amount - lock_amount;

        simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "transfer".to_string(),
                max_units: 1000000,
                params: vec![
                    token_context.clone().into(),
                    depositor.into(),
                    assets.into(),
                ],
            })
            .unwrap();

        simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "approve".to_string(),
                max_units: 1000000,
                params: vec![
                    token_context.clone().into(),
                    vault_id.into(),
                    lock_amount.into(),
                ],
            })
            .unwrap();

        let mut vault_context = TestContext::from(vault_id);
        vault_context.actor = deployer;
        simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "init".to_string(),
                max_units: u64::MAX,
                params: vec![
                    vault_context.clone().into(),
                    asset_id.into(),
                    lock_amount.into(),
                ],
            })
            .unwrap();

        let token_context = TestContext {
            actor: depositor,
            ..token_context.clone()
        };

        simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "approve".to_string(),
                max_units: 1000000,
                params: vec![token_context.clone().into(), vault_id.into(), assets.into()],
            })
            .unwrap();

        let vault_context = TestContext {
            actor: depositor,
            ..vault_context.clone()
        };

        let maybe_shares: Result<u64, VaultError> = simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "deposit".to_string(),
                max_units: u64::MAX,
                params: vec![
                    vault_context.clone().into(),
                    assets.into(),
                    depositor.into(),
                ],
            })
            .unwrap()
            .result
            .response()
            .unwrap();
        assert!(maybe_shares.is_ok());

        let shares = maybe_shares.unwrap();

        let maybe_assets: Result<u64, VaultError> = simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "redeem".to_string(),
                max_units: u64::MAX,
                params: vec![
                    vault_context.clone().into(),
                    shares.into(),
                    depositor.into(),
                    depositor.into(),
                ],
            })
            .unwrap()
            .result
            .response()
            .unwrap();
        assert_eq!(maybe_assets.unwrap(), assets);

        let user_shares: u64 = simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "balance_of".to_string(),
                max_units: u64::MAX,
                params: vec![vault_context.into(), depositor.into()],
            })
            .unwrap()
            .result
            .response()
            .unwrap();

        assert_eq!(user_shares, 0);
    }
}
