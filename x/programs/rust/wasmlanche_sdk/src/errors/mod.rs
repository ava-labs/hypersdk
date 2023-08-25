use thiserror::Error;

#[derive(Clone, Error, Debug)]
pub enum StorageError {
    #[error("an unclassified error has occurred: {0}")]
    Other(String),

    #[error("Invalid byte format.")]
    InvalidBytes,

    #[error("Invalid Byte Length: {0}")]
    InvalidByteLength(usize),

    #[error("Invalid Tag: {0}")]
    InvalidTag(u8),

    #[error("Error Storing Bytes In The Host")]
    HostStoreError,

    #[error("Error Retrieving Bytes In The Host")]
    HostRetrieveError,
}
