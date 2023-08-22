use wasmlanche_sdk::program::Program;
use wasmlanche_sdk::store::ProgramContext;
use wasmlanche_sdk::types::Address;

use expose_macro::expose;

/// Initializes the program with a name, symbol, and total supply.
#[expose]
pub fn init_program() -> i64 {
    let mut token_program = Program::new();

    token_program
        .add_field(String::from("name"), "WasmCoin")
        .and_then(|_| token_program.add_field(String::from("symbol"), "WACK"))
        .and_then(|_| token_program.add_field(String::from("total_supply"), 123456789))
        .unwrap();

    token_program.into()
}

/// Gets total supply or -1 on error.
#[expose]
pub fn get_total_supply(ctx: ProgramContext) -> i64 {
    ctx.get_value("total_supply").unwrap()
}

/// Adds amount coins to the recipients balance.
#[expose]
pub fn mint_to(ctx: ProgramContext, recipient: Address, amount: i64) -> bool {
    let amount: i64 = amount + ctx.get_map_value("balances", &recipient).unwrap_or(0);
    ctx.store_map_value("balances", &recipient, &amount).is_ok()
}

/// Transfers amount coins from the sender to the recipient. Returns whether successful.
#[expose]
pub fn transfer(ctx: ProgramContext, sender: Address, recipient: Address, amount: i64) -> bool {
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
        .and_then(|_| ctx.store_map_value("balances", &recipient, &(recipient_balance + amount)))
        .is_ok()
}

/// Gets the balance of the recipient.
#[expose]
pub fn get_balance(ctx: ProgramContext, recipient: Address) -> i64 {
    ctx.get_map_value("balances", &recipient).unwrap_or(0)
}
