//! A client and types for the VM simulator. This feature allows for Rust
//! developers to construct tests for their programs completely in Rust.
//! Alternatively the `Plan` can be written in YAML/JSON and passed to the
//! Simulator binary directly.

use std::{
    error::Error,
    io::Write,
    process::{Command, Stdio},
};

use serde::{Deserialize, Serialize};

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
pub struct Step<'a> {
    /// A description of the step.
    description: &'a str,
    /// The API endpoint to call.
    endpoint: Endpoint,
    /// The method to call on the endpoint.
    method: &'a str,
    /// The parameters to pass to the method.
    params: Vec<Param<'a>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    require: Option<Require>,
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
pub struct Param<'a> {
    /// The optional name of the parameter. This is only used for readability.
    name: &'a str,
    #[serde(rename = "type")]
    /// The type of the parameter.
    param_type: ParamType,
    /// The value of the parameter.
    value: &'a str,
}

#[derive(Debug, Serialize, Deserialize, PartialEq)]
pub struct Require {
    /// If defined the result of the step must match this assertion.
    result: ResultAssertion,
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
    operator: Operator,
    /// The value to compare against.
    value: String,
}

#[derive(Debug, Serialize, Deserialize, PartialEq)]
pub struct Plan<'a> {
    /// The name of the plan.
    name: &'a str,
    /// A description of the plan.
    description: &'a str,
    /// The key of the caller used in each step of the plan.
    caller_key: &'a str,
    /// The steps to perform in the plan.
    steps: Vec<Step<'a>>,
}

pub struct Client {
    /// Path to the simulator binary
    path: String,
}

impl Client {
    #[must_use]
    pub fn new(path: String) -> Self {
        Self { path }
    }

    /// Runs a `Plan` against the simulator and returns the result.
    /// # Errors
    ///
    /// Returns an error if the if serialization or plan fails.
    pub fn run<T>(&self, plan: &Plan) -> Result<T, Box<dyn Error>>
    where
        T: serde::de::DeserializeOwned + serde::Serialize,
    {
        call_run_stdin(&self.path, plan)
    }

    /// Performs a `ReadOnly` step against the simulator and returns the result.
    /// # Errors
    ///
    /// Returns an error if the if serialization or plan fails.
    pub fn read_only<T>(&self, data: Step, key: &str) -> Result<T, Box<dyn Error>>
    where
        T: serde::de::DeserializeOwned + serde::Serialize,
    {
        let plan = &Plan {
            name: "view",
            description: "single view request",
            caller_key: key,
            steps: vec![data],
        };

        call_run_stdin(&self.path, plan)
    }

    /// Performs a single `Execute` step against the simulator and returns the result.
    /// # Errors
    ///
    /// Returns an error if the if serialization or plan fails.
    pub fn execute<T>(&self, data: Step, key: &str) -> Result<T, Box<dyn Error>>
    where
        T: serde::de::DeserializeOwned + serde::Serialize,
    {
        let plan = &Plan {
            name: "execute",
            description: "single execution request",
            caller_key: key,
            steps: vec![data],
        };

        call_run_stdin(&self.path, plan)
    }
}

fn call_run_stdin<T>(path: &str, plan: &Plan) -> Result<T, Box<dyn Error>>
where
    T: serde::de::DeserializeOwned + serde::Serialize,
{
    let mut child = Command::new(path)
        .arg("run")
        .stdin(Stdio::piped())
        .spawn()?;

    // write json to stdin
    let input =
        serde_json::to_string(plan).map_err(|e| format!("failed to serialize json: {e}"))?;
    if let Some(ref mut stdin) = child.stdin {
        stdin
            .write_all(input.as_bytes())
            .map_err(|e| format!("failed to write to stdin: {e}"))?;
    }

    let output = child
        .wait_with_output()
        .map_err(|e| format!("failed to wait for command to finish: {e}"))?;

    let resp: T = serde_json::from_str(String::from_utf8(output.stdout)?.as_ref())
        .map_err(|e| format!("failed to parse output to json: {e}"))?;

    Ok(resp)
}

// TODO: make this test simpler
#[cfg(test)]
mod tests {
    use super::*;

    fn parse_yaml<'a>(yaml_content: &'a str) -> Result<Plan<'a>, Box<dyn std::error::Error>> {
        let plan: Plan<'a> = serde_yaml::from_str(yaml_content)?;
        Ok(plan)
    }

    #[test]
    fn test_parse_plan_yaml() {
        let yaml_content = r#"
name: token program
description: Deploy and execute token program
caller_key: alice_key
steps:
  - description: create bob key
    endpoint: key
    method: create
    params:
      - name: key name
        type: ed25519
        value: bob_key
  - description: mint 1000 tokens to alice
    endpoint: execute
    method: mint_to
    params:
      - name: program_id
        type: id
        value: 2Ej3Qp6aUZ7yBnqZxBmvvvekUiriCn4ftcqY8VKGwMu5CmZiz
      - name: max_fee
        type: u64
        value: 100000
      - name: owner
        type: ed25519
        value: alice_key
      - name: amount
        type: u64
        value: 1000
  - description: get balance for alice
    endpoint: readonly
    method: get_balance
    params:
      - name: program_id
        type: id
        value: 2Ej3Qp6aUZ7yBnqZxBmvvvekUiriCn4ftcqY8VKGwMu5CmZiz
      - name: owner
        type: ed25519
        value: alice_key
    require:
        result:
            operator: ==
            value: 1000
"#;

        let expected = Plan {
            name: "token program",
            description: "Deploy and execute token program",
            caller_key: "alice_key",
            steps: vec![
                Step {
                    description: "create bob key",
                    endpoint: Endpoint::Key,
                    method: "create",
                    params: vec![Param {
                        name: "key name",
                        param_type: ParamType::Key(Key::Ed25519),
                        value: "bob_key",
                    }],
                    require: None,
                },
                Step {
                    description: "mint 1000 tokens to alice",
                    endpoint: Endpoint::Execute,
                    method: "mint_to",
                    params: vec![
                        Param {
                            name: "program_id",
                            param_type: ParamType::Id,
                            value: "2Ej3Qp6aUZ7yBnqZxBmvvvekUiriCn4ftcqY8VKGwMu5CmZiz",
                        },
                        Param {
                            name: "max_fee",
                            param_type: ParamType::U64,
                            value: "100000",
                        },
                        Param {
                            name: "owner",
                            param_type: ParamType::Key(Key::Ed25519),
                            value: "alice_key",
                        },
                        Param {
                            name: "amount",
                            param_type: ParamType::U64,
                            value: "1000",
                        },
                    ],
                    require: None,
                },
                Step {
                    description: "get balance for alice",
                    endpoint: Endpoint::ReadOnly,
                    method: "get_balance",
                    params: vec![
                        Param {
                            name: "program_id",
                            param_type: ParamType::Id,
                            value: "2Ej3Qp6aUZ7yBnqZxBmvvvekUiriCn4ftcqY8VKGwMu5CmZiz",
                        },
                        Param {
                            name: "owner",
                            param_type: ParamType::Key(Key::Ed25519),
                            value: "alice_key",
                        },
                    ],
                    require: Some(Require {
                        result: ResultAssertion {
                            operator: Operator::NumericEq,
                            value: "1000".into(),
                        },
                    }),
                },
            ],
        };

        assert_eq!(parse_yaml(yaml_content).unwrap(), expected);
    }
}
