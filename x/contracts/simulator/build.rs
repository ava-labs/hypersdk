// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use std::{env, path::PathBuf, process::Command};

// builds the go library
// go build -buildmode=c-shared -o libsimulator.so ffi/ffi.go
// generates the bindings for the C header file
// writes the bindings to the $OUT_DIR/bindings.rs file
fn main() {
    let crate_root = PathBuf::from(env::var("CARGO_MANIFEST_DIR").unwrap());
    let profile = env::var("PROFILE").unwrap();

    let target_dir = env::var("CARGO_TARGET_DIR")
        .or_else(|_| -> Result<_, Box<dyn std::error::Error>> {
            let json = Command::new("cargo").arg("metadata").output()?.stdout;
            let json = serde_json::from_slice::<serde_json::Value>(&json)?;
            Ok(json["target_directory"].as_str().unwrap().to_string())
        })
        .expect("Failed to get target directory");
    let target_dir = PathBuf::from(target_dir).join(&profile);

    let state_package = crate_root.join("state");
    let ffi_package = crate_root.join("ffi");
    let common_path = crate_root.join("common");
    let rust_src = crate_root.join("src");

    // rerun the build script if go files change
    println!("cargo:rerun-if-changed={}", state_package.to_string_lossy());
    println!("cargo:rerun-if-changed={}", ffi_package.to_string_lossy());
    println!("cargo:rerun-if-changed={}", common_path.to_string_lossy());
    println!("cargo:rerun-if-changed={}", rust_src.to_string_lossy());

    let output = target_dir.join("libsimulator.so");
    let go_file = ffi_package.join("ffi.go");

    // Build the Go library
    let status = Command::new("go")
        .args(["build", "-buildmode=c-shared", "-tags=debug", "-o"])
        .arg(&output)
        .arg(&go_file)
        .status()
        .expect("Failed to execute Go build command");

    if !status.success() {
        panic!("Go build command failed");
    }

    println!(
        "cargo::rustc-link-search=native={}",
        target_dir.to_string_lossy()
    );

    // link the dynamic library created by go build
    println!("cargo::rustc-link-lib=dylib=simulator");

    let types_path = common_path.join("types.h");

    // Import the types from the C header file
    let bindings = bindgen::Builder::default()
        .ctypes_prefix("libc")
        .header(types_path.to_string_lossy())
        .parse_callbacks(Box::new(bindgen::CargoCallbacks::new()))
        .generate()
        .expect("unable to generate bindings");

    // Write the bindings to the $OUT_DIR/bindings.rs file.
    let out_path = PathBuf::from(env::var("OUT_DIR").unwrap());

    println!(
        "cargo::warning=Writing bindings to {:?}",
        out_path.join("bindings.rs")
    );

    bindings
        .write_to_file(out_path.join("bindings.rs"))
        .expect("Couldn't write bindings!");
}
