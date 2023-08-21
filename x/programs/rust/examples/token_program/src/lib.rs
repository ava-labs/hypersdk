use wasmlanche_sdk::program::{Program, ProgramValue};
use wasmlanche_sdk::store::ProgramContext;
use wasmlanche_sdk::types::Address;

use expose_macro::expose;

/// Initializes the contract with a name, symbol, and total supply.
/// Technically adding the balances field is not necessary.
#[expose]
pub fn init_program() -> i64 {
    let mut token_program = Program::new();
    token_program.add_field(String::from("name"), "WasmCoin".into());
    token_program.add_field(String::from("symbol"), "WACK".into());
    token_program.add_field(String::from("total_supply"), 123456789.into());
    token_program.add_field(String::from("balances"), ProgramValue::MapObject);
    token_program.publish().unwrap().into()
}

/// Gets total supply or -1 on error.
#[expose]
pub fn get_total_supply(ctx: ProgramContext) -> i64 {
    ctx.get_value("total_supply")
        .unwrap_or(ProgramValue::IntObject(-1))
        .into()
}

/// Adds amount coins to the recipients balance.
#[expose]
pub fn mint_to(ctx: ProgramContext, recipient: Address, amount: i64) -> bool {
    let amount = amount
        + ctx
            .get_map_value("balances", recipient.into())
            .map(i64::from)
            .unwrap_or(0);

    ctx.store_map_value(
        "balances",
        ProgramValue::from(recipient),
        ProgramValue::IntObject(amount),
    )
    .is_ok()
}

/// Transfers amount coins from the sender to the recipient. Returns whether successful.
#[expose]
pub fn transfer(ctx: ProgramContext, sender: Address, recipient: Address, amount: i64) -> bool {
    // require sender != recipient
    if sender == recipient {
        return false;
    }
    // ensure the sender has adequate balance
    let sender_balance = ctx
        .get_map_value("balances", ProgramValue::from(sender))
        .map(i64::from)
        .unwrap_or(-1);

    if amount < 0 || sender_balance < amount {
        return false;
    }

    let recipient_balance = ctx
        .get_map_value("balances", ProgramValue::from(recipient))
        .map(i64::from)
        .unwrap_or(0);
    ctx.store_map_value(
        "balances",
        ProgramValue::from(sender),
        ProgramValue::IntObject(sender_balance - amount),
    )
    .and_then(|_| {
        ctx.store_map_value(
            "balances",
            ProgramValue::from(recipient),
            ProgramValue::IntObject(recipient_balance + amount),
        )
    })
    .is_ok()
}

/// Gets the balance of the recipient.
#[expose]
pub fn get_balance(ctx: ProgramContext, recipient: Address) -> i64 {
    ctx.get_map_value("balances", ProgramValue::from(recipient))
        .map(i64::from)
        .unwrap_or(0)
}
