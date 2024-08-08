use seq_macro::seq;
use wasmlanche_sdk::state_schema;

seq!(N in 0..=255 {
    state_schema! {
        #(
            Variant~N => u8,
        )*
        Variant256 => u8,
    }
});

fn main() {}
