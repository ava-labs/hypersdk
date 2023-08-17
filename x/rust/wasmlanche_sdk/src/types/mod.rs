use crate::program::ProgramValue;
use crate::store::ProgramContext;

/// A struct that enforces a fixed length of 32 bytes which represents an address.
#[derive(Clone, Copy, PartialEq, Eq)]
pub struct Address {
    bytes: [u8; Self::LEN],
}

impl Address {
    pub const LEN: usize = 32;
    // Constructor function for Address
    pub fn new(bytes: [u8; Self::LEN]) -> Self {
        Self { bytes }
    }
    pub fn as_bytes(&self) -> &[u8] {
        &self.bytes
    }
}

impl From<String> for ProgramValue {
    fn from(value: String) -> Self {
        ProgramValue::StringObject(value)
    }
}

impl From<&str> for ProgramValue {
    fn from(value: &str) -> Self {
        ProgramValue::StringObject(String::from(value))
    }
}

impl From<i64> for ProgramValue {
    fn from(value: i64) -> Self {
        ProgramValue::IntObject(value)
    }
}

impl From<Address> for ProgramValue {
    fn from(value: Address) -> Self {
        ProgramValue::AddressObject(value.clone())
    }
}

impl From<i64> for Address {
    fn from(value: i64) -> Self {
        let bytes: [u8; Self::LEN] = unsafe {
            // We want to copy the bytes here, since [value] represents a ptr created by the host
            std::slice::from_raw_parts(value as *const u8, Self::LEN)
                .try_into()
                .unwrap()
        };
        Self { bytes }
    }
}

impl From<ProgramValue> for i64 {
    fn from(value: ProgramValue) -> Self {
        match value {
            ProgramValue::IntObject(i) => i,
            _ => panic!("Cannot conver to i64"),
        }
    }
}

impl From<ProgramValue> for ProgramContext {
    fn from(value: ProgramValue) -> Self {
        match value {
            ProgramValue::ProgramObject(i) => i,
            _ => panic!("Cannot conver to ProgramContext"),
        }
    }
}
