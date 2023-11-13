use wasmlanche_sdk::{program::Program, public, types::Bytes32};
use wasmlanche_sdk::host::verify_ed25519;
use ed25519_dalek::{Signature, Verifier};
// import custom types from types.rs
mod types;
use types::{EDSignatureBytes, BatchPubKeys};

/// Runs multiple ED25519 signature verifications in Wasm. 
/// Returns true if every public key in [pub_keys] verifies [signature_bytes] against [message].
/// Returns false if otherwise.
#[public]
pub fn verify_ed_in_wasm(_: Program, pub_keys: BatchPubKeys, signature_bytes: EDSignatureBytes, message: Bytes32) -> bool {
    let mut result = true;
    let signature: Signature = Signature::from_bytes(signature_bytes.as_bytes());
    // iterate through batch pub keys
    for pub_key in pub_keys.0.iter() {
        match pub_key.as_key().verify(message.as_bytes(), &signature) {
            Ok(_) => {},
            Err(_) => {result = false},
        }
    }
    result
}

#[public]
pub fn verify_ed_multiple_host_func(program: Program, pub_keys: BatchPubKeys, signature_bytes: EDSignatureBytes, message: Bytes32) -> bool {
    let mut result = true;
    let signature: Signature = Signature::from_bytes(signature_bytes.as_bytes());
    // iterate through batch pub keys
    for pub_key in pub_keys.0.iter() {
        match verify_ed25519(&program, message, &signature, pub_key.as_key()) {
            1 => {},
            _ => {result = false},
        }
    }
    result
}

// #[public]
// pub fn verify_ed_batch_in_host(program: Program) {

// }