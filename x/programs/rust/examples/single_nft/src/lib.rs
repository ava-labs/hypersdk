//! A basic NFT contract.
//! The program serves as a non-fungible token with the ability to mint and burn.
//! Only supports whole units with no decimal places.
//!
//! The NFT must support the common NFT metadata format.
//! This includes the name, symbol, and URI of the NFT.
use wasmlanche_sdk::{
    memory::{Memory, Pointer},
    program::Program,
    public, state_keys,
    types::Address,
};

pub mod example;

/// The total supply of the NFT.
/// # Safety
/// In this program only one NFT of each type is intended to be minted.
const TOTAL_SUPPLY: i64 = 1;

/// The program storage keys.
#[state_keys]
enum StateKey {
    /// The total supply of the token. Key prefix 0x0.
    TotalSupply,
    /// The name of the token. Key prefix 0x1.
    Name,
    /// The symbol of the token. Key prefix 0x2.
    Symbol,
    /// Metadata of the token. Key prefix 0x3.
    Uri,
    /// Balance of the NFT token by address. Key prefix 0x4(address).
    Balance(Address),
    /// Counter -- used to keep track of total NFTs minted. Key prefix 0x5.
    Counter,
    /// Owner -- used to keep track of the owner. Key prefix 0x6.
    Owner,
}

/// Initializes the NFT with all required metadata.
/// This includes the name, symbol, image URI, owner, and total supply.
/// Returns true if the initialization was successful.
#[public]
pub fn init(
    program: Program,
    nft_name_ptr: i64,
    nft_name_length: i64,
    nft_symbol_ptr: i64,
    nft_symbol_length: i64,
    nft_uri_ptr: i64,
    nft_uri_length: i64,
) -> bool {
    let name_ptr = Memory::new(Pointer::from(nft_name_ptr));
    let nft_name = unsafe { name_ptr.range(nft_name_length as usize) };

    let nft_symbol_ptr = Memory::new(Pointer::from(nft_symbol_ptr));
    let nft_symbol = unsafe { nft_symbol_ptr.range(nft_symbol_length as usize) };

    let nft_uri_ptr = Memory::new(Pointer::from(nft_uri_ptr));
    let nft_uri = unsafe { nft_uri_ptr.range(nft_uri_length as usize) };

    // Set token name
    program
        .state()
        .store(StateKey::Name.to_vec(), &nft_name)
        .expect("failed to store nft name");

    // Set token symbol
    program
        .state()
        .store(StateKey::Symbol.to_vec(), &nft_symbol)
        .expect("failed to store nft symbol");

    // Set token URI
    program
        .state()
        .store(StateKey::Uri.to_vec(), &nft_uri)
        .expect("failed to store nft uri");

    // Set total supply
    program
        .state()
        .store(StateKey::TotalSupply.to_vec(), &TOTAL_SUPPLY)
        .expect("failed to store total supply");

    // Initialize counter
    program
        .state()
        .store(StateKey::Counter.to_vec(), &0_i64)
        .expect("failed to store counter");

    true
}

/// Mints NFT tokens and sends them to the recipient.
#[public]
pub fn mint(program: Program, recipient: Address) -> bool {
    const MINT_AMOUNT: i64 = 1;

    let counter = program
        .state()
        .get::<i64, _>(StateKey::Counter.to_vec())
        .expect("failed to store ");

    assert!(counter <= TOTAL_SUPPLY, "max supply for nft exceeded");

    let balance = program
        .state()
        .get::<i64, _>(StateKey::Balance(recipient).to_vec())
        .unwrap_or_default();

    program
        .state()
        .store(
            StateKey::Balance(recipient).to_vec(),
            &(balance + MINT_AMOUNT),
        )
        .expect("failed to store balance");

    program
        .state()
        .store(StateKey::Owner.to_vec(), &recipient)
        .expect("failed to store owner");

    program
        .state()
        .store(StateKey::Counter.to_vec(), &(counter + 1_i64))
        .is_ok()
}

