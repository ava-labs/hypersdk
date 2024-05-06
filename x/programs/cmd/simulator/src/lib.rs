//! A client and types for the VM simulator. This crate allows for Rust
//! developers to construct tests for their programs completely in Rust.
//! Alternatively the `Plan` can be written in JSON and passed to the
//! Simulator binary directly.

use serde::{Deserialize, Serialize};
use serde_with::{serde_as, DisplayFromStr};
use std::{
    error::Error,
    ffi::OsStr,
    io::{Read, Write},
    path::Path,
    process::{ChildStdin, ChildStdout, Command, Output, Stdio},
    str::FromStr,
    time::Duration,
};

mod id;

pub use id::Id;

/// The endpoint to call for a [Step].
#[derive(Debug, Serialize, Deserialize, PartialEq, Clone)]
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
#[derive(Debug, Serialize, Deserialize, PartialEq, Clone)]
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
    /// If defined the result of the step must match this assertion.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub require: Option<Require>,
}

#[derive(Debug, Serialize, Deserialize, PartialEq, Clone)]
#[serde(rename_all = "camelCase")]
pub struct Step2 {
    /// The key of the caller used in each step of the plan.
    pub caller_key: String,
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

#[derive(Debug, Serialize, Deserialize, PartialEq, Clone)]
pub struct Require {
    /// If defined the result of the step must match this assertion.
    pub result: ResultAssertion,
}

#[serde_as]
#[derive(Debug, Serialize, Deserialize, PartialEq, Clone)]
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

#[derive(Debug, Serialize, Deserialize, PartialEq)]
pub struct Plan {
    /// The key of the caller used in each step of the plan.
    pub caller_key: String,
    /// The steps to perform in the plan.
    pub steps: Vec<Step>,
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
    pub fn add_step(&mut self, step: Step) -> Id {
        self.steps.push(step);
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
    pub response: Option<Vec<i64>>,
}

/// A [Client] is required to pass a [Plan] to the simulator, then to [run](Self::run_plan) the actual simulation.
pub struct Client {
    /// Path to the simulator binary
    path: &'static str,
}

impl Client {
    #[must_use]
    pub fn new() -> Result<Self, Box<dyn Error>> {
        let path = env!("SIMULATOR_PATH");

        if !Path::new(path).exists() {
            eprintln!();
            eprintln!("Simulator binary not found at path: {path}");
            eprintln!();
            eprintln!("Please run `cargo clean -p simulator` and rebuild your dependent crate.");
            eprintln!();

            panic!("Simulator binary not found, must rebuild simulator");
        }

        Ok(Self { path })
    }

    /// Runs a [Plan] against the simulator and returns vec of result.
    /// # Errors
    ///
    /// Returns an error if the serialization or plan fails.
    pub fn run_plan(&self, plan: &Plan) -> Result<Vec<PlanResponse>, Box<dyn Error>> {
        let child = Command::new(self.path)
            .arg("interpreter")
            .arg("--cleanup")
            .arg("--log-level")
            .arg("error") // TODO control this through an env variable
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .spawn()?;

        let stdin = &mut child.stdin.unwrap();
        let stdout = &mut child.stdout.unwrap();

        run_steps(stdin, stdout, plan)
    }

    // TODO use this function instead !
    /*
    /// Performs a single `Execute` step against the simulator and returns the result.
    /// # Errors
    ///
    /// Returns an error if the serialization or single-[Step]-[Plan] fails.
    /*pub fn execute_step(&self, key: &str, step: Step) -> Result<PlanResponse, Box<dyn Error>> {
        let plan = &Plan {
            caller_key: key.into(),
            steps: vec![step],
        };

        run_step(self.path, plan)
    }*/*/
}

fn cmd_output<P>(path: P, plan: &Plan) -> Result<Output, Box<dyn Error>>
where
    P: AsRef<OsStr>,
{
    let mut child = Command::new(path)
        .arg("run")
        .arg("--cleanup")
        .arg("--stdin")
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .spawn()?;

    // write json to stdin
    let input =
        serde_json::to_string(plan).map_err(|e| format!("failed to serialize json: {e}"))?;
    if let Some(ref mut stdin) = child.stdin {
        stdin
            .write_all(input.as_bytes())
            .map_err(|e| format!("failed to write to stdin: {e}"))?;
    }

    child
        .wait_with_output()
        .map_err(|e| format!("failed to wait for child: {e}").into())
}

fn run_steps<T>(
    stdin: &mut ChildStdin,
    stdout: &mut ChildStdout,
    plan: &Plan,
) -> Result<Vec<T>, Box<dyn Error>>
where
    T: serde::de::DeserializeOwned + serde::Serialize,
{
    let mut items: Vec<T> = Vec::new();
    for step in &plan.steps {
        let resp = run_step::<T>(
            stdin,
            stdout,
            &Plan {
                caller_key: plan.caller_key.clone(),
                steps: vec![step.clone()],
            },
        )?;
        items.push(resp);
    }
    Ok(items)
}

fn run_step<T>(
    stdin: &mut ChildStdin,
    stdout: &mut ChildStdout,
    plan: &Plan,
) -> Result<T, Box<dyn Error>>
where
    T: serde::de::DeserializeOwned + serde::Serialize,
{
    let run_command = "run --stdin\n".to_string();
    stdin
        .write_all(run_command.as_bytes())
        .map_err(|e| format!("failed to write to stdin: {e}"))?;
    stdin
        .flush()
        .map_err(|e| format!("failed to flush from stdin: {e}"))?;

    // TODO write function
    let buf: Vec<u8> = stdout
        .bytes()
        .take_while(|char| char.as_ref().is_ok_and(|char| *char != b'\n'))
        .collect::<Result<_, _>>()
        .map_err(|e| format!("failed to read from stdout: {e}"))?;

    let _resp: T = serde_json::from_str(String::from_utf8(buf)?.as_ref())
        .map_err(|e| format!("failed to parse output to json: {e}"))?;

    let step = plan.steps.first().unwrap().clone();
    let step = Step2 {
        caller_key: plan.caller_key.clone(),
        endpoint: step.endpoint,
        method: step.method,
        max_units: step.max_units,
        params: step.params,
        require: step.require,
    };
    let mut input =
        dbg!(serde_json::to_string(&step).map_err(|e| format!("failed to serialize json: {e}"))?);
    input.push('\n');
    stdin
        .write_all(input.as_bytes())
        .map_err(|e| format!("failed to write to stdin: {e}"))?;
    stdin
        .flush()
        .map_err(|e| format!("failed to flush from stdin: {e}"))?;

    // TODO write function
    let buf: Vec<u8> = stdout
        .bytes()
        .take_while(|char| char.as_ref().is_ok_and(|char| *char != b'\n'))
        .collect::<Result<_, _>>()
        .map_err(|e| format!("failed to read from stdout: {e}"))?;

    let resp: T = serde_json::from_str(String::from_utf8(buf)?.as_ref())
        .map_err(|e| format!("failed to parse output to json: {e}"))?;

    Ok(resp)
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
