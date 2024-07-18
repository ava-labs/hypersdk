use crate::codec::{base64_decode, id_from_usize};
use crate::{param::Param, ClientError, Endpoint, Id};
use base64::{engine::general_purpose::STANDARD as b64, Engine};
use borsh::BorshDeserialize;
use serde::{Deserialize, Deserializer, Serialize, Serializer};
use std::borrow::Cow;
use std::path::Path;
use thiserror::Error;
use wasmlanche_sdk::ExternalCallError;

/// A [`SimulatorRequest`] is a call to the simulator
#[derive(Debug, Serialize, PartialEq, Clone)]
#[serde(rename_all = "camelCase")]
pub struct SimulatorRequest {
    /// The API endpoint to call.
    pub endpoint: Endpoint,
    /// The method to call on the endpoint.
    pub method: String,
    /// The maximum number of units the Request can consume.
    pub max_units: u64,
    /// The parameters to pass to the method.
    pub params: Vec<Param>,
}

impl SimulatorRequest {
    pub fn new_read(method: String, params: Vec<Param>) -> Self {
        Self {
            endpoint: Endpoint::ReadOnly,
            method: method.to_string(),
            max_units: 0,
            params,
        }
    }

    pub fn new_execute(method: String, params: Vec<Param>, max_units: u64) -> Self {
        Self {
            endpoint: Endpoint::Execute,
            method,
            max_units,
            params,
        }
    }

    pub fn new_create_program(path: String) -> Self {
        Self {
            endpoint: Endpoint::CreateProgram,
            // TODO: this does not need to be passed in
            method: "create_program".to_string(),
            max_units: 0,
            params: vec![Param::Path(path)],
        }
    }
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
