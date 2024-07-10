// use wasmlanche_sdk::{public, state_keys, types::Address, Context, Program};

// #[state_keys]
// pub enum StateKeys {
//     // Internal accounting
//     TokenX,
//     TokenY,

//     // Constant product invariant
//     K,
//     // total LP shares
//     TotalSupply,
//     // balances of LP shares
//     Balances(Address),

//     TotalySupply,
// }


// // increases the amount of LP shares in circulation
// fn mint_lp(context: Context<StateKeys>, amount: u64) {
//     let program = context.program();
//     let total_supply = total_supply(program);

//     program
//         .state()
//         .store_by_key(StateKeys::TotalSupply, &(total_supply + amount))
//         .unwrap();
// }

// // decreases the amount of LP shares in circulation
// fn burn_lp(context: Context<StateKeys>, amount: u64) {
//     let program = context.program();
//     let total_supply = total_supply(program);

//     assert!(total_supply >= amount, "not enough shares to burn");
    
//     program
//         .state()
//         .store_by_key(StateKeys::TotalSupply, &(total_supply - amount))
//         .unwrap();
// }

// #[public]
// pub fn swap(context: Context<StateKeys>, amount_in: u64, x_to_y: bool) -> u64 {
   
// }

// #[public]
// pub fn add_liquidity(context: Context<StateKeys>, amount_x: u64, amount_y: u64) -> u64 {
   
// }

// #[public]
// pub fn remove_liquidity(context: Context<StateKeys>, shares: u64) -> (u64, u64) {
   
// }


// fn total_supply(program: &Program<StateKeys>) -> u64 {
//     program
//         .state()
//         .get(StateKeys::TotalSupply)
//         .unwrap()
//         .unwrap_or_default()
// }

// fn reserves(program: &Program<StateKeys>) -> (u64, u64) {
//     (
//         program
//             .state()
//             .get(StateKeys::ReserveX)
//             .unwrap()
//             .unwrap_or_default(),
//         program
//             .state()
//             .get(StateKeys::ReserveY)
//             .unwrap()
//             .unwrap_or_default(),
//     )
// }

// #[cfg(test)]
// mod tests {
//     use simulator::{Endpoint, Key, Step, StepResponseError, TestContext};
//     use wasmlanche_sdk::ExternalCallError;

//     const PROGRAM_PATH: &str = env!("PROGRAM_PATH");

//     #[test]
//     fn init_state() {
//         let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

//         let owner = String::from("owner");

//         let program_id = simulator
//             .run_step(&Step::create_program(PROGRAM_PATH))
//             .unwrap()
//             .id;

//         simulator
//             .run_step(&Step::create_key(Key::Ed25519(owner)))
//             .unwrap();

//         let test_context = TestContext::from(program_id);

//         let resp_err = simulator
//             .run_step(&Step {
//                 endpoint: Endpoint::Execute,
//                 method: "remove_liquidity".to_string(),
//                 max_units: u64::MAX,
//                 params: vec![test_context.clone().into(), 100000u64.into()],
//             })
//             .unwrap()
//             .result
//             .response::<(u64, u64)>()
//             .unwrap_err();

//         let StepResponseError::ExternalCall(call_err) = resp_err else {
//             panic!("wrong error returned");
//         };

//         assert!(matches!(call_err, ExternalCallError::CallPanicked));

//         let resp_err = simulator
//             .run_step(&Step {
//                 endpoint: Endpoint::Execute,
//                 method: "swap".to_string(),
//                 max_units: u64::MAX,
//                 params: vec![test_context.into(), 100000u64.into(), true.into()],
//             })
//             .unwrap()
//             .result
//             .response::<u64>()
//             .unwrap_err();

//         let StepResponseError::ExternalCall(call_err) = resp_err else {
//             panic!("wrong error returned");
//         };

//         assert!(matches!(call_err, ExternalCallError::CallPanicked));
//     }

//     #[test]
//     fn add_liquidity_same_ratio() {
//         let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

//         let owner = String::from("owner");

//         let program_id = simulator
//             .run_step(&Step::create_program(PROGRAM_PATH))
//             .unwrap()
//             .id;

//         simulator
//             .run_step(&Step::create_key(Key::Ed25519(owner)))
//             .unwrap();

//         let test_context = TestContext::from(program_id);

//         let resp = simulator
//             .run_step(&Step {
//                 endpoint: Endpoint::Execute,
//                 method: "add_liquidity".to_string(),
//                 max_units: u64::MAX,
//                 params: vec![test_context.clone().into(), 1000u64.into(), 1000u64.into()],
//             })
//             .unwrap()
//             .result
//             .response::<u64>()
//             .unwrap();

//         assert_eq!(resp, 1000);

//         let resp = simulator
//             .run_step(&Step {
//                 endpoint: Endpoint::Execute,
//                 method: "add_liquidity".to_string(),
//                 max_units: u64::MAX,
//                 params: vec![test_context.into(), 1000u64.into(), 1001u64.into()],
//             })
//             .unwrap()
//             .result
//             .response::<u64>()
//             .unwrap_err();

//         let StepResponseError::ExternalCall(call_err) = resp else {
//             panic!("unexpected error");
//         };

//         assert!(matches!(call_err, ExternalCallError::CallPanicked));
//     }

//     #[test]
//     fn swap_changes_ratio() {
//         let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

//         let owner = String::from("owner");

//         let program_id = simulator
//             .run_step(&Step::create_program(PROGRAM_PATH))
//             .unwrap()
//             .id;

//         simulator
//             .run_step(&Step::create_key(Key::Ed25519(owner)))
//             .unwrap();

//         let test_context = TestContext::from(program_id);

//         let resp = simulator
//             .run_step(&Step {
//                 endpoint: Endpoint::Execute,
//                 method: "add_liquidity".to_string(),
//                 max_units: u64::MAX,
//                 params: vec![test_context.clone().into(), 1000u64.into(), 1000u64.into()],
//             })
//             .unwrap()
//             .result
//             .response::<u64>()
//             .unwrap();

//         assert_eq!(resp, 1000);

//         simulator
//             .run_step(&Step {
//                 endpoint: Endpoint::Execute,
//                 method: "swap".to_string(),
//                 max_units: u64::MAX,
//                 params: vec![test_context.clone().into(), 10u64.into(), true.into()],
//             })
//             .unwrap();

//         let (amount_x, amount_y) = simulator
//             .run_step(&Step {
//                 endpoint: Endpoint::Execute,
//                 method: "remove_liquidity".to_string(),
//                 max_units: u64::MAX,
//                 params: vec![test_context.into(), 1000.into()],
//             })
//             .unwrap()
//             .result
//             .response::<(u64, u64)>()
//             .unwrap();

//         assert!(amount_x > 1000);
//         assert!(amount_y < 1000);
//     }
// }
