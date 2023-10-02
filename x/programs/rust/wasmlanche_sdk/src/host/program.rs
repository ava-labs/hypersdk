//! The `program` module provides functions for calling other programs.
use crate::program::Program;

#[link(wasm_import_module = "program")]
extern "C" {
    #[link_name = "call_program"]
    fn _call_program(
        caller_id: i64,
        target_id: i64,
        max_units: i64,
        function_ptr: *const u8,
        function_len: usize,
        args_ptr: *const u8,
        args_len: usize,
    ) -> i64;
}

/// Calls another program `target` and returns the result.
#[must_use]
pub(crate) fn call(
    caller: &Program,
    target: &Program,
    max_units: i64,
    function_name: &str,
    args: &[u8],
) -> i64 {
    let function_bytes = function_name.as_bytes();
    unsafe {
        _call_program(
            caller.id(),
            target.id(),
            max_units,
            function_bytes.as_ptr(),
            function_bytes.len(),
            args.as_ptr(),
            args.len(),
        )
    }
}
