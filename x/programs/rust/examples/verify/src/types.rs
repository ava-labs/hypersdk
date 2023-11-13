use ed25519_dalek::{Signature, SigningKey, Signer, VerifyingKey};
use wasmlanche_sdk::{program::Program, public, types::Bytes32};

// Custom Args
// verifying key = ed25519 pub key
pub struct EDVerifyingKey(ed25519_dalek::VerifyingKey);
pub struct EDSignatureBytes([u8; 64]);
pub struct BatchPubKeys(pub Vec<EDVerifyingKey>);

impl From<i64> for EDVerifyingKey {
    fn from(value: i64) -> Self {
        let bytes = Bytes32::from(value);
        let mut verifying_key_bytes: [u8; 32] = [0; 32];
        verifying_key_bytes.copy_from_slice(bytes.as_bytes());
        // TODO: is unwrap ok here
        Self(VerifyingKey::from_bytes(&verifying_key_bytes).unwrap())
    }
}

impl BatchPubKeys {
    const TEMP_LEN: usize = 5;
    pub fn push(&mut self, pub_key: EDVerifyingKey) {
        self.0.push(pub_key);
    }
    pub fn len(&self) -> usize {
        self.0.len()
    }
}

impl From<i64> for BatchPubKeys {
    fn from(value: i64) -> Self {
        let mut pub_keys : BatchPubKeys = BatchPubKeys(Vec::new());
        // in the meantime we need to hardcode this
        // once we have dynamic args we can marshal the length in go
        // and unmarshal in the macro
        for i in 0..BatchPubKeys::TEMP_LEN {
            let ptr : i64 = value + (i as i64) * 32;
            pub_keys.push(EDVerifyingKey::from(ptr));
        }

        if pub_keys.len() != 5 {
            panic!("Expected 5 initialized values");
        }

        pub_keys
    }
}

impl EDVerifyingKey {
    pub fn as_key(&self) -> &ed25519_dalek::VerifyingKey {
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