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

/// The program storage keys.
#[state_keys]
enum StateKey {
    /// The total supply of the token. Key prefix 0x0.
    MaxSupply,
    /// The name of the token. Key prefix 0x1.
    Name,
    /// The symbol of the token. Key prefix 0x2.
    Symbol,
    /// Metadata of the token. Key prefix 0x3.
    Uri,
    /// Balances of the NFT token by address. Key prefix 0x4(address).
    Balances(Address),
    /// Counter -- used to keep track of total NFTs minted. Key prefix 0x5.
    Counter,
}

/// Initializes the NFT with all required metadata.
/// This includes the name, symbol, image URI, owner, and total supply.
/// Returns true if the initialization was successful.
#[public]
// TODO: update the macro to enable String arguments
#[allow(clippy::too_many_arguments)]
pub fn init(
    program: Program,
    nft_name_ptr: i64,
    nft_name_length: i64,
    nft_symbol_ptr: i64,
    nft_symbol_length: i64,
    nft_uri_ptr: i64,
    nft_uri_length: i64,
    nft_max_supply: i64,
) -> bool {
    let name_ptr = Memory::new(Pointer::from(nft_name_ptr));
    let nft_name = unsafe { name_ptr.range(nft_name_length as usize) };

    let nft_symbol_ptr = Memory::new(Pointer::from(nft_symbol_ptr));
    let nft_symbol = unsafe { nft_symbol_ptr.range(nft_symbol_length as usize) };

    let nft_uri_ptr = Memory::new(Pointer::from(nft_uri_ptr));
    let nft_uri = unsafe { nft_uri_ptr.range(nft_uri_length as usize) };

    let counter = program
        .state()
        .get::<i64, _>(StateKey::Counter.to_vec())
        .unwrap_or_default();

    assert_eq!(counter, 0, "init already called");

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
        .store(StateKey::MaxSupply.to_vec(), &nft_max_supply)
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
pub fn mint(program: Program, recipient: Address, amount: i64) -> bool {
    let counter = program
        .state()
        .get::<i64, _>(StateKey::Counter.to_vec())
        .unwrap_or_default();

    let max_supply = program
        .state()
        .get::<i64, _>(StateKey::MaxSupply.to_vec())
        .unwrap_or_default();

    assert!(
        counter + amount <= max_supply,
        "max supply for nft exceeded"
    );

    let balance = program
        .state()
        .get::<i64, _>(StateKey::Balances(recipient).to_vec())
        .unwrap_or_default();

    // TODO: check for overflow
    program
        .state()
        .store(StateKey::Balances(recipient).to_vec(), &(balance + amount))
        .expect("failed to store balance");

    // TODO: check for overflow
    program
        .state()
        .store(StateKey::Counter.to_vec(), &(counter + amount))
        .is_ok()
}

#[public]
pub fn burn(program: Program, from: Address, amount: i64) -> bool {
    let balance = program
        .state()
        .get::<i64, _>(StateKey::Balances(from).to_vec())
        .unwrap_or_default();

    assert!(
        balance >= amount,
        "balance must be greater than or equal to amount burned"
    );

    let counter = program
        .state()
        .get::<i64, _>(StateKey::Counter.to_vec())
        .unwrap_or_default();

    assert!(counter >= amount, "cannot burn more nfts");

    // TODO: check for underflow
    program
        .state()
        .store(StateKey::Balances(from).to_vec(), &(balance - amount))
        .is_ok()
}

#[public]
fn balance(program: Program, owner: Address) -> i64 {
    program
        .state()
        .get::<i64, _>(StateKey::Balances(owner).to_vec())
        .unwrap_or_default()
}
