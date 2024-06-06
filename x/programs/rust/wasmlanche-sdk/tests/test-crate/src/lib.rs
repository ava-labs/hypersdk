#![no_std]

use wasmlanche_sdk::{public, state_keys, Context};

#[state_keys]
pub enum StateKeys {}

#[public]
pub fn always_true(_: Context<StateKeys>) -> bool {
    true
}

#[public]
pub fn combine_last_bit_of_each_id_byte(context: Context<StateKeys>) -> u32 {
    context
        .program
        .account()
        .into_iter()
        .map(|byte| byte as u32)
        .fold(0, |acc, byte| (acc << 1) + (byte & 1))
}
