use crate::Error;
use borsh::BorshSerialize;
use std::ops::Deref;

#[macro_export]
macro_rules! params {
    ($first:expr $(,$rest:expr)* $(,)*) => {
        std::iter::once($crate::params::serialize_param($first))
            $(
                .chain(Some($crate::params::serialize_param($rest)))
            )*
            .collect::<Result<$crate::Params, $crate::Error>>()
    }
}

/// A [borsh] serialized parameter.
pub struct Param(Vec<u8>);

/// A collection of [borsh] serialized parameters.
pub struct Params(Vec<u8>);

impl Deref for Params {
    type Target = Vec<u8>;

    fn deref(&self) -> &Self::Target {
        &self.0
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

#[cfg(test)]
mod tests {
    use crate::serialize_param;
    use borsh::{BorshDeserialize, BorshSerialize};

    #[derive(BorshSerialize, BorshDeserialize, Debug, PartialEq)]
    struct Data(usize);

    #[test]
    fn valid_serialized_host_ptr() {
        let data = Data(12345);
        let param = serialize_param(&data).unwrap();
        let ret_data = check_components(&param.0);
        assert_eq!(data, ret_data);

        let ser_params = serialize_param(&param.0).unwrap().0;
        let params = params!(&param.0).unwrap();

        assert_eq!(params.0, ser_params);
    }

    fn check_components<T: BorshDeserialize>(bytes: &[u8]) -> T {
        let len_bytes = bytes.get(0..4).unwrap();
        let len_comp = u32::from_be_bytes(len_bytes.try_into().unwrap());
        assert_eq!(bytes.len(), 4 + len_comp as usize, "invalid len pointer");
        let data_bytes = bytes.get(4..).unwrap();
        borsh::from_slice(data_bytes).unwrap()
    }
}
