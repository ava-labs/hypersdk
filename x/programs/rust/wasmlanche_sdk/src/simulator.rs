//! A client and types for the VM simulator. This feature allows for Rust
//! developers to construct tests for their programs completely in Rust.
//! Alternatively the `Plan` can be written in JSON and passed to the
//! Simulator binary directly.

use std::{
    error::Error,
    ffi::OsStr,
    io::Write,
    process::{Command, Output, Stdio},
};

pub const PATH_KEY: &str = "SIMULATOR_PATH";

use serde::{Deserialize, Serialize};

/// Converts the step index to a string identifier. This is used to populate Ids
/// created in previous inline plan steps.
#[must_use]
pub fn id_from_step(i: usize) -> String {
    format!("step_{i}")
}

#[derive(Debug, Serialize, Deserialize, PartialEq)]
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

#[derive(Debug, Serialize, Deserialize, PartialEq)]
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
    #[serde(skip_serializing_if = "Option::is_none")]
    pub require: Option<Require>,
}

#[derive(Debug, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ParamType {
    U64,
    String,
    Id,
    #[serde(untagged)]
    Key(Key),
}

#[derive(Debug, Serialize, Deserialize, PartialEq)]
#[serde(rename_all = "lowercase")]
pub enum Key {
    Ed25519,
    Secp256r1,
}

#[derive(Debug, Serialize, Deserialize, PartialEq)]
pub struct Param {
    #[serde(rename = "type")]
    /// The type of the parameter.
    pub param_type: ParamType,
    /// The value of the parameter.
    pub value: String,
}

impl Param {
    #[must_use]
    pub fn new(param_type: ParamType, value: &str) -> Self {
        Self {
            param_type,
            value: value.into(),
        }
    }
}

#[derive(Debug, Serialize, Deserialize, PartialEq)]
pub struct Require {
    /// If defined the result of the step must match this assertion.
    pub result: ResultAssertion,
}

#[derive(Debug, Serialize, Deserialize, PartialEq)]
pub enum Operator {
    #[serde(rename = "==")]
    NumericEq,
    #[serde(rename = "!=")]
    NumericNe,
    #[serde(rename = ">")]
    NumericGt,
    #[serde(rename = "<")]
    NumericLt,
    #[serde(rename = ">=")]
    NumericGe,
    #[serde(rename = "<=")]
    NumericLe,
}

#[derive(Debug, Serialize, Deserialize, PartialEq)]
pub struct ResultAssertion {
    /// The operator to use for the assertion.
    pub operator: Operator,
    /// The value to compare against.
    pub value: String,
}

#[derive(Debug, Serialize, Deserialize, PartialEq)]
pub struct Plan {
    /// The key of the caller used in each step of the plan.
    pub caller_key: String,
    /// The steps to perform in the plan.
    pub steps: Vec<Step>,
}

impl Plan {
    #[must_use]
    pub fn new(caller_key: &str) -> Self {
        Self {
            caller_key: caller_key.into(),
            steps: vec![],
        }
    }

