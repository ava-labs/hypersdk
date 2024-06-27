// use borsh::BorshSerialize;
// use wasmlanche_sdk::{public, types::Address, types::Gas, Context};

// #[derive(BorshSerialize)]
// pub struct ComplexReturn {
//     account: Address,
//     max_units: Gas,
// }

// #[public]
// pub fn get_value(ctx: Context) -> ComplexReturn {
//     let account = *ctx.program().account();
//     ComplexReturn {
//         account,
//         max_units: 1000,
//     }
// }
