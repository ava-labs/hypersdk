use base64::{engine::general_purpose::STANDARD as b64, Engine};
use borsh::BorshDeserialize;
use serde::{Deserialize, Deserializer, Serialize, Serializer};
use std::path::Path;
use thiserror::Error;
use wasmlanche_sdk::ExternalCallError;

use crate::codec::{base64_decode, id_from_usize};
use crate::{param::Param, ClientError, Endpoint, Id};

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

#[derive(Error, Debug)]
pub enum SimulatorError {
    #[error("Client error: {0}")]
    Client(#[from] ClientError),
    #[error("Serialization / Deserialization error: {0}")]
    Serde(#[from] serde_json::Error),
    #[error("Borsh deserialization error: {0}")]
    BorshDeserialization(#[from] borsh::io::Error),
    #[error("Program error: {0}")]
    Program(String),
}
pub(crate) type SimulatorResponseItem = Result<SimulatorResponse, SimulatorError>;

impl Step {
    /// Create a [`Step`] that creates a program.
    #[must_use]
    pub fn create_program<P: AsRef<Path>>(path: P) -> Self {
        let path = path.as_ref().to_string_lossy();

        Self {
            endpoint: Endpoint::Execute,
            method: "program_create".into(),
            max_units: 0,
            params: vec![Param::Path(path.into())],
        }
    }
}

#[derive(Debug, Deserialize)]
pub struct SimulatorResult {
    /// The ID created from the program execution.
    pub action_id: Option<String>,
    /// The timestamp of the function call response.
    pub timestamp: u64,
    /// The result of the function call.
    #[serde(deserialize_with = "base64_decode")]
    response: Vec<u8>,
}

#[derive(Error, Debug)]
pub enum SimulatorResponseError {
    #[error(transparent)]
    Serialization(#[from] borsh::io::Error),
    #[error(transparent)]
    ExternalCall(#[from] ExternalCallError),
}

impl SimulatorResult {
    pub fn response<T>(&self) -> Result<T, SimulatorResponseError>
    where
        T: BorshDeserialize,
    {
        let res: Result<T, ExternalCallError> = borsh::from_slice(&self.response)?;
        res.map_err(SimulatorResponseError::ExternalCall)
    }
}

#[derive(Debug, Deserialize)]
pub struct SimulatorResponse {
    /// The numeric id of the step.
    #[serde(deserialize_with = "id_from_usize")]
    pub id: Id, // TODO override of the Id Deserialize before removing the prefix
    /// An optional error message.
    pub error: Option<String>,
    pub result: SimulatorResult,
}
