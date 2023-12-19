

use crate::{program::Program, memory::to_smart_ptr};
use ed25519_dalek::{PUBLIC_KEY_LENGTH, SIGNATURE_LENGTH};
use borsh::{to_vec, BorshDeserialize, BorshSerialize};

#[derive(BorshDeserialize, BorshSerialize)]
pub struct SignedMessage {
    pub message: [u8; 32],
    pub signature: [u8; SIGNATURE_LENGTH],
    pub public_key: [u8; PUBLIC_KEY_LENGTH],
}


#[link(wasm_import_module = "crypto")]
extern "C" {
    #[link_name = "verify_ed25519"]
    fn _verify_ed25519(
        caller_id: i64,
        signedMsg: i64,
    ) -> i32;

    #[link_name = "batch_verify_ed25519"]
    fn _batch_verify_ed25519(
        caller_id: i64,
        signedMsgs: i64,
    ) -> i32;
}


#[must_use]
pub fn verify_ed25519(
    caller: &Program,
    signed_message: &SignedMessage,
)  -> i32 {
    let caller = to_smart_ptr(caller.id()).unwrap();
    let signed_msg_bytes = to_vec(signed_message).unwrap();
    let signed_message = to_smart_ptr(&signed_msg_bytes).unwrap();  
    unsafe { _verify_ed25519(caller,
                                signed_message,
    ) }
}
 
#[must_use]
pub fn batch_verify_ed25519(
    caller: &Program,
    signed_messages: &[SignedMessage],
) -> i32 {
    let caller = to_smart_ptr(caller.id()).unwrap();
    let signed_msg_bytes = to_vec(signed_messages).unwrap();
    let signed_messages = to_smart_ptr(&signed_msg_bytes).unwrap();  
    unsafe { _batch_verify_ed25519(caller,
                                signed_messages,
    ) }
}
