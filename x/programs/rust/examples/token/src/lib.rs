use wasmlanche_sdk::{public, store::State, types::Address};

/// Initializes the program with a name, symbol, and total supply.
#[public]
pub fn init(state: State) -> bool {
    state
        .store_value("total_supply", &123456789_i64)
        .store_value("name", "WasmCoin")
        .store_value("symbol", "WACK")
        .is_ok()
}

/// Gets total supply or -1 on error.
#[public]
pub fn get_total_supply(state: State) -> i64 {
    state.get_value("total_supply").unwrap()
}

/// Adds amount coins to the recipients balance.
#[public]
pub fn mint_to(state: State, recipient: Address, amount: i64) -> bool {
    let amount = amount + state.get_map_value("balances", &recipient).unwrap_or(0);
    state
        .store_map_value("balances", &recipient, &amount)
        .is_ok()
}

/// Transfers amount coins from the sender to the recipient. Returns whether successful.
#[public]
pub fn transfer(state: State, sender: Address, recipient: Address, amount: i64) -> bool {
    // require sender != recipient
    if sender == recipient {
        return false;
    }
    // ensure the sender has adequate balance
    let sender_balance: i64 = state.get_map_value("balances", &sender).unwrap_or(0);
    if amount < 0 || sender_balance < amount {
        return false;
    }
    let recipient_balance: i64 = state.get_map_value("balances", &recipient).unwrap_or(0);
    state
        .store_map_value("balances", &sender, &(sender_balance - amount))
        .store_map_value("balances", &recipient, &(recipient_balance + amount))
        .is_ok()
}

/// Gets the balance of the recipient.
#[public]
pub fn get_balance(state: State, recipient: Address) -> i64 {
    state.get_map_value("balances", &recipient).unwrap_or(0)
}
