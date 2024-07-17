use wasmlanche_sdk::{public, state_keys, Context};

#[state_keys]
pub enum StateKeys {
    // Internal accounting
    ReserveX,
    ReserveY,

    // Liquidity token
    TotalySupply,
}

#[public]
pub fn add_liquidity(context: Context<StateKeys>, amount_x: u64, amount_y: u64) -> u64 {
    let total_supply = total_supply(&context);
    // tokens    | shares
    // -------------------------
    // amount_x  | minted
    // reserve_x | total_supply
    let (reserve_x, reserve_y) = reserves(&context);
    let minted = if total_supply == 0 {
        let minted = amount_x;
        assert_eq!(minted, amount_y);
        minted
    } else {
        let minted = amount_x * total_supply / reserve_x;
        assert_eq!(minted, amount_y * total_supply / reserve_y); // make sure that the ratio is good
        minted
    };

    context
        .store([
            (StateKeys::ReserveX, &(reserve_x + amount_x)),
            (StateKeys::ReserveY, &(reserve_y + amount_y)),
            (StateKeys::TotalySupply, &(total_supply + minted)),
        ])
        .unwrap();

    minted
}

#[public]
pub fn remove_liquidity(context: Context<StateKeys>, shares: u64) -> (u64, u64) {
    let total_supply = total_supply(&context);
    let (reserve_x, reserve_y) = reserves(&context);
    let (amount_x, amount_y) = (
        shares * reserve_x / total_supply,
        shares * reserve_y / total_supply,
    );

    context
        .store([
            (StateKeys::ReserveX, &(reserve_x - amount_x)),
            (StateKeys::ReserveY, &(reserve_y - amount_y)),
            (StateKeys::TotalySupply, &(total_supply - shares)),
        ])
        .unwrap();

    (amount_x, amount_y)
}

#[public]
pub fn swap(context: Context<StateKeys>, amount_in: u64, x_to_y: bool) -> u64 {
    let total_supply = total_supply(&context);
    assert!(total_supply > 0, "no liquidity");
    // x * y = constant
    // x' = x + dx
    // y' = y + dy
    // (x + dx) * (y + dy) = x * y
    // y + dy = (x * y) / (x + dx)
    // dy = ((x * y) / (x + dx)) - y
    // skip a few steps
    // -dy = y * dx / (x + dx)
    let (reserve_x, reserve_y) = reserves(&context);
    let (reserve_x, reserve_y, out) = if x_to_y {
        let dy = (reserve_y * amount_in) / (reserve_x + amount_in);
        (reserve_x + amount_in, reserve_y - dy, dy)
    } else {
        let dx = (reserve_x * amount_in) / (reserve_y + amount_in);
        (reserve_x - dx, reserve_y + amount_in, dx)
    };

    context
        .store([
            (StateKeys::ReserveX, &reserve_x),
            (StateKeys::ReserveY, &reserve_y),
        ])
        .unwrap();

    out
}

fn total_supply(context: &Context<StateKeys>) -> u64 {
    context
        .get(StateKeys::TotalySupply)
        .unwrap()
        .unwrap_or_default()
}

fn reserves(context: &Context<StateKeys>) -> (u64, u64) {
    (
        context
            .get(StateKeys::ReserveX)
            .unwrap()
            .unwrap_or_default(),
        context
            .get(StateKeys::ReserveY)
            .unwrap()
            .unwrap_or_default(),
    )
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
