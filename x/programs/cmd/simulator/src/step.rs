
use std::path::Path;
use base64::{engine::general_purpose::STANDARD as b64, Engine};
use borsh::BorshDeserialize;
use serde::{Deserialize, Deserializer, Serialize, Serializer};
use thiserror::Error;
use wasmlanche_sdk::ExternalCallError;

use crate::{param::Param, ClientError, Endpoint, Id, Key};
use crate::codec::{base64_decode, id_from_usize};

/// A [`Step`] is a call to the simulator
#[derive(Debug, Serialize, PartialEq, Clone)]
#[serde(rename_all = "camelCase")]
pub struct Step {
    /// The API endpoint to call.
    pub endpoint: Endpoint,
    /// The method to call on the endpoint.
    pub method: String,
    /// The maximum number of units the step can consume.
    pub max_units: u64,
    /// The parameters to pass to the method.
    pub params: Vec<Param>,
}

impl Step {
    /// Create a [`Step`] that creates a key.
    #[must_use]
    pub fn create_key(key: Key) -> Self {
        Self {
            endpoint: Endpoint::Key,
            method: "key_create".into(),
            max_units: 0,
            params: vec![Param::Key(key)],
        }
    }
}


#[derive(Debug, Deserialize)]
pub struct StepResult {
    /// The ID created from the program execution.
    pub action_id: Option<String>,
    /// An optional message.
    pub msg: Option<String>,
    /// The timestamp of the function call response.
    pub timestamp: u64,
    /// The result of the function call.
    #[serde(deserialize_with = "base64_decode")]
    response: Vec<u8>,
}

#[derive(Error, Debug)]
pub enum StepResponseError {
    #[error(transparent)]
    Serialization(#[from] borsh::io::Error),
    #[error(transparent)]
    ExternalCall(#[from] ExternalCallError),
}

impl StepResult {
    pub fn response<T>(&self) -> Result<T, StepResponseError>
    where
        T: BorshDeserialize,
    {
        let res: Result<T, ExternalCallError> = borsh::from_slice(&self.response)?;
        res.map_err(StepResponseError::ExternalCall)
    }
}

pub(crate) type StepResultItem = Result<StepResponse, StepError>;

#[derive(Debug, Deserialize)]
pub struct StepResponse {
    /// The numeric id of the step.
    #[serde(deserialize_with = "id_from_usize")]
    pub id: Id, // TODO override of the Id Deserialize before removing the prefix
    /// An optional error message.
    pub error: Option<String>,
    pub result: StepResult,
}


#[derive(Error, Debug)]
pub enum StepError {
    #[error("Client error: {0}")]
    Client(#[from] ClientError),
    #[error("Serialization / Deserialization error: {0}")]
    Serde(#[from] serde_json::Error),
    #[error("Borsh deserialization error: {0}")]
    BorshDeserialization(#[from] borsh::io::Error),
    #[error("Program error: {0}")]
    Program(String),
}

