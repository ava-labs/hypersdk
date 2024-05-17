//! A client and types for the VM simulator. This crate allows for Rust
//! developers to construct tests for their programs completely in Rust.
//! Alternatively the `Plan` can be written in JSON and passed to the
//! Simulator binary directly.

use serde::{Deserialize, Serialize};
use serde_with::{serde_as, DisplayFromStr};
use std::{
    io::{BufRead, BufReader, Write},
    path::Path,
    process::{Child, Command, Stdio},
};
use thiserror::Error;
use wasmlanche_sdk::Params;

mod id;

pub use id::Id;

/// The endpoint to call for a [Step].
#[derive(Debug, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
#[deprecated]
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
#[derive(Debug, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct Step {
    /// The API endpoint to call.
    pub endpoint: Endpoint,
    /// The method to call on the endpoint.
    pub method: String,
    /// The maximum number of units the step can consume.
    pub max_units: u64,
    /// The parameters to pass to the method.
    pub params: Vec<Param>,
    /// If defined the result of the step must match this assertion.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub require: Option<Require>,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct StepTODO {
    step_type: StepType,
    message: ExecuteMessage,
}

impl From<KeyStep> for StepTODO {
    fn from(value: KeyStep) -> Self {
        Self {
            step_type: StepType::Key,
            message: ExecuteMessage::Key(value),
        }
    }
}

impl From<CallStep> for StepTODO {
    fn from(value: CallStep) -> Self {
        Self {
            step_type: StepType::Call,
            message: ExecuteMessage::Call(value),
        }
    }
}

impl From<CreateStep> for StepTODO {
    fn from(value: CreateStep) -> Self {
        Self {
            step_type: StepType::Create,
            message: ExecuteMessage::Create(value),
        }
    }
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "lowercase")]
pub enum StepType {
    Key,
    Call,
    Create,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub enum ExecuteMessage {
    Key(KeyStep),
    Call(CallStep),
    Create(CreateStep),
}

#[derive(Debug, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum Curve {
    Ed25519,
    Secp256r1,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct KeyStep {
    pub name: String,
    pub curve: Curve,
}

#[derive(Debug, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CallStep {
    pub program_id: Id,
    pub method: String,
    // #[serde(rename = "data", serialize_with = "borsh_serialize")]
    #[serde(rename = "data")]
    pub params: Vec<Param>,
    // pub params: Params,
    // pub params: Vec<u8>,
    pub max_units: u64,
}

// fn borsh_serialize<S>(params: &Vec<Param>, s: S) -> Result<S::Ok, S::Error> where S: Serializer {
//     let v = borsh::to_vec(params).map_err(|_| S::Error::custom("borsh serialization failed"))?;
//     v.serialize(serializer)
// }

#[derive(Debug, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ReadStep {
    pub program_id: Id,
    pub method: String,
    pub params: Vec<Param>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct CreateStep {
    pub path: String,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct SimulatorStep<'a> {
    /// The key of the caller used in each step of the plan.
    pub caller_key: &'a str,
    #[serde(flatten)]
    pub step: &'a StepTODO,
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
            require: None,
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
            require: None,
        }
    }
}

/// The algorithm used to generate the key along with a [String] identifier for the key.
#[derive(Debug, Serialize, Deserialize, Clone)]
#[serde(rename_all = "lowercase")]
#[serde(tag = "type", content = "value")]
pub enum Key {
    Ed25519(String),
    Secp256r1(String),
}

// TODO:
// add `Cow` types for borrowing
#[serde_as]
#[derive(Debug, Serialize, Deserialize, Clone)]
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

#[derive(Debug, Serialize, Deserialize)]
pub struct Require {
    /// If defined the result of the step must match this assertion.
    pub result: ResultAssertion,
}

