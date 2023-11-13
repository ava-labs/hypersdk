
use crate::program::Program;

#[link(wasm_import_module = "crypto")]
extern "C" {
    #[link_name = "verify_ed25519"]
    fn _verify_ed25519(
        caller_id: i64
    ) -> i32;
}

#[must_use]
pub fn verify_ed25519(
    caller: &Program,
) -> i32 {
    unsafe { _verify_ed25519(caller.id()) }
}

