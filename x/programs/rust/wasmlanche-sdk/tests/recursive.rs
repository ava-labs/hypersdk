use simulator::{Endpoint, Key, Plan, Step};
use std::{path::PathBuf, process::Command};

const WASM_TARGET: &str = "wasm32-unknown-unknown";
const TEST_PKG: &str = "test-crate";
const PROFILE: &str = "release"; // "dev";

#[test]
fn recursive() {
    let wasm_path = {
        let manifest_dir = std::env::var("CARGO_MANIFEST_DIR").unwrap();
        let manifest_dir = std::path::Path::new(&manifest_dir);
        let test_crate_dir = manifest_dir.join("tests").join(TEST_PKG);
        let target_dir = std::env::var("CARGO_TARGET_DIR")
            .map(PathBuf::from)
            .unwrap_or_else(|_| manifest_dir.join("target"));

        let status = Command::new("cargo")
            .arg("build")
            .arg("--package")
            .arg(TEST_PKG)
            .arg("--target")
            .arg(WASM_TARGET)
            .arg("--profile")
            .arg(PROFILE)
            .arg("--target-dir")
            .arg(&target_dir)
            .current_dir(&test_crate_dir)
            .status()
            .expect("cargo build failed");

        if !status.success() {
            panic!("cargo build failed");
        }

        let profile_location: &str = match PROFILE {
            "dev" => "debug",
            "release" => "release",
            _ => panic!("unrecognized profile {}", PROFILE),
        };

        target_dir
            .join(WASM_TARGET)
            .join(profile_location)
            .join(TEST_PKG.replace('-', "_"))
            .with_extension("wasm")
    };

    let simulator = simulator::Client::new();

    let owner_key = String::from("owner");
    let alice_key = Key::Ed25519(String::from("alice"));

    let mut plan = Plan::new(owner_key.clone());

    plan.add_step(Step::create_key(Key::Ed25519(owner_key)));
    plan.add_step(Step::create_key(alice_key));

    let program_id = plan.add_step(Step::create_program(wasm_path.to_str().unwrap().to_owned()));
    let max_units = 10000000;

    plan.add_step(Step {
        endpoint: Endpoint::Execute,
        method: "recursive".into(),
        max_units,
        params: vec![program_id.into(), max_units.into()],
        require: None,
    });

    // run plan
    let plan_responses = simulator.run_plan(plan).unwrap();

    assert!(
        plan_responses.iter().all(|resp| resp.error.is_none()),
        "error: {:?}",
        plan_responses
            .iter()
            .filter_map(|resp| resp.error.as_ref())
            .next()
    );
}
