use sdk_dec_macros::{assert_eq, assert_gt, require};
use wasmlanche_sdk::{public, store::State, types::Address};

/// @title A basic ERC-20 compatible contract
/// @author Ava Labs
/// @notice Serves as a fungible token
/// @dev Limits the token functionality to the bare minimum required

/// @notice Initializes the program with a name, symbol, and total supply
/// @param state The initial program state
/// @return Whether the initialization succeeded
#[public]
// TODO: parameterize initial fn arguments
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
    // Require sender != recipient.
    if !assert_eq!(sender, recipient) {
        return false;
    }

    // Ensure amount sent is greater than 0.
    if !assert_gt!(amount, 0) {
        return false;
    }

    // Ensure the sender has adequate balance.
    let sender_balance: i64 = state.get_map_value("balances", &sender).unwrap_or(0);
    if !assert_gt!(sender_balance, amount) {
        return false;
    }

    let recipient_balance: i64 = state.get_map_value("balances", &recipient).unwrap_or(0);
    let sender_delta = sender_balance.checked_sub(amount);
    let recipient_delta = recipient_balance.checked_add(amount);
    if !assert_eq!(
        require!(sender_delta.is_some()),
        require!(recipient_delta.is_some())
    ) {
        return false;
    }

    if let (Some(sender_delta), Some(recipient_delta)) = (sender_delta, recipient_delta) {
        state
            .store_map_value("balances", &sender, &sender_delta)
            .store_map_value("balances", &recipient, &recipient_delta)
            .is_ok()
    } else {
        false
    }
}

/// @notice Gets the balance of the recipient
/// @param state The current program state
/// @param address The address whose balance will be returned
/// @return The balance of the recipient
#[public]
pub fn get_balance(state: State, address: Address) -> i64 {
    state.get_map_value("balances", &address).unwrap_or(0)
}
