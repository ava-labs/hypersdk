use seq_macro::seq;
use wasmlanche_sdk::state_keys;

seq!(N in 0..=255  {
    #[state_keys]
    pub enum StateKeysFail {
        #(
            Variant~N,
        )*
        Variant256,
    }
});

fn main() {}
