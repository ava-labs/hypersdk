use ed25519_dalek::{Signature, SigningKey, Signer};
use wasmlanche_sdk::{program::Program, public, types::Bytes32};

// Custom Args
pub struct EDSigningKey(ed25519_dalek::SigningKey);
pub struct EDSignatureBytes([u8; 64]);
pub struct BatchSigningKeys(pub Vec<EDSigningKey>);

impl From<i64> for EDSigningKey {
    fn from(value: i64) -> Self {
        let bytes = Bytes32::from(value);
        let mut signing_key_bytes: [u8; 32] = [0; 32];
        signing_key_bytes.copy_from_slice(bytes.as_bytes());
        Self(SigningKey::from_bytes(&signing_key_bytes))
    }
}

impl BatchSigningKeys {
    const TEMP_LEN: usize = 5;
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
        // in the meantime we need to hardcode this
        // once we have dynamic args we can marshal the length in go
        // and unmarshal in the macro
        for i in 0..BatchSigningKeys::TEMP_LEN {
            let ptr : i64 = value + (i as i64) * 32;
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

impl EDSignatureBytes {
    const LEN: usize = 64;
    pub fn as_bytes(&self) -> &[u8; 64] {
        &self.0
    }
}

impl From<i64> for EDSignatureBytes {
    fn from(value: i64) -> Self {
        // Temporary workaround for now
        let bytes: [u8; Self::LEN] = unsafe {
            // We want to copy the bytes here, since [value] represents a ptr created by the host
            std::slice::from_raw_parts(value as *const u8, Self::LEN)
                .try_into()
                .unwrap()
        };
        Self(bytes)
    }
}