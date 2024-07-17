use base64::{engine::general_purpose::STANDARD as b64, Engine};
use borsh::BorshDeserialize;
use serde::{Deserialize, Deserializer, Serialize, Serializer};

use crate::{Id, TestContext};

pub(crate) fn base64_encode<S>(struc: &TestContext, serializer: S) -> Result<S::Ok, S::Error>
where
    S: Serializer,
{
    let bytes = serde_json::to_vec(struc).unwrap();
    serializer.serialize_str(&b64.encode(bytes))
}

pub(crate) fn base64_decode<'de, D>(deserializer: D) -> Result<Vec<u8>, D::Error>
where
    D: Deserializer<'de>,
{
    <&str>::deserialize(deserializer).and_then(|s| {
        b64.decode(s)
            .map_err(|err| serde::de::Error::custom(err.to_string()))
    })
}

pub(crate) fn id_from_usize<'de, D>(deserializer: D) -> Result<Id, D::Error>
where
    D: Deserializer<'de>,
{
    <usize as Deserialize>::deserialize(deserializer).map(Id::from)
}
