use ed25519_dalek::{Signature, SigningKey, Signer};
 
use wasmlanche_sdk::{program::Program, public};

/// Verifies the ed25519 signature in wasm. 
#[public]
pub fn verify_ed_in_wasm(_: Program) {
    println!("statt sss world");

    use ed25519_dalek::SigningKey;
    use ed25519_dalek::SECRET_KEY_LENGTH;
    use ed25519_dalek::SignatureError;
    
    let secret_key_bytes: [u8; SECRET_KEY_LENGTH] = [
       157, 097, 177, 157, 239, 253, 090, 096,
       186, 132, 074, 244, 146, 236, 044, 196,
       068, 073, 197, 105, 123, 050, 105, 025,
       112, 059, 172, 003, 028, 174, 127, 096, ];
       println!("here sss world");
    
    let signing_key: SigningKey = SigningKey::from_bytes(&secret_key_bytes);
    println!("back sss world");

    // Parse the public key and signature
    let message: &[u8] = b"This is a test of the tsunami alert system.";
    let signature: Signature = signing_key.sign(message);


    match signing_key.verify(message, &signature) {
        Ok(_) => println!("Signature verified"),
        Err(_) => println!("Signature not verified"),
    }
    println!("Hello sss world");
    // // Anyone else, given the public half of the signing_key can also easily verify this signature:
   
    // let verifying_key: VerifyingKey = signing_key.verifying_key();
    // assert!(verifying_key.verify(message, &signature).is_ok());
}

// #[public]
// pub fn verify_ed_multiple_host_func(program: Program)  {

// }

// #[public]
// pub fn verify_ed_batch_in_host(program: Program) {

// }


// /// Seeding WASM RNG with the the player's address(which is currently randomly generated from host)
// /// For demo purposes only, as this isn't a true rng.
// fn get_random_number(seed: Address, index: usize) -> i64 {
//     use rand::Rng;
//     use rand_chacha::rand_core::SeedableRng;
//     use rand_chacha::ChaCha8Rng;

//     let mut rng = ChaCha8Rng::seed_from_u64(seed.as_bytes()[index] as u64);
//     rng.gen_range(0..100)
// }