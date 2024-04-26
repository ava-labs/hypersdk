use crate::{
    memory::{to_host_ptr, HostPtr},
    state::Error as StateError,
    Error,
};
use borsh::BorshSerialize;

#[macro_export]
macro_rules! params {
    ($first:expr $(,$rest:expr)* $(,)*) => {
        std::iter::once(wasmlanche_sdk::params::serialize_param($first))
            $(
                .chain(Some(wasmlanche_sdk::params::serialize_param($rest)))
            )*
            .collect::<Result<wasmlanche_sdk::Params, wasmlanche_sdk::Error>>()
    }
}

/// A [borsh] serialized parameter.
pub struct Param(Vec<u8>);

/// A collection of [borsh] serialized parameters.
pub struct Params(Vec<u8>);

impl Params {
    pub(crate) fn into_host_ptr(self) -> Result<HostPtr, StateError> {
        to_host_ptr(&self.0)
    }
}

impl FromIterator<Param> for Params {
    fn from_iter<T: IntoIterator<Item = Param>>(iter: T) -> Self {
        Params(iter.into_iter().flat_map(|param| param.0).collect())
    }
}

/// Serializes the parameter into a byte vector.
/// # Errors
/// Will return an error if the parameter cannot be serialized.
/// # Panics
/// Will panic if the byte-length of the serialized parameter exceeds [`u32::MAX`].
pub fn serialize_param<T>(param: &T) -> Result<Param, Error>
where
    T: BorshSerialize,
{
    let bytes = borsh::to_vec(param).map_err(Error::Param)?;
    let bytes = u32::try_from(bytes.len())
        .expect("pointer out range")
        .to_be_bytes()
        .into_iter()
        .chain(bytes)
        .collect();

    Ok(Param(bytes))
}