#[serde_as]
#[derive(Debug, Serialize, Deserialize)]
#[serde(tag = "operator", content = "value")]
pub enum ResultAssertion {
    #[serde(rename = "==")]
    NumericEq(#[serde_as(as = "DisplayFromStr")] u64),
    #[serde(rename = "!=")]
    NumericNe(#[serde_as(as = "DisplayFromStr")] u64),
    #[serde(rename = ">")]
    NumericGt(#[serde_as(as = "DisplayFromStr")] u64),
    #[serde(rename = "<")]
    NumericLt(#[serde_as(as = "DisplayFromStr")] u64),
    #[serde(rename = ">=")]
    NumericGe(#[serde_as(as = "DisplayFromStr")] u64),
    #[serde(rename = "<=")]
    NumericLe(#[serde_as(as = "DisplayFromStr")] u64),
}

#[derive(Debug, Serialize)]
pub struct Plan {
    /// The key of the caller used in each step of the plan.
    pub caller_key: String,
    /// The steps to perform in the plan.
    pub steps: Vec<StepTODO>,
}

impl Plan {
    /// Pass in the `caller_key` to be used in each step of the plan.
    #[must_use]
    pub fn new(caller_key: String) -> Self {
        Self {
            caller_key,
            steps: vec![],
        }
    }

    /// returns the [Id] of the added [Step]
    pub fn add_step<S>(&mut self, step: S) -> Id
    where
        S: Into<StepTODO>,
    {
        self.steps.push(step.into());
        Id::from(self.steps.len() - 1)
    }
}

#[derive(Debug, Serialize, Deserialize)]
pub struct PlanResponse {
    /// The numeric id of the step.
    pub id: u32,
    /// The result of the plan.
    pub result: PlanResult,
    /// An optional error message.
    pub error: Option<String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct PlanResult {
    /// The ID created from the program execution.
    pub id: Option<String>,
    /// An optional message.
    pub msg: Option<String>,
    /// The timestamp of the function call response.
    pub timestamp: u64,
    /// The result of the function call.
    pub response: Option<String>,
}

#[derive(Error, Debug)]
pub enum ClientError {
    #[error("Read error: {0}")]
    Read(#[from] std::io::Error),
    #[error("Deserialization error: {0}")]
    Deserialization(#[from] serde_json::Error),
    #[error("EOF")]
    Eof,
    #[error("Missing handle")]
    StdIo,
}

/// A [Client] is required to pass a [Plan] to the simulator, then to [run](Self::run_plan) the actual simulation.
pub struct Client {
    /// Path to the simulator binary
    path: &'static str,
}

impl Client {
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

    /// Runs a [Plan] against the simulator and returns vec of result.
    /// # Errors
    ///
    /// Returns an error if the serialization or plan fails.
<<<<<<< Updated upstream
    pub fn run_plan(self, plan: Plan) -> Result<Vec<PlanResponse>, ClientError> {
        let Child {
            mut stdin,
            mut stdout,
            ..
        } = Command::new(self.path)
            .arg("interpreter")
            .arg("--cleanup")
            .arg("--log-level")
            .arg("debug")
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .spawn()?;
=======
    pub fn run_plan(self, plan: Plan) -> Result<Vec<PlanResponse>, Box<dyn Error>> {
        let mut child =
            SimulatorChild::<ChildStdin, Lines<BufReader<ChildStdout>>>::new(self.path)?;
>>>>>>> Stashed changes

        let writer = stdin.as_mut().ok_or(ClientError::StdIo)?;
        let reader = stdout.as_mut().ok_or(ClientError::StdIo)?;

        let responses = BufReader::new(reader)
            .lines()
            .map(|line| serde_json::from_str(&line?).map_err(Into::into));

        let mut child = SimulatorChild { writer, responses };

        plan.steps
            .iter()
            .map(|step| child.run_step(&plan.caller_key, step))
            .collect()
    }
}

impl Default for Client {
    fn default() -> Self {
        Self::new()
    }
}

<<<<<<< Updated upstream
struct SimulatorChild<W, R> {
    writer: W,
    responses: R,
}

impl<W, R> SimulatorChild<W, R>
where
    W: Write,
    R: Iterator<Item = Result<PlanResponse, ClientError>>,
{
    fn run_step(&mut self, caller_key: &str, step: &StepTODO) -> Result<PlanResponse, ClientError> {
        let run_command = b"run --step '";
        self.writer.write_all(run_command)?;
=======
struct SimulatorStd {
    // reader:
}

impl Iterator for SimulatorStd {
    type Item = Result<PlanResponse, Box<dyn Error>>;

    fn next(&mut self) -> Option<Self::Item> {
        todo!()
    }
}

impl SimulatorStd {
    fn new(path: &str) -> Result<Self, Box<dyn Error>> {
        let child = Command::new(path)
            .arg("interpreter")
            .arg("--cleanup")
            .arg("--log-level")
            .arg("error")
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .spawn()?;

        let (Some(stdin), Some(stdout)) = (child.stdin, child.stdout) else {
            panic!("could not attach to the child process stdio");
        };
        let lines = BufReader::new(stdout).lines();

        let mut sim = Self { stdin, lines };

        let _ = sim.read_response()?; // intepreter ready

        Ok(sim)
    }
}

struct SimulatorChild<W, R>
where
    W: std::io::Write,
    R: Iterator<Item = Result<PlanResponse, Box<dyn Error>>>,
{
    writer: W,
    responses: R,
}

impl<W, R> SimulatorChild<W, R>
where
    W: std::io::Write,
    R: Iterator<Item = Result<PlanResponse, Box<dyn Error>>>,
{
    fn new(writer: W, responses: R) -> Result<Self, Box<dyn Error>> {
        Ok(SimulatorChild { writer, responses })
    }

    fn run_step(
        &mut self,
        caller_key: &String,
        step: &Step,
    ) -> Result<PlanResponse, Box<dyn Error>> {
        let run_command = "run --stdin\n";
        self.write_stdin(run_command.as_bytes())?;
        let _ = self.read_response()?;
>>>>>>> Stashed changes

        let step = SimulatorStep { caller_key, step };
        let input = serde_json::to_vec(&step)?;
        self.writer.write_all(&input)?;
        self.writer.write_all(b"'\n")?;
        self.writer.flush()?;

        let resp = self.responses.next().ok_or(ClientError::Eof)??;

        Ok(resp)
    }
<<<<<<< Updated upstream
=======

    fn read_response(&mut self) -> Result<PlanResponse, Box<dyn Error>> {
        let text: String = self
            .responses
            .next()
            .ok_or("EOF")?
            .map_err(|e| format!("failed to read from stdout: {e}"))?;

        let resp = serde_json::from_str(&text)
            .map_err(|e| format!("failed to parse output to json: {e}"))?;

        Ok(resp)
    }

    fn write_stdin(&mut self, bytes: &[u8]) -> Result<(), String> {
        self.writer
            .write_all(bytes)
            .map_err(|e| format!("failed to write to stdin: {e}"))?;
        self.writer
            .flush()
            .map_err(|e| format!("failed to flush from stdin: {e}"))?;

        Ok(())
    }
>>>>>>> Stashed changes
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

        let output_param: Param = serde_json::from_value(dbg!(expected_json)).unwrap();

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
