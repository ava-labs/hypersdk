use wasmlanche_sdk::{public, state_keys, Context, Program};

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
    let program = context.program();
    let total_supply = total_supply(program);
    // tokens    | shares
    // -------------------------
    // amount_x  | minted
    // reserve_x | total_supply
    let (reserve_x, reserve_y) = reserves(program);
    let minted = amount_x * total_supply / reserve_x;
    assert_eq!(minted, amount_y * total_supply / reserve_y); // make sure that the ratio is good
    program
        .state()
        .store(StateKeys::TotalySupply, &(total_supply + minted))
        .unwrap();
    minted
}

#[public]
pub fn remove_liquidity(context: Context<StateKeys>, shares: u64) -> (u64, u64) {
    let program = context.program();
    let total_supply = total_supply(program);
    let (reserve_x, reserve_y) = reserves(program);
    let (amount_x, amount_y) = (
        shares * reserve_x / total_supply,
        shares * reserve_y / total_supply,
    );
    program
        .state()
        .store(StateKeys::TotalySupply, &(total_supply - shares))
        .unwrap();
    (amount_x, amount_y)
}

#[public]
pub fn swap(context: Context<StateKeys>, amount_in: u64, x_to_y: bool) -> u64 {
    // k = x * y
    // x' * y' = x * y
    // (x + dx) * (y - dy) = x * y
    // y - dy = (x * y) / (x + dx)
    // dy = y - (x * y) / (x + dx)
    // dy = y * dx / (x + dx)
    let (reserve_x, reserve_y) = reserves(context.program());
    if x_to_y {
        (reserve_y * amount_in) / (reserve_x + amount_in)
    } else {
        (reserve_x * amount_in) / (reserve_y + amount_in)
    }
}

fn total_supply(program: &Program<StateKeys>) -> u64 {
    program
        .state()
        .get(StateKeys::TotalySupply)
        .unwrap()
        .unwrap_or_default()
}

fn reserves(program: &Program<StateKeys>) -> (u64, u64) {
    (
        program
            .state()
            .get(StateKeys::ReserveX)
            .unwrap()
            .unwrap_or_default(),
        program
            .state()
            .get(StateKeys::ReserveY)
            .unwrap()
            .unwrap_or_default(),
    )
}

#[cfg(test)]
mod tests {
    use simulator::{Endpoint, Key, Param, Step, StepError, StepResponseError};
    use wasmlanche_sdk::ExternalCallError;

    const PROGRAM_PATH: &str = env!("PROGRAM_PATH");

    #[test]
    fn add_liquidity() {
        let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        let owner = "owner";

        let program_id = simulator
            .run_step(owner, &Step::create_program(PROGRAM_PATH))
            .unwrap()
            .id;

        simulator
            .run_step(owner, &Step::create_key(Key::Ed25519(owner.to_string())))
            .unwrap();

        let resp_err = simulator
            .run_step(
                owner,
                &Step {
                    endpoint: Endpoint::Execute,
                    method: "remove_liquidity".to_string(),
                    max_units: u64::MAX,
                    params: vec![program_id.into(), 100000u64.into()],
                },
            )
            .unwrap()
            .result
            .response::<(u64, u64)>()
            .unwrap_err();

        let StepResponseError::ExternalCall(call_err) = resp_err else {
            panic!("wrong error returned");
        };

        assert_eq!(call_err, ExternalCallError::CallPanicked);
    }
}
