use std::{path::PathBuf, process::Command};
use wasmlanche_sdk::{Context, Program};
use wasmtime::{Instance, Module, Store};

const WASM_TARGET: &str = "wasm32-unknown-unknown";
const TEST_PKG: &str = "test-crate";
const PROFILE: &str = "release";

#[test]
fn public_functions() {
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

    let wasm_path = target_dir
        .join(WASM_TARGET)
        .join(PROFILE)
        .join(TEST_PKG.replace('-', "_"))
        .with_extension("wasm");

    let mut store: Store<()> = Store::default();
    let module = Module::from_file(store.engine(), wasm_path).expect("failed to load wasm");
    let instance = Instance::new(&mut store, &module, &[]).expect("failed to instantiate wasm");

    type AllocParam = i32;
    type AllocReturn = i32;
    let allocate = instance
        .get_typed_func::<AllocParam, AllocReturn>(&mut store, "alloc")
        .expect("failed to find `allocate` function");

    let mut program_id = std::iter::repeat(1);
    let program_id: [u8; Program::LEN] = std::array::from_fn(|_| program_id.next().unwrap());

    // this is a hack to create a program since the constructor is private
    let program: Program = borsh::from_slice(&program_id).expect("the program should deserialize");

    let context = Context { program };

    let serialized_context = borsh::to_vec(&context).expect("failed to serialize context");
    let context_len = serialized_context.len() as AllocParam;

    let offset = allocate
        .call(&mut store, context_len)
        .expect("failed to allocate memory");

    let offset = offset as u32;

    let ptr = ((context_len as i64) << 32) | offset as i64;

    let memory = instance
        .get_memory(&mut store, "memory")
        .expect("failed to get memory");

    memory
        .write(&mut store, offset as usize, &serialized_context)
        .expect("failed to write to memory");

    let noop = instance
        .get_typed_func::<i64, i64>(&mut store, "noop_guest")
        .expect("failed to find `noop` function");

    let result = noop
        .call(&mut store, ptr)
        .expect("failed to call `noop` function");

    assert_eq!(result, true as i64);

    let return_id = instance
        .get_typed_func::<i64, u32>(&mut store, "return_id_guest")
        .expect("return_id should be a function");

    let result = return_id
        .call(store, ptr)
        .expect("failed to call `return_id` function");

    assert_eq!(result, u32::MAX);
}
