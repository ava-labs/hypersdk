//! A client and types for the VM simulator.

use std::{
    error::Error,
    io::Write,
    process::{Command, Stdio},
};

use serde::{Deserialize, Deserializer, Serialize, Serializer};

#[derive(Debug, Serialize, Deserialize, PartialEq)]
#[serde(rename_all = "lowercase")]
pub enum Endpoint {
    /// Perform an operation against the key api.
    Key,
    /// Make a read-only call to a program function and return the result.
    View,
    /// Create a transaction from a possible state changing program function
    /// call. A program's function can internally optionally call other
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
    #[serde(default)]
    require: Option<Require>,
}

#[derive(Debug, PartialEq)]
pub enum ParamType {
    U64,
    String,
    Key(Key),
    ID,
}

#[derive(Debug, Serialize, Deserialize, PartialEq)]
#[serde(rename_all = "lowercase")]
pub enum Key {
    Ed25519,
    Secp256r1,
}

#[derive(Debug, Serialize, Deserialize, PartialEq)]
pub struct Param<'a> {
    /// The optional name of the parameter. This is used for readability.
    name: &'a str,
    #[serde(rename = "type")]
    /// The type of the parameter.
    param_type: ParamType,
    /// The value of the parameter.
    value: &'a str,
}

impl Serialize for ParamType {
    fn serialize<S>(&self, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: Serializer,
    {
        match self {
            ParamType::U64 => serializer.serialize_str("u64"),
            ParamType::String => serializer.serialize_str("string"),
            ParamType::Key(Key::Ed25519) => serializer.serialize_str("ed25519"),
            ParamType::Key(Key::Secp256r1) => serializer.serialize_str("secp256r1"),
            ParamType::ID => serializer.serialize_str("id"),
        }
    }
}

impl<'de> Deserialize<'de> for ParamType {
    fn deserialize<D>(deserializer: D) -> Result<Self, D::Error>
    where
        D: Deserializer<'de>,
    {
        use serde::de::Error;
        let s = String::deserialize(deserializer)?;
        match s.as_str() {
            "u64" => Ok(ParamType::U64),
            "string" => Ok(ParamType::String),
            "ed25519" => Ok(ParamType::Key(Key::Ed25519)),
            "secp256r1" => Ok(ParamType::Key(Key::Secp256r1)),
            "id" => Ok(ParamType::ID),
            _ => Err(D::Error::custom(format!("unknown param type: {}", s))),
        }
    }
}

#[derive(Debug, Serialize, Deserialize, PartialEq)]
pub struct Require {
    result: ResultCondition,
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
pub struct ResultCondition {
    operator: Operator,
    operand: String,
}

#[derive(Debug, Serialize, Deserialize, PartialEq)]
pub struct Simulation<'a> {
    name: &'a str,
    description: &'a str,
    caller_key: &'a str,
    steps: Vec<Step<'a>>,
}

pub struct Client {
    /// Path to the simulator binary
    path: String,
}

impl Client {
    pub fn new(path: String) -> Self {
        Self { path }
    }

    /// Runs a simulation against the simulator and returns the result.
    pub fn run<T>(&self, simulation: &Simulation) -> Result<T, Box<dyn Error>>
    where
        T: serde::de::DeserializeOwned + serde::Serialize,
    {
        call_run_stdin(&self.path, simulation)
    }

    /// Performs a view step against the simulator and returns the result.
    pub fn view<T>(&self, data: Step, key: &str) -> Result<T, Box<dyn Error>>
    where
        T: serde::de::DeserializeOwned + serde::Serialize,
    {
        let simulation = &Simulation {
            name: "view",
            description: "single view request",
            caller_key: key,
            steps: vec![data],
        };

        call_run_stdin(&self.path, simulation)
    }

    /// Performs a single execution step against the simulator and returns the result.
    pub fn execute<T>(&self, data: Step, key: &str) -> Result<T, Box<dyn Error>>
    where
        T: serde::de::DeserializeOwned + serde::Serialize,
    {
        let simulation = &Simulation {
            name: "execute",
            description: "single execution request",
            caller_key: key,
            steps: vec![data],
        };

        call_run_stdin(&self.path, simulation)
    }
}

fn call_run_stdin<T>(path: &str, simulation: &Simulation) -> Result<T, Box<dyn Error>>
where
    T: serde::de::DeserializeOwned + serde::Serialize,
{
    let mut child = Command::new(path)
        .arg("run")
        .stdin(Stdio::piped())
        .spawn()?;

    // write json to stdin
    let input =
        serde_json::to_string(simulation).map_err(|e| format!("failed to serialize json: {e}"))?;
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

#[cfg(test)]
mod tests {
    use super::*;

    fn parse_yaml<'a>(yaml_content: &'a str) -> Result<Simulation<'a>, Box<dyn std::error::Error>> {
        let sim: Simulation<'a> = serde_yaml::from_str(yaml_content)?;
        Ok(sim)
    }

    #[test]
    fn test_parse_key_yaml() {
        let yaml_content = "
name: token program
description: Deploy and execute token program
caller_key: alice_key
steps:
  - description: create alice key
    endpoint: key
    method: create
    params:
      - name: key name
        type: ed25519
        value: alice_key
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
    endpoint: view
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
            operand: 1000
";

        let expected = Simulation {
            name: "token program",
            description: "Deploy and execute token program",
            caller_key: "alice_key",
            steps: vec![
                Step {
                    description: "create alice key",
                    endpoint: Endpoint::Key,
                    method: "create",
                    params: vec![Param {
                        name: "key name",
                        param_type: ParamType::Key(Key::Ed25519),
                        value: "alice_key",
                    }],
                    require: None,
                },
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
                            param_type: ParamType::ID,
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
                    endpoint: Endpoint::View,
                    method: "get_balance",
                    params: vec![
                        Param {
                            name: "program_id",
                            param_type: ParamType::ID,
                            value: "2Ej3Qp6aUZ7yBnqZxBmvvvekUiriCn4ftcqY8VKGwMu5CmZiz",
                        },
                        Param {
                            name: "owner",
                            param_type: ParamType::Key(Key::Ed25519),
                            value: "alice_key",
                        },
                    ],
                    require: Some(Require {
                        result: ResultCondition {
                            operator: Operator::NumericEq,
                            operand: "1000".into(),
                        },
                    }),
                },
            ],
        };

        assert_eq!(parse_yaml(yaml_content).unwrap(), expected);
    }
}
