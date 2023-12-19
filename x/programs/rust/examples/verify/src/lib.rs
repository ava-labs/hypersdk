use ed25519_dalek::{Signature, Verifier, VerifyingKey};
use wasmlanche_sdk::host::{batch_verify_ed25519, verify_ed25519, SignedMessage};
use wasmlanche_sdk::{program::Program, public};

/// Runs multiple ED25519 signature verifications in Wasm.
/// Returns the number of successfull verifications of [signature_bytes]
/// against [message] using the public keys in [pub_keys].
#[public]
pub fn verify_ed_in_wasm(_: Program, signed_messages: Vec<SignedMessage>) -> i32 {
    let mut success_count = 0;
    for signed_message in signed_messages.iter() {
        let signature: Signature = Signature::from_bytes(&signed_message.signature);
        let pub_key = VerifyingKey::from_bytes(&signed_message.public_key).expect("invalid bytes");
        if pub_key.verify(&signed_message.message, &signature).is_ok() {
            success_count += 1;
        }
    }
    success_count
}

/// Runs multiple ED25519 signature verifications in the host.
/// Returns the number of successfull verifications of [signature_bytes]
/// against [message] using the public keys in [pub_keys].
#[public]
pub fn verify_ed_multiple_host_func(program: Program, signed_messages: Vec<SignedMessage>) -> i32 {
    let mut success_count = 0;
    for signed_message in signed_messages.iter() {
        if verify_ed25519(&program, signed_message).unwrap() == 1 {
            success_count += 1;
        }
    }
    success_count
}

/// Runs multiple ED25519 signature verifications in the host with just one call.
#[public]
pub fn verify_ed_batch_host_func(program: Program, signed_messages: Vec<SignedMessage>) -> i32 {
    batch_verify_ed25519(&program, &signed_messages).unwrap()
}
