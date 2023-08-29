use wasmlanche_sdk::store::Context;
use wasmlanche_sdk::types::Address;

use expose_macro::expose;

/// Initializes the program with a name, symbol, and total supply.
#[expose]
pub fn init(ctx: Context) -> bool {
    ctx.store_value("total_supply", &123456789_i64)
        .store_value("name", "WasmCoin")
        .store_value("symbol", "WACK")
        .is_ok()
}

/// Gets total supply or -1 on error.
#[expose]
pub fn get_total_supply(ctx: Context) -> i64 {
    ctx.get_value("total_supply").unwrap()
}

/// Adds amount coins to the recipients balance.
#[expose]
pub fn mint_to(ctx: Context, recipient: Address, amount: i64) -> bool {
    let amount = amount + ctx.get_map_value("balances", &recipient).unwrap_or(0);
    ctx.store_map_value("balances", &recipient, &amount).is_ok()
}

/// Transfers amount coins from the sender to the recipient. Returns whether successful.
#[expose]
pub fn transfer(ctx: Context, sender: Address, recipient: Address, amount: i64) -> bool {
    // require sender != recipient
    if sender == recipient {
        return false;
    }
    // ensure the sender has adequate balance
    let sender_balance: i64 = ctx.get_map_value("balances", &sender).unwrap_or(0);
    if amount < 0 || sender_balance < amount {
        return false;
    }
    let recipient_balance: i64 = ctx.get_map_value("balances", &recipient).unwrap_or(0);
    ctx.store_map_value("balances", &sender, &(sender_balance - amount))
        .store_map_value("balances", &recipient, &(recipient_balance + amount))
        .is_ok()
}

/// Gets the balance of the recipient.
#[expose]
pub fn get_balance(ctx: Context, recipient: Address) -> i64 {
    ctx.get_map_value("balances", &recipient).unwrap_or(0)
}
