use std::env;
use std::path::Path;
use std::path::PathBuf;
use std::process::Command;

// builds the go library
// go build -buildmode=c-shared -o libsimulator.so ffi.go
// generates the bindings for the C header file
// writes the bindings to the $OUT_DIR/bindings.rs file
fn main() {
    let dir = env::var("CARGO_MANIFEST_DIR").unwrap();
    let output = Path::new(&dir).join("libsimulator.so");
    let go_file = Path::new(&dir).join("./ffi/ffi.go");

    // Build the Go library
    let status = Command::new("go")
        .args(["build", "-buildmode=c-shared", "-o"])
        .arg(&output)
        .arg(&go_file)
        .status()
        .expect("Failed to execute Go build command");

    if !status.success() {
        panic!("Go build command failed");
    }

    println!("cargo::rustc-link-search=native={}", dir);
    // link the dynamic library created by go build
    println!("cargo::rustc-link-lib=dylib=simulator");

    // Rerun the script if simulator.so changes
    println!(
        "cargo::rerun-if-changed={}",
        Path::new(&dir).join("simulator.so").display()
    );

    // Import the types from the C header file
    let bindings = bindgen::Builder::default()
        .ctypes_prefix("libc")
        .header("./common/types.h")
        .parse_callbacks(Box::new(bindgen::CargoCallbacks::new()))
        .generate()
        .expect("unable to generate bindings");

    // Write the bindings to the $OUT_DIR/bindings.rs file.
    let out_path = PathBuf::from(env::var("OUT_DIR").unwrap());
    bindings
        .write_to_file(out_path.join("bindings.rs"))
        .expect("Couldn't write bindings!");
}
