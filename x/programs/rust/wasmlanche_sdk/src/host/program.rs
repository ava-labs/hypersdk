//! The `program` module provides functions for calling other programs.
use crate::program::Program;
use crate::memory::to_ptr_arg;

#[link(wasm_import_module = "program")]
extern "C" {
    #[link_name = "call_program"]
    fn _call_program(
        caller_id: i64,
        target_id: i64,
        max_units: i64,
        function: i64,
        args_ptr: i64,
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
    
    let caller_id = caller.id();
    let caller = to_ptr_arg(&caller_id).unwrap();
    let target_id = target.id();
    let target = to_ptr_arg(&target_id).unwrap();
    let function = to_ptr_arg(function_name.as_bytes()).unwrap();
    let args = to_ptr_arg(args).unwrap();

    unsafe {
        _call_program(
            caller,
            target,
            max_units,
            function,
            args,
        )
    }
}
