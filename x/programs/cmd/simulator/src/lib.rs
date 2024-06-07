//! A client and types for the VM simulator. This crate allows for Rust
//! developers to construct tests for their programs completely in Rust.
//! Alternatively the [Step]s can be written in JSON and passed to the
//! Simulator binary directly.

use base64::{engine::general_purpose::STANDARD as b64, Engine};
use borsh::BorshDeserialize;
use serde::{Deserialize, Deserializer, Serialize, Serializer};
use std::{
    io::{BufRead, BufReader, Write},
    path::Path,
    process::{Child, Command, Stdio},
};
use thiserror::Error;

mod id;

pub use id::Id;

/// The endpoint to call for a [Step].
#[derive(Debug, Serialize, PartialEq, Clone)]
#[serde(rename_all = "lowercase")]
pub enum Endpoint {
    /// Perform an operation against the key api.
    Key,
    /// Make a read-only call to a program function and return the result.
    ReadOnly,
    /// Create a transaction on-chain from a possible state changing program
    /// function call. A program's function can internally optionally call other
    /// functions including program to program.
    Execute,
}

/// A [Step] is a call to the simulator
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

#[derive(Debug, Serialize, PartialEq)]
#[serde(rename_all = "camelCase")]
pub struct SimulatorStep<'a> {
    /// The key of the caller used in each step of the plan.
    pub caller_key: &'a str,
    #[serde(flatten)]
    pub step: &'a Step,
}

impl Step {
    /// Create a [Step] that creates a key.
    #[must_use]
    pub fn create_key(key: Key) -> Self {
        Self {
            endpoint: Endpoint::Key,
            method: "create_key".into(),
            max_units: 0,
            params: vec![Param::Key(key)],
        }
    }

    /// Create a [Step] that creates a program.
    #[must_use]
    pub fn create_program<P: AsRef<Path>>(path: P) -> Self {
        let path = path.as_ref().to_string_lossy();

        Self {
            endpoint: Endpoint::Execute,
            method: "program_create".into(),
            max_units: 0,
            params: vec![Param::String(path.into())],
        }
    }
}

/// The algorithm used to generate the key along with a [String] identifier for the key.
#[derive(Clone, Debug, Serialize, Deserialize, PartialEq)]
#[serde(rename_all = "lowercase")]
#[serde(tag = "type", content = "value")]
pub enum Key {
    #[serde(serialize_with = "base64_encode")]
    Ed25519(String),
    #[serde(serialize_with = "base64_encode")]
    Secp256r1(String),
}

// TODO:
// add `Cow` types for borrowing
#[derive(Clone, Debug, PartialEq)]
pub enum Param {
    U64(u64),
    String(String),
    Id(Id),
    Key(Key),
}

#[derive(Serialize)]
#[serde(rename_all = "lowercase", tag = "type", content = "value")]
enum StringParam {
    U64(String),
    String(String),
    Id(String),
}

impl From<&Param> for StringParam {
    fn from(value: &Param) -> Self {
        match value {
            Param::U64(num) => StringParam::U64(b64.encode(num.to_le_bytes())),
            Param::String(text) => StringParam::String(b64.encode(text.clone())),
            Param::Id(id) => {
                let num: &usize = id.into();
                StringParam::Id(b64.encode(num.to_le_bytes()))
            }
            Param::Key(_) => unreachable!(),
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
            _ => StringParam::from(self).serialize(serializer),
        }
    }
}

