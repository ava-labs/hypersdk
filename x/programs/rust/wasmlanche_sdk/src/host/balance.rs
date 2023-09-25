// The balance module exposes helpers for transferring tokens between accounts.
#[link(wasm_import_module = "balance")]
extern "C" {
    #[link_name = "get"]
    fn _get(
        caller_id: i64,
        key_ptr: *const u8,
        key_len: usize,
        value_ptr: *const u8,
        value_len: usize,
    ) -> i32;

    #[link_name = "get_bytes_len"]
    fn _get_bytes_len(caller_id: i64, key_ptr: *const u8, key_len: usize) -> i32;

    #[link_name = "get_bytes"]
    fn _get_bytes(caller_id: i64, key_ptr: *const u8, key_len: usize, val_len: i32) -> i32;
}