use ed25519_dalek::{PUBLIC_KEY_LENGTH, SIGNATURE_LENGTH};
use borsh::{BorshDeserialize, BorshSerialize};

#[derive(BorshDeserialize, BorshSerialize)]
pub struct SignedMessage {
    pub message: [u8; 32],
    pub signature: [u8; SIGNATURE_LENGTH],
    pub public_key: [u8; PUBLIC_KEY_LENGTH],
}
