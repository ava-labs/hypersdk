//! A client and types for the VM simulator. This crate allows for Rust
//! developers to construct tests for their programs completely in Rust.
//! Alternatively the [`Step`]s can be written in JSON and passed to the
//! Simulator binary directly.

use base64::{engine::general_purpose::STANDARD as b64, Engine};
use borsh::BorshDeserialize;
use param::Param;
use serde::{Deserialize, Deserializer, Serialize, Serializer};
use std::{
    io::{BufRead, BufReader, Write},
    path::Path,
    process::{Child, Command, Stdio},
};
use step::StepError;
use thiserror::Error;
use wasmlanche_sdk::{Address, ExternalCallError};

mod codec;
mod id;
pub mod param;
pub mod step;
use crate::codec::base64_encode;
use crate::step::{Step, StepResultItem};
pub use id::Id;

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
}

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
pub(crate) struct SimulatorTestContext {
    #[serde(serialize_with = "base64_encode")]
    value: TestContext,
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

fn get_path() -> &'static str {
    let path = env!("SIMULATOR_PATH");

    if !Path::new(path).exists() {
        eprintln!();
        eprintln!("Simulator binary not found at path: {path}");
        eprintln!();
        eprintln!("Please run `cargo clean -p simulator` and rebuild your dependent crate.");
        eprintln!();

        panic!("Simulator binary not found, must rebuild simulator");
    }

    path
}

/// A [`Client`] is required to pass [`Step`]s to the simulator by calling [`run`](Self::run_step).
pub struct Simulator<W, R> {
    writer: W,
    responses: R,
}

pub fn build() -> Result<Simulator<impl Write, impl Iterator<Item = StepResultItem>>, ClientError> {
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
        .map(|line| serde_json::from_str(&line?).map_err(StepError::Serde));

    Ok(Simulator { writer, responses })
}

impl<W, R> Simulator<W, R>
where
    W: Write,
    R: Iterator<Item = StepResultItem>,
{
    pub fn run_step(&mut self, step: &Step) -> StepResultItem {
        let run_command = b"run --step '";
        self.writer.write_all(run_command)?;

        let input = serde_json::to_vec(&step).map_err(StepError::Serde)?;
        self.writer.write_all(&input)?;
        self.writer.write_all(b"'\n")?;
        self.writer.flush()?;
        // println!("input: {:?}", input);
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

    pub fn create_program<P: AsRef<Path>>(&mut self, path: P) -> StepResultItem {
        let path = path.as_ref().to_string_lossy();
        let run_command = b"run --step '";
        self.writer.write_all(run_command)?;

        let input = serde_json::to_vec(&Step {
            endpoint: Endpoint::Execute,
            method: "program_create".into(),
            max_units: 0,
            params: vec![Param::Path(path.into())],
        })?;
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

    // pub fn read(&mut self, method: String, params: Vec<Param>) -> Result<StepResultItem, ClientError> {
    //     let read_command = b"read --method '";
    //     self.writer.write_all(read_command)?;

    //     let input = serde_json::to_vec(&Step {
    //         endpoint: Endpoint::ReadOnly,
    //         method,
    //         max_units: 1000000,
    //         params,
    //     })?;

    //     self.writer.write_all(&input)?;
    //     self.writer.write_all(b"'\n")?;
    //     self.writer.flush()?;

    //     Ok(self.responses.next().ok_or(ClientError::Eof)?)
    // }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::param::Param;
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
