

use crate::{program::Program, types::Bytes32};
use ed25519_dalek::{Signature, VerifyingKey};


#[link(wasm_import_module = "crypto")]
extern "C" {
    #[link_name = "verify_ed25519"]
    fn _verify_ed25519(
        caller_id: i64,
        msgPtr: i64,
        msgLen: i64,
        sigPtr: i64,
        pubKeyPtr: i64,
    ) -> i32;
}


#[must_use]
pub fn verify_ed25519(
    caller: &Program,
    msg: Bytes32,
    sig: &Signature,
    pub_key: &VerifyingKey,
)  -> i32 {
    unsafe { _verify_ed25519(caller.id(),
                            msg.as_bytes().as_ptr() as i64,
                            Bytes32::LEN as i64,
                            sig.to_bytes().as_ptr() as i64,
                            pub_key.as_bytes().as_ptr() as i64
    ) }
}

