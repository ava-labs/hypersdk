use borsh::{BorshDeserialize, BorshSerialize};
#[cfg(not(feature = "bindings"))]
use wasmlanche_sdk::Context;
use wasmlanche_sdk::{public, state_keys, Address, ExternalCallContext, Program};

#[state_keys]
pub enum StateKeys {
    /// Address of the vault asset
    Asset,
    /// Total amount of assets stored in the vault
    TotalAssets,
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

enum Rounding {
    Down,
    Up,
}

#[public]
pub fn init(ctx: Context<StateKeys>, asset: Program) {
    if ctx
        .get::<Program>(StateKeys::Asset)
        .expect("failed to get asset")
        .is_some()
    {
        panic!("already initialized");
    }

    ctx.store_by_key(StateKeys::Asset, &asset)
        .expect("failed to store asset");

    // lock tokens for defining the exchange rate
    // TODO actually transfer them
    internal::accounting::add(&ctx, 1000, 1000, Address::default())
        .expect("failed to lock up tokens");
}

/// Deposit assets in the vault and return the amount of shares minted
#[public]
pub fn deposit(ctx: Context<StateKeys>, assets: u64, receiver: Address) -> Result<u64, VaultError> {
    let shares = internal::accounting::convert_to_shares(&ctx, assets, Some(Rounding::Down))?;

    let ext_ctx = ExternalCallContext::new(internal::assets::asset(&ctx), 1000000, 0);
    token::transfer_from(&ext_ctx, ctx.actor(), *ctx.program().account(), assets);

    internal::accounting::add(&ctx, shares, assets, receiver)?;

    Ok(shares)
}

/// Mint shares in the vault and return the assets minted
#[public]
pub fn mint(ctx: Context<StateKeys>, shares: u64, receiver: Address) -> Result<u64, VaultError> {
    let assets = internal::accounting::convert_to_assets(&ctx, shares, Some(Rounding::Down))?;

    let ext_ctx = ExternalCallContext::new(internal::assets::asset(&ctx), 1000000, 0);
    token::transfer_from(&ext_ctx, ctx.actor(), *ctx.program().account(), assets);

    internal::accounting::add(&ctx, shares, assets, receiver)?;

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
    let shares = internal::accounting::convert_to_shares(&ctx, assets, Some(Rounding::Up))?;

    internal::accounting::sub(&ctx, shares, assets, owner)?;

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
    let assets = internal::accounting::convert_to_assets(&ctx, shares, Some(Rounding::Up))?;

    internal::accounting::sub(&ctx, shares, assets, owner)?;

    let ext_ctx = ExternalCallContext::new(internal::assets::asset(&ctx), 1000000, 0);
    token::transfer(&ext_ctx, receiver, assets);

    Ok(assets)
}

mod internal {
    use super::*;

    pub mod assets {
        use wasmlanche_sdk::Program;

        use super::*;

        pub fn total_assets(ctx: &Context<StateKeys>) -> u64 {
            ctx.get(StateKeys::TotalAssets)
                .expect("failed to get total assets")
                .unwrap_or_default()
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
            assets: u64,
            receiver: Address,
        ) -> Result<(), VaultError> {
            let total_assets = self::assets::total_assets(ctx);

            // # Safety
            // It is okay to only check the add of assets because the ratio of shares/assets is <= 1
            // Thus, if TotalAssets does not overflow, so does not TotalSupply, and BalanceOf.
            let new_total_assets = total_assets
                .checked_add(assets)
                .ok_or(VaultError::TotalAssetsOverflow)?;
            let total_supply = internal::assets::total_supply(ctx);
            let new_total_supply = total_supply + shares;
            let balance = internal::assets::balance_of(ctx, receiver);
            let new_balance = balance + shares;

            ctx.store([
                (StateKeys::TotalAssets, &new_total_assets),
                (StateKeys::TotalSupply, &new_total_supply),
                (StateKeys::BalanceOf(receiver), &new_balance),
            ])
            .expect("failed to store balances increases");

            Ok(())
        }

        pub fn sub(
            ctx: &Context<StateKeys>,
            shares: u64,
            assets: u64,
            receiver: Address,
        ) -> Result<(), VaultError> {
            let total_assets = self::assets::total_assets(ctx);

            // # Safety
            // It is okay to only check the sub of assets because the ratio of shares/assets is <= 1
            // Thus, if TotalAssets does not underflow, so does not TotalSupply, and BalanceOf.
            let new_total_assets = total_assets
                .checked_sub(assets)
                .ok_or(VaultError::TotalAssetsOverflow)?;
            let total_supply = internal::assets::total_supply(ctx);
            let new_total_supply = total_supply - shares;
            let balance = internal::assets::balance_of(ctx, receiver);
            let new_balance = balance - shares;

            ctx.store([
                (StateKeys::TotalAssets, &new_total_assets),
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
            params: vec![TestContext::from(program_id).into(), asset.into()],
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

        let (trimmed_program_path, _) = PROGRAM_PATH.rsplit_once('/').unwrap();
        let (_, profile) = trimmed_program_path.rsplit_once('/').unwrap();

        let token_program_path = [
            trimmed_program_path,
            &format!("/../../../../token/build/wasm32-unknown-unknown/{profile}/token.wasm"),
        ]
        .concat();

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

        let vault_context = TestContext::from(program_id);
        simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "init".to_string(),
                max_units: 1000000,
                params: vec![vault_context.clone().into(), asset_id.into()],
            })
            .unwrap();

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

        let maybe_shares: Result<u64, VaultError> = simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "deposit".to_string(),
                max_units: 10000000,
                params: vec![
                    vault_context.into(),
                    token_amount.into(),
                    Address::default().into(),
                ],
            })
            .unwrap()
            .result
            .response()
            .unwrap();
        assert!(dbg!(maybe_shares).is_ok());
    }
}
