//! The `program` module provides functions for calling other programs.
use crate::errors::StateError;
use crate::memory::{to_smart_ptr, SmartPtr};
use crate::program::Program;

#[link(wasm_import_module = "program")]
extern "C" {
    #[link_name = "call_program"]
    fn _call_program(target_id: i64, function: i64, args_ptr: i64, max_units: i64) -> i64;
}

#[link(wasm_import_module = "program")]
extern "C" {
    #[link_name = "enter_program"]
    fn _enter_program(target_id: i64, function: i64) -> i64;
}


/// Calls a program `target` and returns the result.
pub(crate) fn call(
    target: &Program,
    function_name: &str,
    args: &[u8],
    max_units: i64,
) -> Result<i64, StateError> {
    let target = to_smart_ptr(target.id())?;
    let function = to_smart_ptr(function_name.as_bytes())?;
    let args = to_smart_ptr(args)?;

    Ok(unsafe { _call_program(target, function, args, max_units) })
}

pub fn enter_program(
    target: SmartPtr,
    function_name: &str
) -> Result<bool, StateError> {
    let function = to_smart_ptr(function_name.as_bytes())?;
    Ok(unsafe { _enter_program(target, function) == 1 })
}