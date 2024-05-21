use std::io::Write;
use std::process::{Command, Stdio};
use std::{env, path::PathBuf};
use std::{fs, io, process};
use tempdir::TempDir;

/// compile a test harness from name and return the location of the WASM binary
pub fn compile_wasm(harness: &str) -> (TempDir, PathBuf) {
    let crate_path = env::current_dir().expect("failed to get current directory");
    let harness_path = crate_path.join(format!("tests/runtime/harness/{harness}.rs"));
    if !harness_path.exists() {
        panic!("{} is not on your filesystem", harness_path.display());
    }

    let tmp_dir = TempDir::new(harness).expect("failed to create temporary directory");
    let tmp_path = tmp_dir.path();

    let crate_dir_display = crate_path.display();
    let cargo_toml = format!(
        r#"
        [package]
        name = "{harness}"

        [lib]
        crate-type = ["cdylib"]

        [dependencies]
        borsh = {{ version = "1.5.0", features = ["derive"] }}
        wasmlanche-sdk = {{ path = "{crate_dir_display}" }}
        sdk-macros = {{ path = "{crate_dir_display}/../sdk-macros" }}
    "#
    );

    let manifest_path = tmp_path.join("Cargo.toml");
    let mut cargo_toml_file =
        fs::File::create_new(&manifest_path).expect("failed to write Cargo.toml file");
    cargo_toml_file
        .write_all(cargo_toml.as_bytes())
        .expect("failed to write Cargo.toml file");

    let src_dir = tmp_path.join("src");
    fs::create_dir(&src_dir).expect("failed to create src/ directory");

    let lib_rs = format!("mod {harness};");
    let tmp_lib_file = &mut fs::File::create_new(src_dir.join(format!("lib.rs")))
        .expect("failed to create lib.rs file");
    tmp_lib_file
        .write_all(lib_rs.as_bytes())
        .expect("failed to write lib.rs file");

    let harness_file = &mut fs::File::open(harness_path).expect("failed to open harness file");
    let tmp_harness_file = &mut fs::File::create_new(src_dir.join(format!("{harness}.rs")))
        .expect("failed to create new harness file");
    io::copy(harness_file, tmp_harness_file).expect("failed to copy harness file");
    let target_path = tmp_path.join("target");
    let target_dir = target_path
        .as_os_str()
        .to_str()
        .expect("failed to get target directory");

    let child = Command::new("cargo")
        .arg("build")
        .args([
            "--manifest-path",
            manifest_path.to_str().expect("failed to get tmp path str"),
        ])
        .args(["--target", "wasm32-unknown-unknown"])
        .args(["--target-dir", target_dir])
        .spawn()
        .expect("failed to run cargo build");
    child
        .wait_with_output()
        .expect("cargo build command failure");

    let binary_path = target_path.join(format!("wasm32-unknown-unknown/debug/{harness}.wasm"));

    (tmp_dir, binary_path)
}
