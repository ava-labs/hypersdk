use wasmlanche_sdk::{public, store::State};

pub mod metadata;

/// A basic ERC-721 compatible contract.
/// Serves as a non-fungible token with the ability to mint, burn, and pause.
/// Only supports whole units with no decimal places.
///
/// NOTE: The NFT must support the common NFT metadata format.
/// This JSON encoded file provides all the necessary metadata about the NFT.

/// Initializes the program with a name, symbol, and total supply
/// Returns whether the initialization was successful.
// TODO: Parametrize string inputs
#[public]
pub fn init(state: State) -> bool {
    state
        .store_value("total_supply", &10000_i64)
        .store_value("name", "NFToken")
        .store_value("symbol", "NFT")
        .is_ok()
}

/// Returns the total supply of the token.
#[public]
pub fn get_total_supply(state: State) -> i64 {
    state.get_value("total_supply").unwrap_or(0)
}