#[public]
pub fn burn(program: Program, from: Address) -> bool {
    const BURN_AMOUNT: i64 = 1;
    let null_address = Address::new([0; 32]);

    // Only the owner of the NFT can burn it
    let owner = program
        .state()
        .get::<Address, _>(StateKey::Owner.to_vec())
        .unwrap_or(null_address);

    assert_eq!(owner, from, "only the owner can burn the nft");

    let balance = program
        .state()
        .get::<i64, _>(StateKey::Balance(from).to_vec())
        .unwrap_or_default();

    assert!(
        BURN_AMOUNT <= balance,
        "amount burned must be less than or equal to the user balance"
    );

    let counter = program
        .state()
        .get::<i64, _>(StateKey::Counter.to_vec())
        .unwrap_or_default();

    assert!(counter > 0, "cannot burn more nfts");

    // Burn the NFT by transferring it to the zero address
    program
        .state()
        .store(StateKey::Balance(from).to_vec(), &(balance - BURN_AMOUNT))
        .expect("failed to store new balance");

    // TODO move to a lazy static? Or move to the VM layer entirely
    program
        .state()
        .store(StateKey::Owner.to_vec(), &null_address)
        .is_ok()
}

#[cfg(test)]
mod tests {
    use serial_test::serial;
    use std::env;
    use wasmlanche_sdk::simulator::{self, Key, PlanResponse};

    use crate::example;

    // export SIMULATOR_PATH=/path/to/simulator
    // export PROGRAM_PATH=/path/to/program.wasm
    // cargo cargo test --package token --lib nocapture -- tests::test_token_plan --exact --nocapture --ignored
    #[test]
    #[serial]
    #[ignore = "requires SIMULATOR_PATH and PROGRAM_PATH to be set"]
    fn test_single_nft_plan() {
        use wasmlanche_sdk::simulator::{self, Key};
        let s_path = env::var(simulator::PATH_KEY).expect("SIMULATOR_PATH not set");
        let simulator = simulator::Client::new(s_path);

        let alice_key = "alice_key";
        // create owner key in single step
        let resp = simulator
            .key_create::<PlanResponse>(alice_key, Key::Ed25519)
            .unwrap();
        assert_eq!(resp.error, None);

        // create multiple step test plan
        let nft_name = "MyNFT";
        let binding = nft_name.len().to_string();
        let nft_name_length: &str = binding.as_ref();

        let nft_symbol = "MNFT";
        let binding = nft_symbol.len().to_string();
        let nft_symbol_length: &str = binding.as_ref();

        let nft_uri = "ipfs://my-nft.jpg";
        let binding = nft_uri.len().to_string();
        let nft_uri_length: &str = binding.as_ref();

        let plan = example::initialize_plan(
            nft_name,
            nft_name_length,
            nft_symbol,
            nft_symbol_length,
            nft_uri,
            nft_uri_length,
        );

        // run plan
        let plan_responses = simulator.run::<PlanResponse>(&plan).unwrap();

        assert!(
            plan_responses.iter().all(|resp| resp.error.is_none()),
            "error: {:?}",
            plan_responses
                .iter()
                .filter_map(|resp| resp.error.as_ref())
                .next()
        );
    }

    #[test]
    #[serial]
    #[ignore = "requires SIMULATOR_PATH and PROGRAM_PATH to be set"]
    fn test_create_nft_program() {
        let s_path = env::var(simulator::PATH_KEY).expect("SIMULATOR_PATH not set");
        let simulator = simulator::Client::new(s_path);

        let alice_key = "alice_key";
        // create alice key in single step
        let resp = simulator
            .key_create::<PlanResponse>(alice_key, Key::Ed25519)
            .unwrap();
        assert_eq!(resp.error, None);

        let p_path = env::var("PROGRAM_PATH").expect("PROGRAM_PATH not set");
        // create a new program on chain.
        let resp = simulator
            .program_create::<PlanResponse>("owner", p_path.as_ref())
            .unwrap();
        assert_eq!(resp.error, None);
        assert!(resp.result.id.is_some());
    }
}
