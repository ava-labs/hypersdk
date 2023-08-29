use expose_macro::expose;
use serde::{Deserialize, Serialize};
use wasmlanche_sdk::store::Context;
use wasmlanche_sdk::types::Address;

#[derive(Serialize, Deserialize, Debug)]
struct Pokemon {
    name: String,
    level: u32,
    hp: u32,
    moves: Vec<String>,
}

// non-string keys are not supported by serde
type OwnedPokemon = Vec<Pokemon>;

#[expose]
pub fn init(ctx: Context) -> bool {
    ctx.store_value("total_supply", &10_i64).is_ok()
}

#[expose]
pub fn catch(ctx: Context, player: Address) -> bool {
    let pokemon = Pokemon {
        name: String::from("Pikachu"),
        level: 1,
        hp: get_random_number(player, 0) as u32,
        moves: vec![String::from("Thunderbolt"), String::from("Quick Attack")],
    };

    let mut owned: OwnedPokemon = ctx.get_map_value("owned", &player).unwrap_or_default();
    owned.push(pokemon);

    ctx.store_map_value("owned", &player, &owned).is_ok()
}

#[expose]
pub fn get_owned(ctx: Context, player: Address) -> bool {
    // get players pokemon and print to screen
    let owned: OwnedPokemon = ctx
        .get_map_value("owned", &player)
        .unwrap_or_else(|_| vec![]);
    println!("Owned: {:?}", owned);
    true
}

// Seeding WASM RNG with the the player's address(which is currently randomly generated from host)
// For demo purposes only, as this isn't a true rng.
fn get_random_number(seed: Address, index: usize) -> i64 {
    use rand::Rng;
    use rand_chacha::rand_core::SeedableRng;
    use rand_chacha::ChaCha8Rng;

    let mut rng = ChaCha8Rng::seed_from_u64(seed.as_bytes()[index] as u64);
    rng.gen_range(0..100)
}
