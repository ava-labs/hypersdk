use crate::{
    program::Program,
    types::Address,
};
use borsh::{BorshDeserialize, BorshSerialize};

#[derive(Clone, Copy, BorshDeserialize, BorshSerialize)]
pub struct Context {
   pub program: Program,
   pub gas_remaining: u64,
  // pub actor: Address,
}