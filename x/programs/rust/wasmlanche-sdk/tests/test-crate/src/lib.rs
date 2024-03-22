#![no_std]

use wasmlanche_sdk::{public, Context};

#[public]
pub fn noop(_: Context) -> i64 {
    true as i64
}

#[public]
pub fn return_id(context: Context) -> u32 {
    context
        .program
        .id()
        .iter()
        .map(|byte| *byte as u32)
        .reduce(|acc, byte| (acc << 1) + (byte & 1))
        .unwrap()
}
