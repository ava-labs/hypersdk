use ed25519_dalek::VerifyingKey;
use wasmlanche_sdk::types::Bytes32;

// We create these custom wrapper types to allow us to pass them
// across the WASM boundary.

// EDVerifyingKey is a wrapper around ed25519_dalek::VerifyingKey.
// this represents an ED25519 public key
pub struct EDVerifyingKey(ed25519_dalek::VerifyingKey);

// EDSignatureBytes represents the bytes of an ed25519 signature.
pub struct EDSignatureBytes([u8; EDSignatureBytes::LEN]);

// BatchPubKeys represents a batch of EDVerifyingKeys.
// Currently, we need to hardcode the length of the batch(due to limitations of the SDK).
// Once we have dynamic args, we can marshal the length in go and unmarshal in the macro.
pub struct BatchPubKeys(pub Vec<EDVerifyingKey>);

impl EDVerifyingKey {
    const LEN: usize = ed25519_dalek::PUBLIC_KEY_LENGTH;
    pub fn as_key(&self) -> &ed25519_dalek::VerifyingKey {
        &self.0
    }
}

impl EDSignatureBytes {
    const LEN: usize = ed25519_dalek::SIGNATURE_LENGTH;
    pub fn as_bytes(&self) -> &[u8; Self::LEN] {
        &self.0
    }
}

impl BatchPubKeys {
    // hardcoded length of batch
    const TEMP_LEN: usize = 5;
    pub fn push(&mut self, pub_key: EDVerifyingKey) {
        self.0.push(pub_key);
    }
    pub fn len(&self) -> usize {
        self.0.len()
    }
}

impl From<i64> for EDVerifyingKey {
    fn from(value: i64) -> Self {
        let bytes = Bytes32::from(value);
        Self(
            VerifyingKey::from_bytes(
            bytes.as_bytes()
                .try_into()
                .expect("invalid bytes")
            ).expect("invalid bytes"),
        )
    }
}

impl From<i64> for BatchPubKeys {
    fn from(value: i64) -> Self {
        let mut pub_keys: BatchPubKeys = BatchPubKeys(Vec::new());
        // in the meantime we need to hardcode this
        // once we have dynamic args we can marshal the length in go
        // and unmarshal in the macro
        for i in 0..BatchPubKeys::TEMP_LEN {
            // 32 bytes per key, update ptr & grab next key
            let ptr: i64 = value + (i * EDVerifyingKey::LEN) as i64;
            pub_keys.push(EDVerifyingKey::from(ptr));
        }

        pub_keys
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
