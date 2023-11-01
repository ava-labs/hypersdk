extern crate ed25519_dalek;

use ed25519_dalek::Verifier;
use ed25519_dalek::Signature;
use wasmlanche_sdk::{program::Program, public, state_keys, types::Address};

/// Initializes the program
#[public]
pub fn init(program: Program) -> bool {
    true
}

/// Verifies the ed25519 signature in wasm. 
#[public]
pub fn verify_ed_in_wasm(program: Program) {
    let public_key_bytes = [0u8; 32]; // Replace with your public key bytes
    let signature_bytes = [0u8; 64]; // Replace with your signature bytes

    // Parse the public key and signature
    let public_key = ed25519_dalek::PublicKey::from_bytes(&public_key_bytes).unwrap();
    let signature = ed25519_dalek::Signature::from_bytes(&signature_bytes).unwrap();

    // Replace this with your actual message
    let message = "Hello, world!";

    let verifier = public_key.into();

    if verifier.verify(message.as_bytes(), &signature).is_ok() {
        println!("Signature is valid");
    } else {
        println!("Signature is invalid");
    }
}

// #[public]
// pub fn verify_ed_multiple_host_func(program: Program)  {

// }

// #[public]
// pub fn verify_ed_batch_in_host(program: Program) {

// }