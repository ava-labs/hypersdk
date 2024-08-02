use wasmlanche_sdk::Address as SdkAddress;

pub use crate::{Address, Bytes};

impl From<SdkAddress> for Address {
    fn from(value: SdkAddress) -> Self {
        Address {
            address: value.as_bytes().try_into().expect("Invalid address format"),
        }
    }
}

impl Bytes {
    pub fn get_slice(&self) -> &[u8] {
        unsafe { std::slice::from_raw_parts(self.data, self.length as usize) }
    }
}
