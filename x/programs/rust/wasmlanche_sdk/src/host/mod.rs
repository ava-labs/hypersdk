use crate::store::ProgramContext;

// The map module contains functionality for storing and retrieving key-value pairs.
#[link(wasm_import_module = "map")]
extern "C" {
    #[link_name = "init_program"]
    fn _init_program() -> i64;

    #[link_name = "store_bytes"]
    fn _store_bytes(
        contractId: u64,
        key_ptr: *const u8,
        key_len: usize,
        value_ptr: *const u8,
        value_len: usize,
    ) -> i32;

    #[link_name = "get_bytes_len"]
    fn _get_bytes_len(contract_id: u64, key_ptr: *const u8, key_len: usize) -> i32;

    #[link_name = "get_bytes"]
    fn _get_bytes(contract_id: u64, key_ptr: *const u8, key_len: usize, val_len: i32) -> i32;
}

// The program module contains functionality for invoking external programs.
#[link(wasm_import_module = "program")]
extern "C" {
    #[link_name = "invoke_program"]
    fn _invoke_program(
        contract_id: u64,
        call_contract_id: u64,
        method_name_ptr: *const u8,
        method_name_len: usize,
        args_ptr: *const u8,
        args_len: usize,
    ) -> i64;
}

/* wrappers for unsafe imported functions ----- */
/// Returns the map_id or None if there was an error
pub fn init_program_storage() -> ProgramContext {
    unsafe { ProgramContext::from(_init_program()) }
}

pub fn store_bytes(
    ctx: &ProgramContext,
    key_ptr: *const u8,
    key_len: usize,
    value_ptr: *const u8,
    value_len: usize,
) -> i32 {
    unsafe { _store_bytes(ctx.program_id, key_ptr, key_len, value_ptr, value_len) }
}

pub fn get_bytes_len(ctx: &ProgramContext, key_ptr: *const u8, key_len: usize) -> i32 {
    unsafe { _get_bytes_len(ctx.program_id, key_ptr, key_len) }
}

pub fn get_bytes(ctx: &ProgramContext, key_ptr: *const u8, key_len: usize, val_len: i32) -> i32 {
    unsafe { _get_bytes(ctx.program_id, key_ptr, key_len, val_len) }
}

pub fn host_program_invoke(
    ctx: &ProgramContext,
    call_ctx: &ProgramContext,
    method_name: &str,
    args: &[u8],
) -> i64 {
    let method_name_bytes = method_name.as_bytes();
    unsafe {
        _invoke_program(
            ctx.program_id,
            call_ctx.program_id,
            method_name_bytes.as_ptr(),
            method_name_bytes.len(),
            args.as_ptr(),
            args.len(),
        )
    }
}

/* memory functions ------------------------------------------- */
// https://radu-matei.com/blog/practical-guide-to-wasm-memory/

/// Allocate memory into the module's linear memory
/// and return the offset to the start of the block.
#[no_mangle]
pub fn alloc(len: usize) -> *mut u8 {
    // create a new mutable buffer with capacity `len`
    let mut buf = Vec::with_capacity(len);
    // take a mutable pointer to the buffer
    let ptr = buf.as_mut_ptr();
    // take ownership of the memory block and
    // ensure that its destructor is not
    // called when the object goes out of scope
    // at the end of the function
    std::mem::forget(buf);
    // return the pointer so the runtime
    // can write data at this offset
    ptr
}

#[no_mangle]
pub unsafe fn dealloc(ptr: *mut u8, capacity: usize) {
    // always deallocate the full capacity, initialize vs uninitialized memory is irrelevant here
    let data = Vec::from_raw_parts(ptr, capacity, capacity);
    std::mem::drop(data);
}
