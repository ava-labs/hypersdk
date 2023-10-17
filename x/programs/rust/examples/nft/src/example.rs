use wasmlanche_sdk::simulator::{Client, Endpoint, Key, Param, ParamType, Plan, Step};

fn initialize_plan() -> Plan<'static> {
    let steps: Vec<Step> = vec![
        Step {
            description: "init",
            endpoint: Endpoint::Execute,
            method: "init",
            params: vec![],
            require: None,
        },
        Step {
            description: "create alice key",
            endpoint: Endpoint::Key,
            method: "create",
            params: vec![Param {
                name: "key name",
                param_type: ParamType::Key(Key::Ed25519),
                value: "alice key",
            }],
            require: None,
        },
        Step {
            description: "mint NFT for alice",
            endpoint: Endpoint::Execute,
            method: "mint_to",
            params: vec![Param {
                name: "recipient",
                param_type: ParamType::Key(Key::Ed25519),
                value: "alice_key",
            }],
            require: None,
        },
    ];

    let plan = Plan {
        name: "nft program",
        description: "initialize nft program",
        caller_key: "alice key",
        steps,
    };

    plan
}

fn run() {
    let plan = initialize_plan();
    let client = Client::new("path to simulator".to_owned());

    let _ = client.run(&plan);
}

/*
TODO:
Add generic parameter to client.run
Determine whether to pass the program id to the plan -- how to get program id beforehand?
*/
