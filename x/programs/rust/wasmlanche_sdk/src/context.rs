use crate::{
    program::Program,
};
use borsh::{BorshDeserialize, BorshSerialize};

#[derive(Clone, Copy, BorshDeserialize, BorshSerialize)]
pub struct Context {
   pub program: Program,
}