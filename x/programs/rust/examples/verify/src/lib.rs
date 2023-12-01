use ed25519_dalek::{Signature, Verifier, VerifyingKey};
use wasmlanche_sdk::host::verify_ed25519;
use wasmlanche_sdk::{program::Program, public};

// import custom types from types.rs
mod types;
use types::SignedMessage;


/// Runs multiple ED25519 signature verifications in Wasm.
/// Returns the number of successfull verifications of [signature_bytes] 
/// against [message] using the public keys in [pub_keys].
#[public]
pub fn verify_ed_in_wasm(
    _: Program,
    signed_messages: Vec<SignedMessage>,
) -> i32 {
    let mut success_count = 0;
    for signed_message in signed_messages.iter() {
        let signature: Signature = Signature::from_bytes(&signed_message.signature);
        let pub_key = VerifyingKey::from_bytes(&signed_message.public_key).expect("invalid bytes");
        match pub_key.verify(&signed_message.message, &signature) {
            Ok(_) => {success_count += 1;}
            Err(_) => {},
        }
    }
    success_count
}

/// Runs multiple ED25519 signature verifications in the host.
/// Returns the number of successfull verifications of [signature_bytes] 
/// against [message] using the public keys in [pub_keys].
#[public]
pub fn verify_ed_multiple_host_func(
    program: Program,
    signed_messages: Vec<SignedMessage>,
) -> i32 {
    let mut success_count = 0;
    for signed_message in signed_messages.iter() {
        let signature: Signature = Signature::from_bytes(&signed_message.signature);
        let pub_key = VerifyingKey::from_bytes(&signed_message.public_key).expect("invalid bytes");
        match verify_ed25519(&program, &signed_message.message, &signature, &pub_key) {
            1 => {success_count += 1;}
            _ => {},
        }
    }
    success_count
}