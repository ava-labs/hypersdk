use std::process::Command;

fn main() -> Result<(), Box<dyn std::error::Error>> {
    println!("cargo:rerun-if-changed=build.rs");
    println!("cargo:rerun-if-changed=go.mod");
    println!("cargo:rerun-if-changed=simulator.go");
    println!("cargo:rerun-if-changed=cmd/");

    let go_mod_download_output = Command::new("go").args(["mod", "download"]).output()?;

    if !go_mod_download_output.status.success() {
        println!("cargo:warning=go mod downlod stdout:");

        for line in String::from_utf8_lossy(&go_mod_download_output.stdout).lines() {
            println!("cargo:warning={line}");
        }

        println!("cargo:warning=go mod download stderr:");

        for line in String::from_utf8_lossy(&go_mod_download_output.stderr).lines() {
            println!("cargo:warning={line}");
        }
    }

    let current_dir = std::env::current_dir().unwrap();

    let simulator_path = "bin/simulator";
    let simulator_src = "simulator.go";

    let simulator_path = current_dir.join(simulator_path);
    let simulator_path = simulator_path.to_str().unwrap();

    let simulator_src = current_dir.join(simulator_src);
    let simulator_src = simulator_src.to_str().unwrap();

    let go_build_output = Command::new("go")
        .args(["build", "-o", simulator_path, simulator_src])
        .output()?;

    if !go_build_output.status.success() {
        println!("cargo:warning=go build stdout:");

        for line in String::from_utf8_lossy(&go_build_output.stdout).lines() {
            println!("cargo:warning={line}");
        }

        println!("cargo:warning=go build stderr:");

        for line in String::from_utf8_lossy(&go_build_output.stderr).lines() {
            println!("cargo:warning={line}");
        }
    }

    println!("cargo:rustc-env=SIMULATOR_PATH={simulator_path}");

    Ok(())
}
