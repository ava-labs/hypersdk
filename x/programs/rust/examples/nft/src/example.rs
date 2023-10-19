use wasmlanche_sdk::simulator::{
    Client, Endpoint, ExecuteResponse, Key, Param, ParamType, Plan, Step,
};

// TODO: remove when the simulator merges
#[allow(dead_code)]
const PROGRAM_ID: &str = "0000000000000000000000000000000000000000000000001";

// TODO: remove when the simulator merges
#[allow(dead_code)]
fn initialize_plan() -> Plan<'static> {
    let steps = vec![
        Step {
            description: "init",
            endpoint: Endpoint::Execute,
            method: "init",
            params: vec![Param {
                name: "program_id",
                param_type: ParamType::Id,
                value: PROGRAM_ID,
            }],
            require: None,
        },
        Step {
            description: "create alice key",
            endpoint: Endpoint::Key,
            method: "create",
            params: vec![
                Param {
                    name: "program_id",
                    param_type: ParamType::Id,
                    value: PROGRAM_ID,
                },
                Param {
                    name: "key name",
                    param_type: ParamType::Key(Key::Ed25519),
                    value: "alice key",
                },
            ],
            require: None,
        },
        Step {
            description: "mint NFT for alice",
            endpoint: Endpoint::Execute,
            method: "mint_to",
            params: vec![
                Param {
                    name: "program_id",
                    param_type: ParamType::Id,
                    value: PROGRAM_ID,
                },
                Param {
                    name: "recipient",
                    param_type: ParamType::Key(Key::Ed25519),
                    value: "alice_key",
                },
            ],
            require: None,
        },
    ];

    Plan {
        name: "nft program",
        description: "run the nft program",
        caller_key: "alice key",
        steps,
    }
}

// TODO: remove when the simulator merges
#[allow(dead_code)]
fn run() -> Result<ExecuteResponse, Box<dyn std::error::Error>> {
    let plan = initialize_plan();
    let client = Client::new("path to simulator".to_owned());

    let tx = client.run::<ExecuteResponse>(&plan)?;
    Ok(tx)
}
