use ed25519_dalek::{Signature, SigningKey, Signer};
 
use wasmlanche_sdk::{program::Program, public, types::Bytes32};


// Custom Args
pub struct EDSigningKey(ed25519_dalek::SigningKey);

impl From<i64> for EDSigningKey {
    fn from(value: i64) -> Self {
        let bytes = Bytes32::from(value);
        let mut signing_key_bytes: [u8; 32] = [0; 32];
        signing_key_bytes.copy_from_slice(bytes.as_bytes());
        Self(SigningKey::from_bytes(&signing_key_bytes))
    }
}

impl EDSigningKey {
    pub fn as_key(&self) -> &ed25519_dalek::SigningKey {
        &self.0
    }
}


/// Verifies the ed25519 signature in wasm. 
#[public]
pub fn verify_ed_in_wasm(_: Program, signing_key: EDSigningKey, message: Bytes32) {
    let signature: Signature = signing_key.as_key().sign(message.as_bytes());

    match signing_key.as_key().verify(message.as_bytes(), &signature) {
        Ok(_) => println!("Signature verified!"),
        Err(_) => println!("Signature not verified!"),
    }
}

// #[public]
// pub fn verify_ed_multiple_host_func(program: Program)  {

// }

// #[public]
// pub fn verify_ed_batch_in_host(program: Program) {

// }