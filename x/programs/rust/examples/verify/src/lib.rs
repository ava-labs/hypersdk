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


pub struct BatchSigningKeys(Vec<EDSigningKey>);

impl BatchSigningKeys {
    pub fn push(&mut self, signing_key: EDSigningKey) {
        self.0.push(signing_key);
    }
    pub fn len(&self) -> usize {
        self.0.len()
    }
}

impl From<i64> for BatchSigningKeys {
    fn from(value: i64) -> Self {
        let mut signing_keys : BatchSigningKeys = BatchSigningKeys(Vec::new());

        for i in 0..5 {
            let ptr : i64 = value + i * 32;
            signing_keys.push(EDSigningKey::from(ptr));
        }

        if signing_keys.len() != 5 {
            panic!("Expected 5 initialized values");
        }

        signing_keys
    }
}

impl EDSigningKey {
    pub fn as_key(&self) -> &ed25519_dalek::SigningKey {
        &self.0
    }
}

/// Verifies the ed25519 signature in wasm. 
#[public]
pub fn verify_ed_in_wasm(_: Program, signing_keys: BatchSigningKeys, message: Bytes32) {
    // first signing key
    let signing_key = signing_keys.0.get(0).unwrap();
    let signature: Signature = signing_key.as_key().sign(message.as_bytes());
    // get the bytes of the signature
    let signature_bytes = signature.to_bytes();

    // iterate through batch signing keys
    for signing_key in signing_keys.0.iter() {
        // println!("Signature bytes: {:?}", signature_bytes);
        match signing_key.as_key().verify(message.as_bytes(), &signature) {
            Ok(_) => println!("Signature verified!"),
            Err(_) => println!("Signature not verified!"),
        }
    }
}

// #[public]
// pub fn verify_ed_multiple_host_func(program: Program)  {

// }

// #[public]
// pub fn verify_ed_batch_in_host(program: Program) {

// }