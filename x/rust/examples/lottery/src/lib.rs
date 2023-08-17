/// Counter but only for even numbers
use expose_macro::expose;
use wasmlanche_sdk::program::{Program, ProgramValue};
use wasmlanche_sdk::store::{ProgramContext, Store};
use wasmlanche_sdk::types::Address;

// Define the name of the token contract in the programs storage map.
static TOKEN_CONTRACT_NAME: &str = "token_contract";

/// Initializes the program.
#[expose]
fn init_contract() -> i64 {
    let token_contract = Program::new();
    token_contract.publish().unwrap().into()
}

/// Sets the token contract address and the lotto address. This needs to be set
/// before play can be called, otherwise there is no reference contract and address.
#[expose]
fn set(ctx: ProgramContext, counter_ctx: ProgramContext, lot_address: Address) {
    ctx.store_value(
        &TOKEN_CONTRACT_NAME,
        &ProgramValue::ProgramObject(counter_ctx),
    )
    .expect("Failed to store token contract address");
    ctx.store_value("address", &ProgramValue::from(lot_address))
        .expect("Failed to store address");
}

/// Randomly generates a number (1-100) and transfers those tokens to the player.
/// Calls the token contract(which is an external program call using invoke) to
/// transfer tokens to the player.
#[expose]
fn play(ctx: ProgramContext, player: Address) -> bool {
    let num = get_random_number(player);
    // If win transfer to player
    let call_ctx = match ctx.get_value(&TOKEN_CONTRACT_NAME) {
        Ok(value) => ProgramContext::from(value),
        Err(_) => {
            return false;
        }
    };

    let lotto_addy = match ctx.get_value("address") {
        Ok(value) => value,
        Err(_) => {
            return false;
        }
    };

    // Transfer
    ctx.program_invoke(
        &call_ctx,
        "transfer",
        &[
            lotto_addy,
            ProgramValue::from(player),
            ProgramValue::IntObject(num),
        ],
    );
    true
}

// Seeding WASM RNG with the the player's address(which is currently randomly generated from host)
// For demo purposes only, as this isn't a true rng.
fn get_random_number(seed: Address) -> i64 {
    use rand::Rng;
    use rand_chacha::rand_core::SeedableRng;
    use rand_chacha::ChaCha8Rng;

    let first_val = ProgramValue::from(seed);
    let mut rng = ChaCha8Rng::seed_from_u64(first_val.as_bytes()[1] as u64);
    rng.gen_range(0..100)
}
