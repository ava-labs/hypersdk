use wasmlanche_sdk::{public, store::State, types::Address};

/// @title A basic ERC-20 contract 
/// @author Ava Labs 
/// @notice Serves as a fungible token
/// @dev Limits the token functionality to the bare minimum

/// @notice Initializes the program with a name, symbol, and total supply
/// @param state The initial program state 
/// @return Whether the initialization succeeded
#[public]
pub fn init(state: State) -> bool {
    state
        .store_value("total_supply", &123456789_i64)
        .store_value("name", "WasmCoin")
        .store_value("symbol", "WACK")
        .is_ok()
}

/// @notice Gets total token supply or -1 on error
/// @param state The current program state
/// @return The total supply of the token
#[public]
pub fn get_total_supply(state: State) -> i64 {
    state.get_value("total_supply").unwrap()
}

/// @notice Mints coins to the recipients balance.
/// @param state The current program state
/// @param recipient The address whose balance will be increased
/// @param amount The amount of tokens to add to the recipient's balance
/// @return Whether the minting succeeded
#[public]
pub fn mint_to(state: State, recipient: Address, amount: i64) -> bool {
    let amount = amount + state.get_map_value("balances", &recipient).unwrap_or(0);
    state
        .store_map_value("balances", &recipient, &amount)
        .is_ok()
}

/// @notice Transfers coins from the sender to the recipient
/// @param state The current program state
/// @param sender The address sending tokens
/// @param recipient The address receiving tokens
/// @param amount The amount of tokens to transfer
/// @return Whether the transfer succeeded
#[public]
pub fn transfer(state: State, sender: Address, recipient: Address, amount: i64) -> bool {
    // require sender != recipient
    assert_ne!(sender, recipient);

    // ensure the sender has adequate balance
    let sender_balance: i64 = state.get_map_value("balances", &sender).unwrap_or(0);
    assert!(amount >= 0 && sender_balance >= amount);

    let recipient_balance: i64 = state.get_map_value("balances", &recipient).unwrap_or(0);
    state
        .store_map_value("balances", &sender, &(sender_balance - amount))
        .store_map_value("balances", &recipient, &(recipient_balance + amount))
        .is_ok()
}

/// @notice Gets the balance of the recipient
/// @param state The current program state
/// @param address The address whose balance will be returned
/// @return The balance of the recipient
#[public]
pub fn get_balance(state: State, address: Address) -> i64 {
    state.get_map_value("balances", &address).unwrap_or(0)
}
