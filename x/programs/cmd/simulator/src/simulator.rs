use crate::{
    codec::{base64_decode, id_from_usize},
    param::Param,
    Id,
};
use borsh::{BorshDeserialize, BorshSerialize};
use serde::Serialize;
use std::{
    io::{BufRead, BufReader, Write},
    process::{Child, Command, Stdio},
};
use thiserror::Error;
use wasmlanche_sdk::ExternalCallError;

#[derive(Error, Debug)]
pub enum SimulatorResponseError {
    #[error(transparent)]
    Serialization(#[from] borsh::io::Error),
    #[error(transparent)]
    ExternalCall(#[from] ExternalCallError),
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

pub struct Simulator<W, R> {
    writer: W,
    responses: R,
}

pub fn build_simulator() -> Simulator<impl Write, impl Iterator<Item = SimulatorResponseItem>> {
    let path = get_binary_path();

    let Child { stdin, stdout, .. } = Command::new(path)
        .arg("interpreter")
        .arg("--cleanup")
        .arg("--log-level")
        .arg("debug")
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .spawn()
        .expect("unable to spawn simulator");

    let writer = stdin.expect("unable to get stdin");
    let reader = stdout.expect("unable to get stdout");

    let responses = BufReader::new(reader)
    .lines()
    .map(|line| borsh::from_slice(line?.as_bytes()).map_err(SimulatorError::BorshDeserialization));

    Simulator { writer, responses }
}

impl<W, R> Simulator<W, R>
where
    W: Write,
    R: Iterator<Item = SimulatorResponseItem>,
{
    const RUN_COMMAND: &'static [u8] = b"run --message '";

    pub fn create_program<P: AsRef<Path>>(&mut self, path: P) -> SimulatorResponseItem {
        self.writer.write_all(Self::RUN_COMMAND)?;

        let path = path.as_ref().to_string_lossy();
        let input = serde_json::to_vec(&SimulatorRequest::create_program(path.into()))?;

        self.writer.write_all(&input)?;
        self.writer.write_all(b"'\n")?;
        self.writer.flush()?;

        self.responses
            .next()
            .ok_or(SimulatorError::Client(ClientError::Eof))?
    }

    pub fn read(&mut self, method: String, params: Vec<Param>) -> SimulatorResponseItem {
        self.writer.write_all(Self::RUN_COMMAND)?;

        let input = serde_json::to_vec(&SimulatorRequest::read(method, params))?;

        self.writer.write_all(&input)?;
        self.writer.write_all(b"'\n")?;
        self.writer.flush()?;

        self.responses
            .next()
            .ok_or(SimulatorError::Client(ClientError::Eof))?
    }

    pub fn execute(
        &mut self,
        method: String,
        params: Vec<Param>,
        max_units: u64,
    ) -> SimulatorResponseItem {
        self.writer.write_all(Self::RUN_COMMAND)?;

        let input = serde_json::to_vec(&SimulatorRequest::execute(method, params, max_units))?;

        self.writer.write_all(&input)?;
        self.writer.write_all(b"'\n")?;
        self.writer.flush()?;

        self.responses
            .next()
            .ok_or(SimulatorError::Client(ClientError::Eof))?
    }
}

/// A [`SimulatorRequest`] is a call to the simulator
#[derive(Debug, Serialize, PartialEq, Clone)]
#[serde(rename_all = "camelCase")]
struct SimulatorRequest {
    /// The API endpoint to call.
    endpoint: Endpoint,
    /// The method to call on the endpoint.
    method: String,
    /// The maximum number of units the Request can consume.
    max_units: u64,
    /// The parameters to pass to the method.
    params: Vec<Param>,
}

impl SimulatorRequest {
    pub fn read(method: String, params: Vec<Param>) -> Self {
        Self {
            endpoint: Endpoint::ReadOnly,
            method: method.to_string(),
            max_units: 0,
            params,
        }
    }

    pub fn execute(method: String, params: Vec<Param>, max_units: u64) -> Self {
        Self {
            endpoint: Endpoint::Execute,
            method,
            max_units,
            params,
        }
    }

    pub fn create_program(path: String) -> Self {
        Self {
            endpoint: Endpoint::CreateProgram,
            // TODO: this does not need to be passed in
            method: "create_program".to_string(),
            max_units: 0,
            params: vec![Param::Path(path)],
        }
    }
}

type SimulatorResponseItem = Result<SimulatorResponse, SimulatorError>;

#[derive(Debug, BorshDeserialize, BorshSerialize)]
pub struct SimulatorResult {
    /// The ID created from the program execution.
    pub action_id: Option<String>,
    /// The timestamp of the function call response.
    pub timestamp: u64,
    /// The result of the function call.
    pub response: Vec<u8>,
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

#[derive(Debug, BorshDeserialize, BorshSerialize)]
pub struct SimulatorResponse {
    // #[serde(deserialize_with = "id_from_usize")]
    pub id: usize, // TODO override of the Id Deserialize before removing the prefix
    /// An optional error message.
    pub error: String,
    pub result: SimulatorResult,
}

/// The endpoint to call for a [`SimulatorRequest`].
#[derive(Debug, Serialize, PartialEq, Clone)]
#[serde(rename_all = "lowercase")]
enum Endpoint {
    /// Make a read-only call to a program function and return the result.
    ReadOnly,
    /// Create a transaction on-chain from a possible state changing program
    /// function call. A program's function can internally optionally call other
    /// functions including program to program.
    Execute,
    /// Create a new program on-chain
    CreateProgram,
}

use std::path::Path;
fn get_binary_path() -> &'static str {
    let path = env!("SIMULATOR_PATH");

    if !Path::new(path).exists() {
        eprintln!(
            r#"
        Simulator binary not found at path: {path}

        Please run `cargo clean -p simulator` and rebuild your dependent crate.

        "#
        );

        panic!("Simulator binary not found, must rebuild simulator");
    }

    path
}
