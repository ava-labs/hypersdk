use wasmlanche_sdk::{public, types::Address, Context, ExternalCallContext, Program};

#[public]
pub fn inc(_: Context, external: Program, address: Address) {
    let ctx = ExternalCallContext::new(external, 1_000_000);
    // TODO: currently need to specify the return type (not good)
    counter::inc(ctx, address, 1);
}

#[public]
pub fn get_value(_: Context, external: Program, address: Address) -> u64 {
    let ctx = ExternalCallContext::new(external, 1_000_000);
    counter::get_value(ctx, address)
}

#[cfg(test)]
mod tests {
    use simulator::{ClientBuilder, Endpoint, Key, Param, Step};
    use std::error::Error;

    const PROGRAM_PATH: &str = env!("PROGRAM_PATH");

    #[test]
    fn inc_and_get_value() -> Result<(), Box<dyn Error>> {
        let mut simulator = ClientBuilder::new().try_build().unwrap();

        let counter_path = PROGRAM_PATH
            .replace("counter-external", "counter")
            .replace("counter_external", "counter");

        let owner = String::from("owner");
        let owner_key = Key::Ed25519(owner.clone());

        simulator.run_step::<()>(&owner, &Step::create_key(owner_key.clone()))?;
        let owner_key = Param::Key(owner_key);

        let counter_external = simulator
            .run_step::<()>(&owner, &Step::create_program(PROGRAM_PATH))?
            .base
            .id;
        let counter_external = Param::Id(counter_external.into());

        let counter = simulator
            .run_step::<()>(
                &owner,
                &Step {
                    endpoint: Endpoint::Execute,
                    method: "program_create".into(),
                    max_units: 1_000_000,
                    params: vec![Param::String(counter_path.clone())],
                },
            )?
            .base
            .id;
        let counter = Param::Id(counter.into());

        let params = vec![counter_external, counter.clone(), owner_key.clone()];

        simulator.run_step::<()>(
            &owner,
            &Step {
                endpoint: Endpoint::Execute,
                method: "inc".into(),
                max_units: 100_000_000,
                params: params.clone(),
            },
        )?;

        let response = simulator.run_step::<u64>(
            &owner,
            &Step {
                endpoint: Endpoint::ReadOnly,
                method: "get_value".into(),
                max_units: 1_000_000,
                params,
            },
        )?;

        assert_eq!(response.result.response, 1u64);

        Ok(())
    }
}
