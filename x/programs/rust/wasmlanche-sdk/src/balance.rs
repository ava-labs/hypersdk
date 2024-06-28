use crate::{memory::HostPtr, types::Address, ExternalCallError};

/// Gets the balance for the specified address
/// # Panics
/// Panics if there was an issue deserializing the balance
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

/// Transfer currency from the calling program to the passed address
/// # Panics
/// Panics if there was an issue deserializing the result
pub fn send(to: Address, amount: u64) -> Result<(), ExternalCallError> {
    #[link(wasm_import_module = "balance")]
    extern "C" {
        #[link_name = "send"]
        fn send_value(ptr: *const u8, len: usize) -> HostPtr;
    }
    let ptr = borsh::to_vec(&(to, amount)).expect("failed to serialize args");

    let bytes = unsafe { send_value(ptr.as_ptr(), ptr.len()) };

    borsh::from_slice(&bytes).expect("failed to deserialize the result")
}
