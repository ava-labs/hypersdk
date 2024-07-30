use std::path::Path;

fn main() {
    let manifest_dir = std::env::var("CARGO_MANIFEST_DIR").unwrap();
    let package_name = std::env::var("CARGO_PKG_NAME").unwrap();
    let path = Path::new(&manifest_dir)
        .canonicalize()
        .expect("failed to canonicalize path");
    let token_package_name = "token";
    let token_manifest_dir = path
        .parent()
        .map(|p| p.join(token_package_name))
        .expect("failed to find token manifest path");

    // # Safety
    // This is safe to call in a single-threaded program
    unsafe {
        std::env::set_var("CARGO_MANIFEST_DIR", token_manifest_dir);
        std::env::set_var("CARGO_PKG_NAME", token_package_name);
    }
    wasmlanche_sdk::build::build_wasm_on_test();

    unsafe {
        std::env::set_var("CARGO_MANIFEST_DIR", manifest_dir);
        std::env::set_var("CARGO_PKG_NAME", package_name);
    }
    wasmlanche_sdk::build::build_wasm_on_test();
}
