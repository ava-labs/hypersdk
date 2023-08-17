use thiserror::Error;

#[derive(Clone, Error, Debug)]
pub enum StorageError {
    #[error("an unclassified error has occurred: {0}")]
    Other(String),

    #[error("Invalid Bytes: {0}")]
    InvalidBytes(String),

    #[error("Invalid Tag: {0}")]
    InvalidTag(String),

    #[error("Host Error: {0}")]
    HostError(String),
}
