use wasmlanche_sdk::simulator::{
    Client, Endpoint, ExecuteResponse, Key, Param, ParamType, Plan, Step,
};

// TODO: remove when the simulator merges
#[allow(dead_code)]
const PROGRAM_ID: &str = "0000000000000000000000000000000000000000000000001";

// TODO: remove when the simulator merges
#[allow(dead_code)]
fn initialize_plan<'a>(
    nft_name: &'a str,
    nft_name_length: &'a str,
    nft_symbol: &'a str,
    nft_symbol_length: &'a str,
    nft_uri: &'a str,
    nft_uri_length: &'a str,
) -> Plan<'a> {
    let steps = vec![
        Step {
            description: "create program",
            endpoint: Endpoint::Execute,
            method: "program_create",
            params: vec![Param {
                name: "program_path",
                param_type: ParamType::String,
                value: "../../examples/testdata/single_nft.wasm",
            }],
            require: None,
        },
        Step {
            description: "init",
            endpoint: Endpoint::Execute,
            method: "init",
            params: vec![
                Param {
                    name: "program_id",
                    param_type: ParamType::Id,
                    value: PROGRAM_ID,
                },
                Param {
                    name: "nft_name",
                    param_type: ParamType::String,
                    value: nft_name,
                },
                Param {
                    name: "nft_name_length",
                    param_type: ParamType::U64,
                    value: nft_name_length,
                },
                Param {
                    name: "nft_symbol",
                    param_type: ParamType::String,
                    value: nft_symbol,
                },
                Param {
                    name: "nft_symbol_length",
                    param_type: ParamType::U64,
                    value: nft_symbol_length,
                },
                Param {
                    name: "nft_uri",
                    param_type: ParamType::String,
                    value: nft_uri,
                },
                Param {
                    name: "nft_uri_length",
                    param_type: ParamType::U64,
                    value: nft_uri_length,
                },
            ],
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
