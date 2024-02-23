use std::process::Command;

pub const BUILD_DIR_NAME: &str = "build";
const WASM_TARGET: &str = "wasm32-unknown-unknown";
const RELEASE_PROFILE: &str = "release";

/// Put this in your build.rs file. It currently relies on `/build` directory to be in your crate root.
pub fn build_wasm_on_test() {
    println!("cargo:rerun-if-changed=build.rs");

    // TODO:
    // remove these printlns
    let target = std::env::var("TARGET").unwrap();
    println!("cargo:warning=TARGET={target}");

    let profile = std::env::var("PROFILE").unwrap();
    println!("cargo:warning=PROFILE={profile}");

    if target != WASM_TARGET {
        let package_name = std::env::var("CARGO_PKG_NAME").unwrap();

        println!("cargo:warning=building `{}` wasm file", package_name);

        let manifest_dir = std::env::var("CARGO_MANIFEST_DIR").unwrap();

        let profile = if profile == RELEASE_PROFILE {
            &profile
        } else {
            "test"
        };

        let cargo_build_output = Command::new("cargo")
            .arg("build")
            .arg("--target")
            .arg(WASM_TARGET)
            .arg("--profile")
            .arg(&profile)
            .arg("--target-dir")
            .arg(&format!("{manifest_dir}/{BUILD_DIR_NAME}"))
            .output()
            .expect("command should execute even if it fails");

        if !cargo_build_output.status.success() {
            let stdout = String::from_utf8_lossy(&cargo_build_output.stdout);
            let stderr = String::from_utf8_lossy(&cargo_build_output.stderr);

            println!("cargo:warning=stdout:");

            for line in stdout.lines() {
                println!("cargo:warning={}", line);
            }

            println!("cargo:warning=stderr:");

            for line in stderr.lines() {
                println!("cargo:warning={}", line);
            }

            println!("cargo:warning=exit-status={}", cargo_build_output.status);
        }
    }
}