impl From<u64> for Param {
    fn from(val: u64) -> Self {
        Param::U64(val)
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

impl StepResult {
    pub fn response<T>(&self) -> Result<T, borsh::io::Error>
    where
        T: BorshDeserialize,
    {
        borsh::from_slice(&self.response)
    }
}

fn base64_encode<S>(text: &str, serializer: S) -> Result<S::Ok, S::Error>
where
    S: Serializer,
{
    serializer.serialize_str(&b64.encode(text))
}

fn base64_decode<'de, D>(deserializer: D) -> Result<Vec<u8>, D::Error>
where
    D: Deserializer<'de>,
{
    <&str>::deserialize(deserializer).and_then(|s| {
        b64.decode(s)
            .map_err(|err| serde::de::Error::custom(err.to_string()))
    })
}

#[derive(Debug, Deserialize)]
pub struct StepResponse {
    /// The numeric id of the step.
    #[serde(deserialize_with = "id_from_usize")]
    pub id: Id, // TODO override of the Id Deserialize before removing the prefix
    /// An optional error message.
    pub error: Option<String>,
    pub result: StepResult,
}

fn id_from_usize<'de, D>(deserializer: D) -> Result<Id, D::Error>
where
    D: Deserializer<'de>,
{
    <usize as Deserialize>::deserialize(deserializer).map(Id::from)
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

/// A [Client] is required to pass [Step]s to the simulator by calling [run](Self::run_step).
pub struct Client<W, R> {
    writer: W,
    responses: R,
}

type StepResultItem = Result<StepResponse, StepError>;

pub struct ClientBuilder<'a> {
    path: &'a str,
}

impl ClientBuilder<'_> {
    #[allow(clippy::new_without_default)]
    pub fn new() -> Self {
        let path = env!("SIMULATOR_PATH");

        if !Path::new(path).exists() {
            eprintln!();
            eprintln!("Simulator binary not found at path: {path}");
            eprintln!();
            eprintln!("Please run `cargo clean -p simulator` and rebuild your dependent crate.");
            eprintln!();

            panic!("Simulator binary not found, must rebuild simulator");
        }

        Self { path }
    }

    pub fn try_build(
        self,
    ) -> Result<Client<impl Write, impl Iterator<Item = StepResultItem>>, ClientError> {
        let Child { stdin, stdout, .. } = Command::new(self.path)
            .arg("interpreter")
            .arg("--cleanup")
            .arg("--log-level")
            .arg("error")
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .spawn()?;

        let writer = stdin.ok_or(ClientError::StdIo)?;
        let reader = stdout.ok_or(ClientError::StdIo)?;

        let responses = BufReader::new(reader)
            .lines()
            .map(|line| serde_json::from_str(&line?).map_err(StepError::Serde));

        Ok(Client { writer, responses })
    }
}

impl<W, R> Client<W, R>
where
    W: Write,
    R: Iterator<Item = StepResultItem>,
{
    pub fn run_step(&mut self, caller_key: &str, step: &Step) -> StepResultItem {
        let run_command = b"run --step '";
        self.writer.write_all(run_command)?;

        let step = SimulatorStep { caller_key, step };
        let input = serde_json::to_vec(&step).map_err(StepError::Serde)?;
        self.writer.write_all(&input)?;
        self.writer.write_all(b"'\n")?;
        self.writer.flush()?;

        self.responses
            .next()
            .ok_or(StepError::Client(ClientError::Eof))?
            .and_then(|step| {
                if let Some(err) = step.error {
                    Err(StepError::Program(err))
                } else {
                    Ok(step)
                }
            })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use base64::{engine::general_purpose::STANDARD as b64, Engine};
    use serde_json::json;

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
        let expected_value = value.clone();

        let expected_json = json!({
            "type": expected_param_type,
            "value": &b64.encode(expected_value),
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
    fn convert_key_param() {
        let expected_param_type = "ed25519";
        let expected_value = "id";

        let expected_json = json!({
            "type": expected_param_type,
            "value": &b64.encode(expected_value),
        });

        let key = Key::Ed25519(expected_value.to_string());
        let param = Param::from(key.clone());
        let expected_param = Param::Key(key);

        assert_eq!(param, expected_param);

        let output_json = serde_json::to_value(&param).unwrap();

        assert_eq!(output_json, expected_json);
    }
}
