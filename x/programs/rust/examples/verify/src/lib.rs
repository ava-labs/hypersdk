// use ed25519_dalek::{Signature, SigningKey, Signer};

 
use wasmlanche_sdk::{program::Program, public};

/// Verifies the ed25519 signature in wasm. 
#[public]
pub fn verify_ed_in_wasm(_: Program) {

    // let mut csprng = OsRng;
    // let signing_key: SigningKey = SigningKey::generate(&mut csprng);
    
    // // Parse the public key and signature
    // let message: &[u8] = b"This is a test of the tsunami alert system.";
    // let signature: Signature = signing_key.sign(message);


    // match signing_key.verify(message, &signature) {
    //     Ok(_) => println!("Signature verified"),
    //     Err(_) => println!("Signature not verified"),
    // }
    println!("Hello world");
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