use crate::{errors::StateError, memory::to_smart_ptr, program::Program};
use borsh::{to_vec, BorshDeserialize, BorshSerialize};
use ed25519_dalek::{PUBLIC_KEY_LENGTH, SIGNATURE_LENGTH};

#[derive(BorshDeserialize, BorshSerialize)]
pub struct SignedMessage {
    pub message: Vec<u8>,
    pub signature: [u8; SIGNATURE_LENGTH],
    pub public_key: [u8; PUBLIC_KEY_LENGTH],
}

#[link(wasm_import_module = "crypto")]
extern "C" {
    #[link_name = "verify_ed25519"]
    fn _verify_ed25519(caller_id: i64, signedMsg: i64) -> i32;

    #[link_name = "batch_verify_ed25519"]
    fn _batch_verify_ed25519(caller_id: i64, signedMsgs: i64) -> i32;
}

/// Verifies a signed message by calling out to the host.
/// # Errors
/// Returns a `StateError` if the signed message cannot be serialized or if the argument cannot
/// be converted to a smart pointer.
pub fn verify_ed25519(caller: &Program, signed_message: &SignedMessage) -> Result<i32, StateError> {
    let caller = to_smart_ptr(caller.id())?;
    let signed_msg_bytes = to_vec(signed_message).map_err(|_| StateError::Serialization)?;
    let signed_message = to_smart_ptr(&signed_msg_bytes)?;
    Ok(unsafe { _verify_ed25519(caller, signed_message) })
}

/// Verifies a batch of signed messages by calling out to the host.
/// # Errors
/// Returns a `StateError` if the signed messages cannot be serialized or if the argument cannot
/// be converted to a smart pointer.
pub fn batch_verify_ed25519(
    caller: &Program,
    signed_messages: &[SignedMessage],
) -> Result<i32, StateError> {
    let caller = to_smart_ptr(caller.id())?;
    let signed_msg_bytes = to_vec(signed_messages).map_err(|_| StateError::Serialization)?;
    let signed_messages = to_smart_ptr(&signed_msg_bytes)?;
    Ok(unsafe { _batch_verify_ed25519(caller, signed_messages) })
}
