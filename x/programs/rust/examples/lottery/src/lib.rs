use expose_macro::expose;
use wasmlanche_sdk::program::Program;
use wasmlanche_sdk::store::Context;
use wasmlanche_sdk::types::Address;

// Define the name of the token contract in the programs storage map.
static TOKEN_PROGRAM_NAME: &str = "token_contract";

/// Initializes the program.
#[expose]
fn init_program() -> i64 {
    // Initialize the program with no fields
    Program::new().into()
}

/// Sets the token contract address and the lotto address. This needs to be set
/// before play can be called, otherwise there is no reference contract and address.
#[expose]
fn set(ctx: Context, counter_ctx: Context, lot_address: Address) {
    ctx.store_value(TOKEN_PROGRAM_NAME, &counter_ctx)
        .expect("Failed to store token contract address");
    ctx.store_value("address", &lot_address)
        .expect("Failed to store address");
}

/// Randomly generates a number (1-100) and transfers those tokens to the player.
/// Calls the token contract(which is an external program call using invoke) to
/// transfer tokens to the player.
#[expose]
fn play(ctx: Context, player: Address) -> bool {
    let num = get_random_number(player, 1);
    // If win transfer to player
    let call_ctx = match ctx.get_value(TOKEN_PROGRAM_NAME) {
        Ok(ctx) => ctx,
        Err(_) => return false,
    };

    let Ok(lotto_addy) = ctx.get_value::<Address>("address") else {
        return false;
    };

    // Transfer
    ctx.program_invoke(
        &call_ctx,
        "transfer",
        &[Box::new(lotto_addy), Box::new(player), Box::new(num)],
    );
    true
}

/// Seeding WASM RNG with the the player's address(which is currently randomly generated from host)
/// For demo purposes only, as this isn't a true rng.
fn get_random_number(seed: Address, index: usize) -> i64 {
    use rand::Rng;
    use rand_chacha::rand_core::SeedableRng;
    use rand_chacha::ChaCha8Rng;

    let mut rng = ChaCha8Rng::seed_from_u64(seed.as_bytes()[index] as u64);
    rng.gen_range(0..100)
}
