use std::env;
use std::path::Path;

// build the go library via
// go build -buildmode=c-shared -o libsimulator.so ffi.go
fn main() {
    let dir = env::var("CARGO_MANIFEST_DIR").unwrap();
    println!("cargo:rustc-link-search=native={}", dir);
    println!("cargo:rustc-link-lib=dylib=simulator");

    // Rerun the script if simulator.so changes
    println!(
        "cargo:rerun-if-changed={}",
        Path::new(&dir).join("simulator.so").display()
    );
}
