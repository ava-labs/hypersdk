use crate::codec::{base64_decode, id_from_usize};
use crate::util::get_path;
use crate::{param::Param, Id};
use borsh::BorshDeserialize;
use serde::{Deserialize, Serialize};
use std::{
    io::{BufRead, BufReader, Write},
    path::Path,
    process::{Child, Command, Stdio},
};
use thiserror::Error;
use wasmlanche_sdk::ExternalCallError;

/// The endpoint to call for a [`Step`].
#[derive(Debug, Serialize, PartialEq, Clone)]
#[serde(rename_all = "lowercase")]
pub enum Endpoint {
    /// Make a read-only call to a program function and return the result.
    ReadOnly,
    /// Create a transaction on-chain from a possible state changing program
    /// function call. A program's function can internally optionally call other
    /// functions including program to program.
    Execute,
    /// Create a new program on-chain
    CreateProgram,
}

#[derive(Error, Debug)]
pub enum ClientError {
    #[error("Read error: {0}")]
    Read(#[from] std::io::Error),
    #[error("EOF")]
    Eof,
    #[error("Missing handle")]
    StdIo,
}

/// A [`Client`] is required to pass [`Step`]s to the simulator by calling [`run`](Self::run_step).
pub struct Simulator<W, R> {
    writer: W,
    responses: R,
}

pub fn build_simulator(
) -> Result<Simulator<impl Write, impl Iterator<Item = SimulatorResponseItem>>, ClientError> {
    let path = get_path();

    let Child { stdin, stdout, .. } = Command::new(path)
        .arg("interpreter")
        .arg("--cleanup")
        .arg("--log-level")
        .arg("debug")
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .spawn()?;

    let writer = stdin.ok_or(ClientError::StdIo)?;
    let reader = stdout.ok_or(ClientError::StdIo)?;

    let responses = BufReader::new(reader)
        .lines()
        .map(|line| serde_json::from_str(&line?).map_err(SimulatorError::Serde));

    Ok(Simulator { writer, responses })
}

impl<W, R> Simulator<W, R>
where
    W: Write,
    R: Iterator<Item = SimulatorResponseItem>,
{
    const RUN_COMMAND: &'static [u8] = b"run --message '";

    pub fn create_program<P: AsRef<Path>>(&mut self, path: P) -> SimulatorResponseItem {
        let path = path.as_ref().to_string_lossy();
        self.writer.write_all(Self::RUN_COMMAND)?;

        let input = serde_json::to_vec(&SimulatorRequest::new_create_program(path.into()))?;

        self.writer.write_all(&input)?;
        self.writer.write_all(b"'\n")?;
        self.writer.flush()?;

        self.responses
            .next()
            .ok_or(SimulatorError::Client(ClientError::Eof))?
            .and_then(|resp: SimulatorResponse| {
                if let Some(err) = resp.error {
                    Err(SimulatorError::Program(err))
                } else {
                    Ok(resp)
                }
            })
    }

    pub fn read(&mut self, method: String, params: Vec<Param>) -> SimulatorResponseItem {
        self.writer.write_all(Self::RUN_COMMAND)?;

        let input = serde_json::to_vec(&SimulatorRequest::new_read(method, params))?;

        self.writer.write_all(&input)?;
        self.writer.write_all(b"'\n")?;
        self.writer.flush()?;

        self.responses
            .next()
            .ok_or(SimulatorError::Client(ClientError::Eof))?
            .and_then(|step| {
                if let Some(err) = step.error {
                    Err(SimulatorError::Program(err))
                } else {
                    Ok(step)
                }
            })
    }

    pub fn execute(
        &mut self,
        method: String,
        params: Vec<Param>,
        max_units: u64,
    ) -> SimulatorResponseItem {
        self.writer.write_all(Self::RUN_COMMAND)?;

        let input = serde_json::to_vec(&SimulatorRequest::new_execute(method, params, max_units))?;

        self.writer.write_all(&input)?;
        self.writer.write_all(b"'\n")?;
        self.writer.flush()?;

        self.responses
            .next()
            .ok_or(SimulatorError::Client(ClientError::Eof))?
            .and_then(|step| {
                if let Some(err) = step.error {
                    Err(SimulatorError::Program(err))
                } else {
                    Ok(step)
                }
            })
    }
}

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

#[cfg(test)]
mod tests {
    use super::*;
    use crate::param::Param;
    use base64::{engine::general_purpose::STANDARD as b64, Engine};
    use serde_json::json;
    use wasmlanche_sdk::Address;

    #[test]
    fn convert_u64_param() {
        let value = 42u64;
        let expected_param_type = "u64";
        let expected_value = value.to_le_bytes();

        let expected_json = json!({
            "type": expected_param_type,
            "value": &b64.encode(expected_value),
        });

        let param = Param::from(value);
        let expected_param = Param::U64(value);

        assert_eq!(param, expected_param);

        let output_json = serde_json::to_value(&param).unwrap();

        assert_eq!(output_json, expected_json);
    }

    #[test]
    fn convert_string_param() {
        let value = String::from("hello world");
        let expected_param_type = "string";

        let expected_json = json!({
            "type": expected_param_type,
            "value": &b64.encode(borsh::to_vec(&value).unwrap()),
        });

        let param = Param::from(value.clone());
        let expected_param = Param::String(value);

        assert_eq!(param, expected_param);

        let output_json = serde_json::to_value(&param).unwrap();

        assert_eq!(output_json, expected_json);
    }

    #[test]
    fn convert_id_param() {
        let value: usize = 42;
        let expected_param_type = "id";
        let expected_value = value.to_le_bytes();

        let expected_json = json!({
            "type": expected_param_type,
            "value": &b64.encode(expected_value)
        });

        let id = Id::from(value);
        let param = Param::from(id);
        let expected_param = Param::Id(id);

        assert_eq!(param, expected_param);

        let output_json = serde_json::to_value(&param).unwrap();

        assert_eq!(output_json, expected_json);
    }

    #[test]
    fn convert_bool_param() {
        let value = false;
        let expected_value = value as u8;

        let expected_json = json!({
            "type": "bool",
            "value": &b64.encode(vec![expected_value]),
        });

        let param = Param::from(value);
        let expected_param = Param::Bool(value);

        assert_eq!(param, expected_param);

        let output_json = serde_json::to_value(&param).unwrap();

        assert_eq!(output_json, expected_json);
    }

    #[test]
    fn convert_bytes_param() {
        let value = vec![12, 34, 56, 78, 90];

        let expected_json = json!({
            "type": "bytes",
            "value": &b64.encode(borsh::to_vec(&value).unwrap()),
        });

        let param = Param::from(value.clone());
        let expected_param = Param::Bytes(value);

        assert_eq!(param, expected_param);

        let output_json = serde_json::to_value(&param).unwrap();

        assert_eq!(output_json, expected_json);
    }

    #[test]
    fn convert_address_param() {
        let value = Address::default();

        let expected_json = json!({
            "type": "address",
            "value": &b64.encode(value),
        });

        let param = Param::from(value);
        let expected_param = Param::Address(value);

        assert_eq!(param, expected_param);

        let output_json = serde_json::to_value(&param).unwrap();

        assert_eq!(output_json, expected_json);
    }
}
