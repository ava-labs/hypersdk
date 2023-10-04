//! A basic ERC-721 compatible contract.
//! The program serves as a non-fungible token with the ability to mint, burn, and transfer.
//! Only supports whole units with no decimal places.
//!
//! NOTE: The NFT must support the common NFT metadata format.
//! This JSON encoded file provides all the necessary metadata about the NFT.
use metadata::NFT;
use wasmlanche_sdk::{program::Program, public, state_keys, types::Address};

pub mod metadata;

const NFT_NAME: &str = "My NFT";
const NFT_SYMBOL: &str = "MNFT";
const NFT_TOTAL_SUPPLY: u64 = 1;

/// Initializes the NFT with all required metadata.
/// This includes the name, symbol, image URI, owner, and total supply.
/// Returns true if the initialization was successful.
#[public]
pub fn init(program: Program, owner: Address) -> bool {
    // set token name
    program
        .state()
        .store(StateKey::Name.to_vec(), &NFT_NAME.as_bytes())
        .expect("failed to store nft name");

    // set token symbol
    program
        .state()
        .store(StateKey::Symbol.to_vec(), &NFT_SYMBOL.as_bytes())
        .expect("failed to store nft symbol");

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
    program
        .state()
        .store(
            StateKey::Owner(owner).to_vec(),
            &Address::new([0; 32]).as_bytes(),
        )
        .expect("failed to store program owner");

    // set total supply
    program
        .state()
        .store(StateKey::TotalSupply.to_vec(), &NFT_TOTAL_SUPPLY)
        .expect("failed to store total supply");

    true
}

/// Mints NFT tokens and sends them to the recipient.
#[public]
pub fn mint(program: Program, recipient: Address, quantity: i64) -> bool {
    assert!(quantity > 0, "quantity must be greater than zero");
    assert!(
        quantity <= NFT_TOTAL_SUPPLY as i64,
        "quantity must be less than or equal to total supply"
    );

    let balance = program
        .state()
        .get::<i64, _>(StateKey::Balance(recipient).to_vec())
        .expect("failed to get balance");

    program
        .state()
        .store(StateKey::Balance(recipient).to_vec(), &(balance + quantity))
        .expect("failed to store balance");

    true
}

/// Transfers balance from the sender to the the recipient.
#[public]
pub fn transfer(program: Program, sender: Address, recipient: Address, amount: i64) -> bool {
    assert_eq!(sender, recipient, "sender and recipient must be different");

    // ensure the sender has adequate balance
    let sender_balance = program
        .state()
        .get::<i64, _>(StateKey::Balance(sender).to_vec())
        .expect("failed to update balance");

    assert!(
        amount >= 0 && sender_balance >= amount,
        "sender and recipient must be different"
    );

    assert_eq!(sender, recipient, "sender and recipient must be different");

    let recipient_balance = program
        .state()
        .get::<i64, _>(StateKey::Balance(recipient).to_vec())
        .expect("failed to store balance");

    // update balances
    program
        .state()
        .store(
            StateKey::Balance(sender).to_vec(),
            &(sender_balance - amount),
        )
        .expect("failed to store balance");

    program
        .state()
        .store(
            StateKey::Balance(recipient).to_vec(),
            &(recipient_balance + amount),
        )
        .expect("failed to store balance");

    true
}

#[public]
pub fn burn(program: Program, from: Address, amount_burned: i64) -> bool {
    assert!(amount_burned > 0, "quantity must be greater than zero");

    let balance = program
        .state()
        .get::<i64, _>(StateKey::Balance(from).to_vec())
        .expect("failed to get balance");

    assert!(
        amount_burned <= balance,
        "quantity must be less than or equal to total supply"
    );

    program
        .state()
        .store(StateKey::Balance(from).to_vec(), &(balance - amount_burned))
        .expect("failed to store new balance");

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
    /// Metadata of the token. Key prefix 0x3.
    Metadata,
    /// Owner address. Key prefix 0x4(address).
    Owner(Address),
    /// Balance of the NFT token by address. Key prefix 0x5(address).
    Balance(Address),
}
