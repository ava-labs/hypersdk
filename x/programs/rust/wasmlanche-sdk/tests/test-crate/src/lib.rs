#![no_std]

use wasmlanche_sdk::{public, Context};

#[public]
pub fn always_true<'a>(_: &'a mut Context) -> bool {
    true
}

#[public]
pub fn combine_last_bit_of_each_id_byte<'a>(context: &'a mut Context) -> u32 {
    context
        .program()
        .account()
        .into_iter()
        .map(|byte| byte as u32)
        .fold(0, |acc, byte| (acc << 1) + (byte & 1))
}
