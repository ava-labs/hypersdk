//! A client and types for the VM simulator. This crate allows for Rust
//! developers to construct tests for their programs completely in Rust.
//! Alternatively the `Plan` can be written in JSON and passed to the
//! Simulator binary directly.

use borsh::BorshDeserialize;
use serde::{Deserialize, Serialize};
use serde_with::{base64::Base64, serde_as, DisplayFromStr};
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

/// A [Plan] is made up of [Step]s. Each step is a call to the API and can include verification.
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
    Ed25519(String),
    Secp256r1(String),
}

// TODO:
// add `Cow` types for borrowing
#[serde_as]
#[derive(Clone, Debug, Serialize, Deserialize, PartialEq)]
#[serde(rename_all = "lowercase", tag = "type", content = "value")]
pub enum Param {
    U64(#[serde_as(as = "DisplayFromStr")] u64),
    String(String),
    Id(Id),
    #[serde(untagged)]
    Key(Key),
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

#[derive(Debug, Serialize, PartialEq)]
pub struct Plan<'a> {
    /// The key of the caller used in each step of the plan.
    pub caller_key: &'a str,
    /// The steps to perform in the plan.
    pub steps: Vec<Step>,
}

impl<'a> Plan<'a> {
    /// Pass in the `caller_key` to be used in each step of the plan.
    #[must_use]
    pub fn new(caller_key: &'a str) -> Self {
        Self {
            caller_key,
            steps: vec![],
        }
    }

