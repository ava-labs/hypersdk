use wasmlanche_sdk::simulator::{
    Client, Endpoint, ExecuteResponse, Key, Param, ParamType, Plan, Step,
};

// TODO: remove when the simulator merges
const PROGRAM_ID: &str = "0000000000000000000000000000000000000000000000001";

pub fn initialize_plan<'a>(
    nft_name: &'a str,
    nft_name_length: &'a str,
    nft_symbol: &'a str,
    nft_symbol_length: &'a str,
    nft_uri: &'a str,
    nft_uri_length: &'a str,
) -> Plan {
    let steps = vec![
        Step {
            endpoint: Endpoint::Execute,
            method: "program_create".to_string(),
            max_units: 1000,
            params: vec![Param {
                param_type: ParamType::String,
                value: "../../examples/testdata/single_nft.wasm".to_string(),
            }],
            require: None,
        },
        Step {
            endpoint: Endpoint::Execute,
            method: "init".to_string(),
            max_units: 1000,
            params: vec![
                Param {
                    param_type: ParamType::Id,
                    value: PROGRAM_ID.to_string(),
                },
                Param {
                    param_type: ParamType::String,
                    value: nft_name.to_string(),
                },
                Param {
                    param_type: ParamType::U64,
                    value: nft_name_length.to_string(),
                },
                Param {
                    param_type: ParamType::String,
                    value: nft_symbol.to_string(),
                },
                Param {
                    param_type: ParamType::U64,
                    value: nft_symbol_length.to_string(),
                },
                Param {
                    param_type: ParamType::String,
                    value: nft_uri.to_string(),
                },
                Param {
                    param_type: ParamType::U64,
                    value: nft_uri_length.to_string(),
                },
            ],
            require: None,
        },
        Step {
            endpoint: Endpoint::Key,
            method: "create".to_string(),
            max_units: 1000,
            params: vec![Param {
                param_type: ParamType::Key(Key::Ed25519),
                value: "alice_key".to_string(),
            }],
            require: None,
        },
        Step {
            endpoint: Endpoint::Execute,
            method: "mint_to".to_string(),
            max_units: 1000,
            params: vec![
                Param {
                    param_type: ParamType::Id,
                    value: PROGRAM_ID.to_string(),
                },
                Param {
                    param_type: ParamType::Key(Key::Ed25519),
                    value: "alice_key".to_string(),
                },
            ],
            require: None,
        },
    ];

    Plan {
        caller_key: "alice_key".to_string(),
        steps,
    }
}

// TODO: remove when the simulator merges
#[allow(dead_code)]
fn run() -> Result<Vec<ExecuteResponse>, Box<dyn std::error::Error>> {
    let nft_name = "MyNFT";
    let binding = nft_name.len().to_string();
    let nft_name_length: &str = binding.as_ref();

    let nft_symbol = "MNFT";
    let binding = nft_symbol.len().to_string();
    let nft_symbol_length: &str = binding.as_ref();

    let nft_uri = "ipfs://my-nft.jpg";
    let binding = nft_uri.len().to_string();
    let nft_uri_length: &str = binding.as_ref();

    let plan = initialize_plan(
        nft_name,
        nft_name_length,
        nft_symbol,
        nft_symbol_length,
        nft_uri,
        nft_uri_length,
    );
    let client = Client::new("path to simulator".to_owned());

    let tx = client.run::<ExecuteResponse>(&plan)?;
    Ok(tx)
}