    pub fn add_step(&mut self, step: Step) {
        self.steps.push(step);
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
    pub response: Option<Vec<u64>>,
}

pub struct Client<P> {
    /// Path to the simulator binary
    path: P,
}

impl<P> Client<P>
where
    P: AsRef<OsStr>,
{
    #[must_use]
    pub fn new(path: P) -> Self {
        Self { path }
    }

    /// Runs a `Plan` against the simulator and returns vec of result.
    /// # Errors
    ///
    /// Returns an error if the if serialization or plan fails.
    pub fn run<T>(&self, plan: &Plan) -> Result<Vec<T>, Box<dyn Error>>
    where
        T: serde::de::DeserializeOwned + serde::Serialize,
    {
        run_steps(&self.path, plan)
    }

    /// Performs a `ReadOnly` step against the simulator and returns the result.
    /// # Errors
    ///
    /// Returns an error if the if serialization or plan fails.
    pub fn read_only<T>(
        &self,
        key: &str,
        method: &str,
        params: Vec<Param>,
        require: Option<Require>,
    ) -> Result<T, Box<dyn Error>>
    where
        T: serde::de::DeserializeOwned + serde::Serialize,
    {
        let step = Step {
            endpoint: Endpoint::ReadOnly,
            method: method.into(),
            max_units: 0,
            params,
            require,
        };
        let plan = &Plan {
            caller_key: key.into(),
            steps: vec![step],
        };

        run_step(&self.path, plan)
    }

    /// Performs a single `Execute` step against the simulator and returns the result.
    /// # Errors
    ///
    /// Returns an error if the if serialization or plan fails.
    pub fn execute<T>(&self, step: Step, key: &str) -> Result<T, Box<dyn Error>>
    where
        T: serde::de::DeserializeOwned + serde::Serialize,
    {
        let plan = &Plan {
            caller_key: key.into(),
            steps: vec![step],
        };

        run_step(&self.path, plan)
    }

    /// Creates a key in a single step.
    /// # Errors
    ///
    /// Returns an error if the if serialization or plan fails.
    pub fn key_create<T>(&self, name: &str, key_type: Key) -> Result<T, Box<dyn Error>>
    where
        T: serde::de::DeserializeOwned + serde::Serialize,
    {
        let plan = &Plan {
            caller_key: name.into(),
            steps: vec![Step {
                endpoint: Endpoint::Key,
                method: "create_key".into(),
                max_units: 0,
                params: vec![Param {
                    value: name.into(),
                    param_type: ParamType::Key(key_type),
                }],
                require: None,
            }],
        };

        run_step(&self.path, plan)
    }

    /// Creates a program in a single step.
    /// # Errors
    ///
    /// Returns an error if the if serialization or plan fails.
    pub fn program_create<T>(&self, key: &str, path: &str) -> Result<T, Box<dyn Error>>
    where
        T: serde::de::DeserializeOwned + serde::Serialize,
    {
        let plan = &Plan {
            caller_key: key.into(),
            steps: vec![Step {
                endpoint: Endpoint::Execute,
                method: "program_create".into(),
                max_units: 0,
                params: vec![Param::new(ParamType::String, path)],
                require: None,
            }],
        };

        run_step(&self.path, plan)
    }
}

fn cmd_output<P>(path: P, plan: &Plan) -> Result<Output, Box<dyn Error>>
where
    P: AsRef<OsStr>,
{
    let mut child = Command::new(path)
        .arg("run")
        .arg("-")
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

fn run_steps<P, T>(path: P, plan: &Plan) -> Result<Vec<T>, Box<dyn Error>>
where
    T: serde::de::DeserializeOwned + serde::Serialize,
    P: AsRef<OsStr>,
{
    let output = cmd_output(path, plan)?;
    let mut items: Vec<T> = Vec::new();

    if !output.status.success() {
        return Err(String::from_utf8(output.stdout)?.into());
    }

    for line in String::from_utf8(output.stdout)?
        .lines()
        .filter(|line| !line.trim().is_empty())
    {
        let item = serde_json::from_str(line)
            .map_err(|e| format!("failed to parse output to json: {e}"))?;
        items.push(item);
    }

    Ok(items)
}

fn run_step<P, T>(path: P, plan: &Plan) -> Result<T, Box<dyn Error>>
where
    T: serde::de::DeserializeOwned + serde::Serialize,
    P: AsRef<OsStr>,
{
    let output = cmd_output(path, plan)?;

    if !output.status.success() {
        return Err(String::from_utf8(output.stdout)?.into());
    }

    let resp: T = serde_json::from_str(String::from_utf8(output.stdout)?.as_ref())
        .map_err(|e| format!("failed to parse output to json: {e}"))?;

    Ok(resp)
}
