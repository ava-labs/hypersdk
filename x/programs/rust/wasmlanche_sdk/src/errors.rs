use thiserror::Error;

#[derive(Clone, Error, Debug)]
pub enum StateError {
    #[error("an unclassified error has occurred: {0}")]
    Other(String),

    #[error("invalid byte format.")]
    InvalidBytes,

    #[error("invalid byte length: {0}")]
    InvalidByteLength(usize),

    #[error("invalid tag: {0}")]
    InvalidTag(u8),

    #[error("failed to write to host storage")]
    Write,

    #[error("failed to read from host storage")]
    Read,

    #[error("failed to serialize bytes")]
    Serialization,
}
