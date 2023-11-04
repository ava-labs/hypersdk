use std::{array::TryFromSliceError, num::TryFromIntError};
use thiserror::Error;

#[derive(Clone, Error, Debug)]
pub enum StateError {
    #[error("an unclassified error has occurred: {0}")]
    Other(String),

    #[error("invalid byte format: {0}")]
    InvalidBytes(String),

    #[error("invalid byte length: {0}")]
    InvalidByteLength(usize),

    #[error("invalid tag: {0}")]
    InvalidTag(u8),

    #[error("failed to write to host storage")]
    Write,

    #[error("failed to read from host storage")]
    Read,

    #[error("failed to delete bytes")]
    Delete,

    #[error("failed to serialize bytes")]
    Serialization,

    #[error("underflow")]
    Underflow,

    #[error("try from slice conversion error {0}")]
    TryFromSlice(#[from] TryFromSliceError),

    #[error("try from slice conversion error {0}")]
    TryFromInt(#[from] TryFromIntError),

    #[error("null pointer")]
    NullPointer,
}
