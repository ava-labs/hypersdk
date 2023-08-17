/// Counter but only for even numbers
use expose_macro::expose;
use wasmlanche_sdk::program::{Program, ProgramValue};
use wasmlanche_sdk::store::ProgramContext;
use wasmlanche_sdk::types::Address;

#[expose]
fn init_contract() -> i64 {
    let token_contract = Program::new();
    token_contract.publish().unwrap().into()
}

#[expose]
fn set(ctx: ProgramContext, counter_ctx: ProgramContext) {
    ctx.store_value("counter", &ProgramValue::ProgramObject(counter_ctx))
        .expect("Failed to store token contract address");
}

/// Calls the counter program to increment by twice the amount.
#[expose]
fn inc(ctx: ProgramContext, whose: Address, amt: i64) {
    let call_ctx = match ctx.get_value("counter") {
        Ok(value) => ProgramContext::from(value),
        Err(_) => {
            // Can return error here, up to smart contract designer. Skipping for now.
            return;
        }
    };
    ctx.program_invoke(
        &call_ctx,
        "inc",
        &[ProgramValue::from(whose), ProgramValue::IntObject(amt * 2)],
    );
}

/// Returns the value of whose's counter from the counter program.
#[expose]
fn value(ctx: ProgramContext, whose: Address) -> i64 {
    let call_ctx = match ctx.get_value("counter") {
        Ok(value) => ProgramContext::from(value),
        Err(_) => {
            // Can return error here, up to smart contract designer. Skipping for now.
            return 0;
        }
    };

    let result = ctx.program_invoke(&call_ctx, "value", &[ProgramValue::from(whose)]);
    i64::from(result)
}
