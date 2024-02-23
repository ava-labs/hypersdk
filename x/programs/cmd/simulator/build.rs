use std::{path::Path, process::Command};

fn main() -> Result<(), Box<dyn std::error::Error>> {
    println!("cargo:rerun-if-changed=build.rs");

    println!("cargo:warning=fetching go dependencies...");

    let status = Command::new("go").args(&["mod", "download"]).status()?;

    println!("cargo:warning={status}");

    println!("cargo:warning=building simulator...");

    let simulator_path = "./bin/simulator";
    let simulator_src = "./simulator.go";

    // resolve absolute path for simulator_path
    let simulator_path = Path::new(simulator_path).canonicalize().unwrap();
    let simulator_path = simulator_path.to_str().unwrap();

    let simulator_src = Path::new(simulator_src).canonicalize().unwrap();
    let simulator_src = simulator_src.to_str().unwrap();

    let status = Command::new("go")
        .args(&["build", "-o", simulator_path, simulator_src])
        .status()?;

    println!("cargo:warning={status}");

    println!("cargo:rustc-env=SIMULATOR_PATH={simulator_path}");

    Ok(())
}
