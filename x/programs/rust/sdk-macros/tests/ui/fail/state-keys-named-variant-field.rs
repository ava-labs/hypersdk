use wasmlanche_sdk::state_keys;

#[state_keys]
pub enum StateKeys {
    Valild,
    OtherValid(u8),
    Invalid { foo: u8 },
}

fn main() {}
