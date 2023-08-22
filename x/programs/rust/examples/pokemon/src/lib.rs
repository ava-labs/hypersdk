use expose_macro::expose;
use serde_derive::{Deserialize, Serialize};
use wasmlanche_sdk::program::Program;
use wasmlanche_sdk::store::ProgramContext;
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
pub fn init_program() -> i64 {
    let mut pokemon = Program::new();

    pokemon.add_field(String::from("total_supply"), 10).unwrap();

    pokemon.into()
}

#[expose]
pub fn catch(ctx: ProgramContext, player: Address) -> bool {
    let pokemon = Pokemon {
        name: String::from("Pikachu"),
        level: 1,
        hp: get_random_number(player, 0) as u32,
        moves: vec![String::from("Thunderbolt"), String::from("Quick Attack")],
    };

    let mut owned: OwnedPokemon = ctx
        .get_map_value("owned", &String::from("player"))
        .unwrap_or(vec![]);
    owned.push(pokemon);

    ctx.store_map_value("owned", &String::from("player"), &owned)
        .unwrap();
    println!("Owned: {:?}", owned);
    true
}

#[expose]
pub fn get_owned(ctx: ProgramContext, player: Address) -> bool {
    // get players pokemon and print to screen
    let owned: OwnedPokemon = ctx
        .get_map_value("owned", &String::from("player"))
        .unwrap_or_else(|err| {
            println!("Error: {:?}", err);
            vec![]
        });
    println!("Getting owned: {:?}", owned);
    true
}

// Seeding WASM RNG with the the player's address(which is currently randomly generated from host)
// For demo purposes only, as this isn't a true rng.
fn get_random_number(seed: Address, index: usize) -> i64 {
    use rand::Rng;
    use rand_chacha::rand_core::SeedableRng;
    use rand_chacha::ChaCha8Rng;

    let first_val = seed;
    let mut rng = ChaCha8Rng::seed_from_u64(first_val.as_bytes()[index] as u64);
    rng.gen_range(0..100)
}
