use wasmlanche_sdk::state_schema;

state_schema! {
    Key => u64,
    Tuple(u8, u8) => u8,
    TupleReturn => (u8, u8),
    TupleKeyAndReturn(u8, u8) => (u8, u8),
}

fn main() {}
