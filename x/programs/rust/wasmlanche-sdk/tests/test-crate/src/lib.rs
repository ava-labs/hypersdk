use wasmlanche_sdk::{params, public, Context};

#[public]
pub fn always_true(_: Context) -> bool {
    true
}

#[public]
pub fn combine_last_bit_of_each_id_byte(context: Context) -> u32 {
    context
        .program
        .id()
        .iter()
        .map(|byte| *byte as u32)
        .fold(0, |acc, byte| (acc << 1) + (byte & 1))
}

// TODO move me to the actual test
#[public]
pub fn recursive(Context { program }: Context, units_left: i64) -> bool {
    let units = 100000;
    program
        .call_function(
            "recursive",
            &params!(&(units_left - units)).unwrap(),
            units_left - units,
        )
        .is_ok_and(|res| res)
}
