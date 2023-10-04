//! A basic ERC-721 compatible contract.
//! Serves as a non-fungible token with the ability to mint, burn, and pause.
//! Only supports whole units with no decimal places.
//!
//! NOTE: The NFT must support the common NFT metadata format.
//! This JSON encoded file provides all the necessary metadata about the NFT.
use metadata::NFT;
use wasmlanche_sdk::{program::Program, public, state_keys, types::Address};
pub mod metadata;

const NFT_NAME: &str = "My NFT";
const NFT_SYMBOL: &str = "MNFT";

/// Initializes the program with a name, symbol, and total supply
/// Returns whether the initialization was successful.
#[public]
pub fn init(program: Program) -> bool {
    // set token name
    program
        .state()
        .store(StateKey::Name.to_vec(), &NFT_NAME.as_bytes())
        .expect("failed to store coin name");

    // set token symbol
    program
        .state()
        .store(StateKey::Symbol.to_vec(), &NFT_SYMBOL.as_bytes())
        .expect("failed to store symbol");

    // set total supply
    program
        .state()
        .store(StateKey::TotalSupply.to_vec(), &20)
        .expect("failed to store total supply");

    // Generate NFT metadata and persist to storage
    let nft_metadata = NFT::default()
        .with_symbol(NFT_SYMBOL.to_string())
        .with_name(NFT_NAME.to_string())
        .with_uri("ipfs://my-nft.jpg".to_string());

    program
        .state()
        .store(StateKey::Metadata.to_vec(), &nft_metadata)
        .expect("failed to store nft metadata");

    // Store the original owner of the NFT (the contract creator)
    // TODO: get the program owner from the transaction context
    program
        .state()
        .store(StateKey::Owner.to_vec(), &Address::new([0; 32]).as_bytes())
        .expect("failed to store program owner");

    true
}

/// The program state keys.
#[state_keys]
enum StateKey {
    /// The total supply of the token. Key prefix 0x0.
    TotalSupply,
    /// The name of the token. Key prefix 0x1.
    Name,
    /// The symbol of the token. Key prefix 0x2.
    Symbol,
    /// Metadata of the token
    Metadata,
    /// Owner address
    Owner,
}
