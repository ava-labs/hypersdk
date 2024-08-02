#![allow(non_upper_case_globals)]
#![allow(non_camel_case_types)]
#![allow(non_snake_case)]
// include the generated bindings
// reference: https://rust-lang.github.io/rust-bindgen/tutorial-3.html
include!(concat!(env!("OUT_DIR"), "/bindings.rs"));


mod simulator;
mod state;
mod types;

pub use simulator::Simulator;
