use crate::codec::base64_encode;
use crate::Id;
use serde::{Serialize, Serializer};
use wasmlanche_sdk::Address;

#[derive(Clone, Debug, PartialEq, Default)]
#[non_exhaustive]
pub struct TestContext {
    program_id: Id,
    pub actor: Address,
    pub height: u64,
    pub timestamp: u64,
}

impl Serialize for TestContext {
    fn serialize<S>(&self, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: Serializer,
    {
        #[derive(Serialize)]
        #[serde(rename_all = "camelCase")]
        struct BorrowedContext<'a> {
            program_id: Id,
            actor: &'a [u8],
            height: u64,
            timestamp: u64,
        }

        let Self {
            program_id,
            actor,
            height,
            timestamp,
        } = self;

        BorrowedContext {
            program_id: *program_id,
            actor: actor.as_ref(),
            height: *height,
            timestamp: *timestamp,
        }
        .serialize(serializer)
    }
}

impl From<Id> for TestContext {
    fn from(program_id: Id) -> Self {
        Self {
            program_id,
            ..Default::default()
        }
    }
}

#[derive(Clone, Debug, PartialEq, Serialize)]
#[serde(tag = "type", rename = "testContext")]
pub struct SimulatorTestContext {
    #[serde(serialize_with = "base64_encode")]
    pub value: TestContext,
}
