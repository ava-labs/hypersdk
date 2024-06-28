use crate::{memory::HostPtr, types::Address, ExternalCallError};

/// Gets the remaining fuel available to this program
/// # Panics
/// Panics if there was an issue deserializing the remaining fuel
pub fn get_balance(account: Address) -> u64 {
    #[link(wasm_import_module = "balance")]
    extern "C" {
        #[link_name = "get"]
        fn get(ptr: *const u8, len: usize) -> HostPtr;
    }
    let ptr = borsh::to_vec(&account).expect("failed to serialize args");
    let bytes = unsafe { get(ptr.as_ptr(), ptr.len()) };

    borsh::from_slice(&bytes).expect("failed to deserialize the balance")
}

pub fn send(to: Address, amount: u64) -> Result<(), ExternalCallError> {
    #[link(wasm_import_module = "balance")]
    extern "C" {
        #[link_name = "send"]
        fn send_value(ptr: *const u8, len: usize) -> HostPtr;
    }
    let ptr = borsh::to_vec(&(to, amount)).expect("failed to serialize args");

    let bytes = unsafe { send_value(ptr.as_ptr(), ptr.len()) };

    borsh::from_slice(&bytes).expect("failed to deserialize the remaining fuel")
}
