//! A basic ERC-721 compatible contract.
//! Serves as a non-fungible token with the ability to mint, burn, and pause.
//! Only supports whole units with no decimal places.
//!
//! NOTE: The NFT must support the common NFT metadata format.
//! This JSON encoded file provides all the necessary metadata about the NFT.
use metadata::{Properties, TypeDescription, NFT};
use wasmlanche_sdk::{program::Program, public, state_keys};
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

    let nft_metadata = NFT::default()
        .with_title("My NFT".to_string())
        .with_type("object".to_string())
        .with_properties(Properties::default()
            .with_name(TypeDescription::default()
                .with_type(NFT_SYMBOL.to_string())
                .with_description("Identifies the asset to which this NFT represents".to_string())
            )
            .with_description(TypeDescription::default()
                .with_type(NFT_NAME.to_string())
                .with_description("Describes the asset to which this NFT represents".to_string())
        )
            .with_image(TypeDescription::default()
                .with_type("ipfs://bafybeidlkqhddsjrdue7y3dy27pu5d7ydyemcls4z24szlyik3we7vqvam/nft-image.png".to_string())
                .with_description("A URI pointing to a resource with mime type image".to_string())
        ));

    // set total supply
    program
        .state()
        .store(StateKey::TotalSupply.to_vec(), &20)
        .expect("failed to store total supply");

    // TODO: convert nft_metadata struct into bytes to persist to storage.
    // program
    //     .state()
    //     .store(StateKey::Metadata.to_vec(), nft_metadata)
    //     .expect("failed to store total supply");

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
    /// Edition (for example, edition 1 of 10)
    Edition,
}
