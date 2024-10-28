// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use std::{path::Path, process::Command};

pub const BUILD_DIR_NAME: &str = "build";
const WASM_TARGET: &str = "wasm32-unknown-unknown";
const RELEASE_PROFILE: &str = "release";

#[allow(clippy::module_name_repetitions)]
/// Put this in your build.rs file. It currently relies on `/build` directory to be in your crate root.
/// # Panics
/// Will panic when attempting to buld the wasm file fails.
pub fn build_wasm() {
    let target = std::env::var("TARGET").unwrap();
    let profile = std::env::var("PROFILE").unwrap();

    if target != WASM_TARGET {
        let package_name = std::env::var("CARGO_PKG_NAME").unwrap();
        let manifest_dir = std::env::var("CARGO_MANIFEST_DIR").unwrap();

        let profile = if profile == RELEASE_PROFILE {
            &profile
        } else {
            "test"
        };

        let features = std::env::vars()
            .filter_map(|(key, value)| {
                if key.starts_with("CARGO_FEATURE_") && value == "1" {
                    let feature = key.trim_start_matches("CARGO_FEATURE_").to_lowercase();

                    match feature.as_str() {
                        "bindings" | "test" => None,
                        _ => Some(feature),
                    }
                } else {
                    None
                }
            })
            .collect::<Vec<_>>();

        let target_dir = format!("{manifest_dir}/{BUILD_DIR_NAME}");
        let mut command = Command::new("cargo");
        command
            .arg("rustc")
            .arg("--target")
            .arg(WASM_TARGET)
            .arg("--target-dir")
            .arg(&target_dir);

        if profile == RELEASE_PROFILE {
            command.arg("--release");
        }

        if !features.is_empty() {
            command.arg("--features").arg(features.join(","));
        }

        command.arg("--crate-type").arg("cdylib");

        let cargo_build_output = command
            .output()
            .expect("command should execute even if it fails");

        let profile = if profile == RELEASE_PROFILE {
            "release"
        } else {
            "debug"
        };

        if !cargo_build_output.status.success() {
            let stdout = String::from_utf8_lossy(&cargo_build_output.stdout);
            let stderr = String::from_utf8_lossy(&cargo_build_output.stderr);

            println!("cargo:warning=stdout:");

            for line in stdout.lines() {
                println!("cargo:warning={line}");
            }

            println!("cargo:warning=stderr:");

            for line in stderr.lines() {
                println!("cargo:warning={line}");
            }

            println!("cargo:warning=exit-status={}", cargo_build_output.status);

            panic!("failed to build wasm file");
        }

        let target_dir = Path::new(&target_dir)
            .join(WASM_TARGET)
            .join(profile)
            .join(format!("{}.wasm", package_name.replace('-', "_")));

        let target_dir = match target_dir.canonicalize() {
            Ok(target_dir) => target_dir,
            err @ Err(_) => {
                println!("cargo:warning= not found -> {target_dir:?}");
                err.expect("failed to canonicalize wasm file path")
            }
        };

        println!("cargo:warning=`.wasm` file at {target_dir:?}");

        let target_dir = target_dir
            .to_str()
            .expect("crate name must not contain any non-utf8 characters");
        println!("cargo:rustc-env=CONTRACT_PATH={target_dir}");

        println!(
            r#"cargo:warning=If the simulator fails to find the "{package_name}" contract, try running `cargo clean -p {package_name}` followed by `cargo test` again."#
        );
    }
}
