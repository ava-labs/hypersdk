use serde::Serialize;
use base64::{engine::general_purpose::STANDARD as b64, Engine};
use wasmlanche_sdk::Address;

use crate::{Id, Key, SimulatorTestContext, TestContext};

// TODO:
// add `Cow` types for borrowing
#[derive(Clone, Debug, PartialEq)]
pub enum Param {
    U64(u64),
    Bool(bool),
    String(String),
    Id(Id),
    Key(Key),
    #[allow(private_interfaces)]
    TestContext(SimulatorTestContext),
    Bytes(Vec<u8>),
    Path(String),
    Address(Address),
}

#[derive(Serialize)]
#[serde(rename_all = "lowercase", tag = "type", content = "value")]
enum StringParam {
    U64(String),
    Bool(String),
    String(String),
    Id(String),
    Bytes(String),
    Path(String),
    Address(String),
}

impl From<&Param> for StringParam {
    fn from(value: &Param) -> Self {
        match value {
            Param::U64(num) => StringParam::U64(b64.encode(num.to_le_bytes())),
            Param::Bool(flag) => StringParam::Bool(b64.encode(vec![*flag as u8])),
            Param::String(text) => StringParam::String(
                b64.encode(borsh::to_vec(text).expect("the serialization should work")),
            ),
            Param::Path(text) => StringParam::Path(b64.encode(text)),
            Param::Bytes(bytes) => StringParam::Bytes(
                b64.encode(borsh::to_vec(bytes).expect("the serialization should work")),
            ),
            Param::Address(addr) => StringParam::Address(b64.encode(addr)),
            Param::Id(id) => {
                let num: &usize = id.into();
                StringParam::Id(b64.encode(num.to_le_bytes()))
            }
            Param::Key(_) | Param::TestContext(_) => unreachable!(),
        }
    }
}

impl Serialize for Param {
    fn serialize<S>(&self, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        match self {
            Param::Key(key) => Serialize::serialize(key, serializer),
            Param::TestContext(ctx) => Serialize::serialize(ctx, serializer),
            _ => StringParam::from(self).serialize(serializer),
        }
    }
}

impl From<u64> for Param {
    fn from(val: u64) -> Self {
        Param::U64(val)
    }
}

impl From<bool> for Param {
    fn from(val: bool) -> Self {
        Param::Bool(val)
    }
}

impl From<String> for Param {
    fn from(val: String) -> Self {
        Param::String(val)
    }
}

impl From<Id> for Param {
    fn from(val: Id) -> Self {
        Param::Id(val)
    }
}

impl From<Key> for Param {
    fn from(val: Key) -> Self {
        Param::Key(val)
    }
}

impl From<TestContext> for Param {
    fn from(val: TestContext) -> Self {
        Param::TestContext(SimulatorTestContext { value: val })
    }
}

impl From<Vec<u8>> for Param {
    fn from(val: Vec<u8>) -> Self {
        Param::Bytes(val)
    }
}

impl From<Address> for Param {
    fn from(addr: Address) -> Self {
        Param::Address(addr)
    }
}