    /// returns the [Id] of the added [Step]
    pub fn add_step(&mut self, step: Step) -> Id {
        self.steps.push(step);
        Id::from(self.steps.len() - 1)
    }
}

#[derive(Debug, Deserialize)]
pub struct BaseResponse {
    /// The numeric id of the step.
    pub id: u32,
    /// An optional error message.
    pub error: Option<String>,
}

#[serde_as]
#[derive(Debug, Deserialize)]
pub struct PlanResult {
    /// The ID created from the program execution.
    pub id: Option<String>,
    /// An optional message.
    pub msg: Option<String>,
    /// The timestamp of the function call response.
    pub timestamp: u64,
    /// The result of the function call.
    #[serde_as(as = "Base64")]
    pub response: Vec<u8>,
}

#[derive(Debug, Deserialize)]
pub struct BaseResultTyped<T>
where
    T: BorshDeserialize,
{
    /// The ID created from the program execution.
    pub id: Option<String>,
    /// An optional message.
    pub msg: Option<String>,
    /// The timestamp of the function call response.
    pub timestamp: u64,
    /// The result of the function call.
    pub response: T,
}

#[serde_as]
#[derive(Debug, Deserialize)]
pub struct PlanResponse {
    #[serde(flatten)]
    pub base: BaseResponse,
    /// The result of the plan.
    pub result: PlanResult,
}

#[derive(Debug, Deserialize)]
pub struct PlanResponseTyped<T>
where
    T: BorshDeserialize,
{
    #[serde(flatten)]
    pub base: BaseResponse,
    /// The result of the plan.
    pub result: BaseResultTyped<T>,
}

impl<T> TryInto<PlanResponseTyped<T>> for PlanResponse
where
    T: BorshDeserialize,
{
    type Error = borsh::io::Error;

    fn try_into(self) -> Result<PlanResponseTyped<T>, Self::Error> {
        let PlanResponse {
            base: BaseResponse { id: resp_id, error },
            result:
                PlanResult {
                    id,
                    msg,
                    timestamp,
                    response,
                },
        } = self;

        Ok(PlanResponseTyped {
            base: BaseResponse { id: resp_id, error },
            result: BaseResultTyped {
                id,
                msg,
                timestamp,
                response: borsh::from_slice(&response)?,
            },
        })
    }
}

#[derive(Error, Debug)]
pub enum ClientError {
    #[error("Read error: {0}")]
    Read(#[from] std::io::Error),
    #[error("Deserialization error: {0}")]
    Deserialization(#[from] serde_json::Error),
    #[error("Borsh deserialization error")]
    BorshDeserialization,
    #[error("EOF")]
    Eof,
    #[error("Missing handle")]
    StdIo,
}

/// A [Client] is required to pass a [Plan] to the simulator, then to [run](Self::run_plan) the actual simulation.
pub struct Client<W, R> {
    writer: W,
    responses: R,
}

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

    pub fn build(
        self,
    ) -> Result<
        Client<impl Write, impl Iterator<Item = Result<PlanResponse, ClientError>>>,
        ClientError,
    > {
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
            .map(|line| serde_json::from_str(&line?).map_err(Into::into));

        Ok(Client { writer, responses })
    }
}

impl<W, R> Client<W, R>
where
    W: Write,
    R: Iterator<Item = Result<PlanResponse, ClientError>>,
{
    /// Runs a [Plan] against the simulator and returns vec of result.
    /// # Errors
    ///
    /// Returns an error if the serialization or plan fails.
    pub fn run_plan(&mut self, plan: Plan) -> Result<Vec<PlanResponse>, ClientError> {
        plan.steps
            .iter()
            .map(|step| self._run_step(plan.caller_key, step))
            .collect()
    }

    fn _run_step(&mut self, caller_key: &str, step: &Step) -> Result<PlanResponse, ClientError> {
        let run_command = b"run --step '";
        self.writer.write_all(run_command)?;

        let step = SimulatorStep { caller_key, step };
        let input = serde_json::to_vec(&step)?;
        self.writer.write_all(&input)?;
        self.writer.write_all(b"'\n")?;
        self.writer.flush()?;

        self.responses.next().ok_or(ClientError::Eof)?
    }

    pub fn run_step<T>(
        &mut self,
        caller_key: &str,
        step: &Step,
    ) -> Result<PlanResponseTyped<T>, ClientError>
    where
        T: BorshDeserialize,
    {
        self._run_step(caller_key, step)?
            .try_into()
            .map_err(|_| ClientError::BorshDeserialization)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn convert_u64_param() {
        let value = 42u64;
        let expected_param_type = "u64";
        let expected_value = value.to_string();

        let expected_json = json!({
            "type": expected_param_type,
            "value": &expected_value,
        });

        let param = Param::from(value);
        let expected_param = Param::U64(value);

        assert_eq!(param, expected_param);

        let output_json = serde_json::to_value(&param).unwrap();

        assert_eq!(output_json, expected_json);

        let output_param: Param = serde_json::from_value(expected_json).unwrap();

        assert_eq!(output_param, expected_param);
    }

    #[test]
    fn convert_string_param() {
        let value = String::from("hello world");
        let expected_param_type = "string";
        let expected_value = value.clone();

        let expected_json = json!({
            "type": expected_param_type,
            "value": &expected_value,
        });

        let param = Param::from(value.clone());
        let expected_param = Param::String(value);

        assert_eq!(param, expected_param);

        let output_json = serde_json::to_value(&param).unwrap();

        assert_eq!(output_json, expected_json);

        let output_param: Param = serde_json::from_value(expected_json).unwrap();

        assert_eq!(output_param, expected_param);
    }

    #[test]
    fn convert_id_param() {
        let value = 42;
        let expected_param_type = "id";
        let expected_value = format!("step_{value}");

        let expected_json = json!({
            "type": expected_param_type,
            "value": &expected_value,
        });

        let id = Id::from(value);
        let param = Param::from(id);
        let expected_param = Param::Id(id);

        assert_eq!(param, expected_param);

        let output_json = serde_json::to_value(&param).unwrap();

        assert_eq!(output_json, expected_json);

        let output_param: Param = serde_json::from_value(expected_json).unwrap();

        assert_eq!(output_param, expected_param);
    }

    #[test]
    fn convert_key_param() {
        let expected_param_type = "ed25519";
        let expected_value = "id".into();

        let expected_json = json!({
            "type": expected_param_type,
            "value": &expected_value,
        });

        let key = Key::Ed25519(expected_value);
        let param = Param::from(key.clone());
        let expected_param = Param::Key(key);

        assert_eq!(param, expected_param);

        let output_json = serde_json::to_value(&param).unwrap();

        assert_eq!(output_json, expected_json);

        let output_param: Param = serde_json::from_value(expected_json).unwrap();

        assert_eq!(output_param, expected_param);
    }
}
