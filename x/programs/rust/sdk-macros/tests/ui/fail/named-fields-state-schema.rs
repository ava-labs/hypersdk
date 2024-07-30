use wasmlanche_sdk::state_schema;

state_schema! {
    Valid => u8,
    OtherValid(u8) => u8,
    Invalid { param: u8 } => u8,
}

fn main() {}
