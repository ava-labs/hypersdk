use ed25519_dalek::{Signature, Verifier};
use wasmlanche_sdk::host::verify_ed25519;
use wasmlanche_sdk::{program::Program, public, types::Bytes32};
// import custom types from types.rs
mod types;
use types::{BatchPubKeys, EDSignatureBytes};

/// Runs multiple ED25519 signature verifications in Wasm.
/// Returns the number of successfull verifications of [signature_bytes] 
/// against [message] using the public keys in [pub_keys].
#[public]
pub fn verify_ed_in_wasm(
    _: Program,
    pub_keys: BatchPubKeys,
    signature_bytes: EDSignatureBytes,
    message: Bytes32,
) -> i32 {
    let mut success_count = 0;
    let signature: Signature = Signature::from_bytes(signature_bytes.as_bytes());
    // iterate through batch pub keys
    for pub_key in pub_keys.0.iter() {
        match pub_key.as_key().verify(message.as_bytes(), &signature) {
            Ok(_) => {success_count += 1;}
            Err(_) => {}
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
    pub_keys: BatchPubKeys,
    signature_bytes: EDSignatureBytes,
    message: Bytes32,
) -> i32 {
    let mut success_count = 0;
    let signature: Signature = Signature::from_bytes(signature_bytes.as_bytes());
    // iterate through batch pub keys
    for pub_key in pub_keys.0.iter() {
        match verify_ed25519(&program, message, &signature, pub_key.as_key()) {
            1 => {success_count += 1;}
            _ => {},
        }
    }
    success_count
}

// #[public]
// pub fn verify_ed_batch_in_host(program: Program) {

// }
