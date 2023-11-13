use wasmlanche_sdk::{program::Program, public, types::Bytes32};
use wasmlanche_sdk::host::verify_ed25519;
use ed25519_dalek::Signature;

// import custom types from types.rs
mod types;
use types::{EDSignatureBytes, BatchSigningKeys};

/// Runs multiple ED25519 signature verifications in Wasm. 
/// Returns true if every signing_key in [signing_keys] verifies [signature_bytes] against [message].
/// Returns false if otherwise.
#[public]
pub fn verify_ed_in_wasm(_: Program, signing_keys: BatchSigningKeys, signature_bytes: EDSignatureBytes, message: Bytes32) -> bool {
    // first signing key
    let mut result = true;
    let signature: Signature = Signature::from_bytes(signature_bytes.as_bytes());
    // iterate through batch signing keys
    for signing_key in signing_keys.0.iter() {
        // println!("Signature bytes: {:?}", signature_bytes);
        match signing_key.as_key().verify(message.as_bytes(), &signature) {
            Ok(_) => {},
            Err(_) => {result = false},
        }
    }
    result
}

#[public]
pub fn verify_ed_multiple_host_func(program: Program) -> i32{
     verify_ed25519(&program) 
}

// #[public]
// pub fn verify_ed_batch_in_host(program: Program) {

// }