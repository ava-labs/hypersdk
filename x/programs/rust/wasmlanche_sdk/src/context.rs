use crate::{
    program::Program,
    types::Address,
};
use borsh::{BorshDeserialize, BorshSerialize};

#[derive(Clone, Copy, BorshDeserialize, BorshSerialize)]
pub struct Context {
   pub program: Program,
   pub actor: Address,
   pub originating_actor: Address,
}