use std::{
    path::{Path, PathBuf},
    process::Command,
};
use wasmlanche_sdk::{types::Address, Context, Program};
use wasmtime::{Caller, Extern, Func, Instance, Module, Store, TypedFunc};

const WASM_TARGET: &str = "wasm32-unknown-unknown";
const TEST_PKG: &str = "test-crate";
const PROFILE: &str = "release";

#[test]
fn public_functions() {
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

        target_dir
            .join(WASM_TARGET)
            .join(PROFILE)
            .join(TEST_PKG.replace('-', "_"))
            .with_extension("wasm")
    };

    let mut test_crate = TestCrate::new(wasm_path);

    let context_ptr = test_crate.write_context();
    assert!(test_crate.always_true(context_ptr));

    let context_ptr = test_crate.write_context();
    let combined_binary_digits = test_crate.combine_last_bit_of_each_id_byte(context_ptr);
    assert_eq!(combined_binary_digits, u32::MAX);
}

type AllocParam = i32;
type AllocReturn = u32;
type AllocFn = TypedFunc<AllocParam, AllocReturn>;
type UserDefinedFnParam = u32;
type UserDefinedFnReturn = ();
type UserDefinedFn = TypedFunc<UserDefinedFnParam, UserDefinedFnReturn>;
type StoreData = Option<Vec<u8>>;

struct TestCrate {
    store: Store<StoreData>,
    instance: Instance,
    allocate_func: AllocFn,
    always_true_func: UserDefinedFn,
    combine_last_bit_of_each_id_byte_func: UserDefinedFn,
}

impl TestCrate {
    fn new(wasm_path: impl AsRef<Path>) -> Self {
        let mut store: Store<StoreData> = Store::default();
        let module = Module::from_file(store.engine(), wasm_path).expect("failed to load wasm");

        let set_result_fn = Func::wrap(
            &mut store,
            |mut caller: Caller<'_, StoreData>, ptr: u32, len: u32| {
                let Extern::Memory(memory) = caller
                    .get_export("memory")
                    .expect("memory should be exported")
                else {
                    panic!("export `memory` should be of type `Memory`");
                };

                let (ptr, len) = (ptr as usize, len as usize);

                let result = memory
                    .data(&mut caller)
                    .get(ptr..ptr + len)
                    .expect("data should exist");

                *caller.data_mut() = Some(result.to_vec());
            },
        );

        let instance = Instance::new(&mut store, &module, &[set_result_fn.into()])
            .expect("failed to instantiate wasm");

        let allocate_func = instance
            .get_typed_func(&mut store, "alloc")
            .expect("failed to find `alloc` function");

        let always_true_func = instance
            .get_typed_func(&mut store, "always_true_guest")
            .expect("failed to find `always_true` function");
        let combine_last_bit_of_each_id_byte_func = instance
            .get_typed_func(&mut store, "combine_last_bit_of_each_id_byte_guest")
            .expect("combine_last_bit_of_each_id_byte should be a function");

        Self {
            store,
            instance,
            allocate_func,
            always_true_func,
            combine_last_bit_of_each_id_byte_func,
        }
    }

    fn write_context(&mut self) -> AllocReturn {
        let program_id: [u8; Program::LEN] = std::array::from_fn(|_| 1);
        // this is a hack to create a program since the constructor is private
        let program: Program =
            borsh::from_slice(&program_id).expect("the program should deserialize");
        let actor = Address::new(Default::default());
        let context = Context { program, actor };
        let serialized_context = borsh::to_vec(&context).expect("failed to serialize context");

        self.allocate(serialized_context)
    }

    fn allocate(&mut self, data: Vec<u8>) -> AllocReturn {
        let offset = self
            .allocate_func
            .call(&mut self.store, data.len() as i32)
            .expect("failed to allocate memory");
        let memory = self
            .instance
            .get_memory(&mut self.store, "memory")
            .expect("failed to get memory");

        memory
            .write(&mut self.store, offset as usize, &data)
            .expect("failed to write data to memory");

        offset
    }

    fn always_true(&mut self, ptr: UserDefinedFnParam) -> bool {
        self.always_true_func
            .call(&mut self.store, ptr)
            .expect("failed to call `always_true` function");
        let result = self
            .store
            .data_mut()
            .take()
            .expect("always_true should always return something");

        borsh::from_slice(&result).expect("failed to deserialize result")
    }

    fn combine_last_bit_of_each_id_byte(&mut self, ptr: UserDefinedFnParam) -> u32 {
        self.combine_last_bit_of_each_id_byte_func
            .call(&mut self.store, ptr)
            .expect("failed to call `combine_last_bit_of_each_id_byte` function");
        let result = self
            .store
            .data_mut()
            .take()
            .expect("combine_last_bit_of_each_id_byte should always return something");
        borsh::from_slice(&result).expect("failed to deserialize result")
    }
}
