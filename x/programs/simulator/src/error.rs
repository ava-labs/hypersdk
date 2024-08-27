// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use std::{
    error::Error,
    fmt::{Debug, Display},
    str::Utf8Error,
};

#[derive(thiserror::Error, Debug)]
pub enum SimulatorError {
    #[error("Error across the FFI boundary: {0}")]
    Ffi(#[from] Utf8Error),
    #[error(transparent)]
    Serialization(#[from] wasmlanche_sdk::borsh::io::Error),
    #[error(transparent)]
    ExternalCall(#[from] ExternalCallError),
    #[error("Error during program creation")]
    CreateProgram(String),
    #[error("Error during program execution")]
    CallProgram(String),
}

pub struct ExternalCallError(wasmlanche_sdk::ExternalCallError);

impl From<wasmlanche_sdk::ExternalCallError> for ExternalCallError {
    fn from(e: wasmlanche_sdk::ExternalCallError) -> Self {
        Self(e)
    }
}

impl Display for ExternalCallError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        Display::fmt(&self.0, f)
    }
}

impl Debug for ExternalCallError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        Debug::fmt(&self.0, f)
    }
}

impl Error for ExternalCallError {}
