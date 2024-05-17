use wasmlanche_sdk::params::Param;
use wasmlanche_sdk::{public, Context};

#[public]
pub fn get_value(_: Context) -> i64 {
    0
}