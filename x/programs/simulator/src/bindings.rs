// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

#![allow(non_upper_case_globals)]
#![allow(non_camel_case_types)]
#![allow(non_snake_case)]
#![allow(dead_code)]

use std::ops::Deref;

// include the generated bindings
// reference: https://rust-lang.github.io/rust-bindgen/tutorial-3.html
include!(concat!(env!("OUT_DIR"), "/bindings.rs"));

impl std::ops::Deref for Bytes {
    type Target = [u8];

    fn deref(&self) -> &Self::Target {
        // # Safety:
        // These bytes were allocated by CGo
        // They are guaranteed to be valid for the length of the slice
        unsafe { std::slice::from_raw_parts(self.data, self.length) }
    }
}

impl From<Bytes> for Box<[u8]> {
    fn from(value: Bytes) -> Self {
        Self::from(value.deref())
    }
}

impl From<&[u8]> for Bytes {
    fn from(value: &[u8]) -> Self {
        Bytes {
            data: value.as_ptr(),
            length: value.len(),
        }
    }
}

impl Default for Bytes {
    fn default() -> Self {
        <&[u8] as Default>::default().into()
    }
}
