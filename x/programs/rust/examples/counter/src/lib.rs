use expose_macro::expose;
use wasmlanche_sdk::program::{Program, ProgramValue};
use wasmlanche_sdk::store::ProgramContext;
use wasmlanche_sdk::types::Address;

/// Initializes the program. This program maps addresses with a count.
#[expose]
fn init_program() -> i64 {
    let mut counter_program = Program::new();
    counter_program.add_field(String::from("counter"), 0.into());
    counter_program.add_field(String::from("counts"), ProgramValue::MapObject);
    counter_program.publish().unwrap().into()
}

/// Increments the count at the address by the amount.
#[expose]
fn inc(ctx: ProgramContext, to: Address, amount: i64) {
    let counter = amount + value(ctx.clone(), to);
    // dont check for error/ok
    let _ = ctx.store_map_value(
        "counts",
        ProgramValue::from(to),
        ProgramValue::IntObject(counter),
    );
}

/// Gets the count at the address.
#[expose]
fn value(ctx: ProgramContext, of: Address) -> i64 {
    ctx.get_map_value("counts", ProgramValue::from(of))
        .map(i64::from)
        .unwrap_or(0)
}
