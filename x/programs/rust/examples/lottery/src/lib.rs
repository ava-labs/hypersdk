use wasmlanche_sdk::{public, store::State, types::Address};

// Define the name of the token contract in the programs storage map.
static TOKEN_PROGRAM_NAME: &str = "token_contract";

/// Sets the token contract address and the lotto address. This needs to be set
/// before play can be called, otherwise there is no reference contract and address.
#[public]
fn set(state: State, lottery_state: State, lottery_address: Address) -> bool {
    state
        .store_value(TOKEN_PROGRAM_NAME, &lottery_state)
        .store_value("address", &lottery_address)
        .is_ok()
}

/// Randomly generates a number (1-100) and transfers those tokens to the player.
/// Calls the token contract(which is an external program call using invoke) to
/// transfer tokens to the player.
#[public]
fn play(state: State, player: Address) -> bool {
    let num = get_random_number(player, 1);
    // If win transfer to player
    let lottery_state = match state.get_value(TOKEN_PROGRAM_NAME) {
        Ok(state) => state,
        Err(_) => return false,
    };

    let Ok(lotto_addy) = state.get_value::<Address>("address") else {
        return false;
    };

    // Transfer
    let _ = state.invoke_program(
        lottery_state,
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
