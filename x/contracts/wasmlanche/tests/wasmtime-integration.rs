// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

#![cfg(not(target_arch = "wasm32"))]

use std::{
    collections::HashMap,
    path::{Path, PathBuf},
    process::Command,
};
use wasmlanche::{Address, ID_LEN};
use wasmtime::{
    Caller, Config, Engine, Extern, Instance, InstanceAllocationStrategy, Linker, Memory, Module,
    PoolingAllocationConfig, Store, TypedFunc,
};

const WASM_TARGET: &str = "wasm32-unknown-unknown";
const TEST_PKG: &str = "test-crate";
const PROFILE: &str = "release";

#[test]
fn public_functions() {
    let mut test_crate = build_test_crate();

    let context_ptr = test_crate.write_context();
    assert!(test_crate.always_true(context_ptr));

    let context_ptr = test_crate.write_context();
    let combined_binary_digits = test_crate.combine_last_bit_of_each_id_byte(context_ptr);
    assert_eq!(combined_binary_digits, u32::MAX);
}

#[test]
// the failure message is from the `expect` in this file
#[should_panic = "failed to allocate memory"]
fn allocate_zero() {
    let mut test_crate = build_test_crate();
    test_crate.allocate(Vec::new());
}

const ALLOCATION_MAP_OVERHEAD: usize = 48;

#[test]
fn allocate_data_size() {
    let mut test_crate = build_test_crate();
    let context_ptr = test_crate.write_context();
    let highest_address = test_crate.highest_allocated_address(context_ptr);
    let memory = test_crate.memory();
    let room = memory.data_size(test_crate.store_mut()) - highest_address - ALLOCATION_MAP_OVERHEAD;
    let data = vec![0xaa; room];
    let ptr = test_crate.allocate(data.clone()) as usize;
    let memory = memory.data(test_crate.store_mut());
    assert_eq!(&memory[ptr..ptr + room], &data);
}

#[test]
#[should_panic]
fn allocate_data_size_plus_one() {
    let mut test_crate = build_test_crate();
    let context_ptr = test_crate.write_context();
    let highest_address = test_crate.highest_allocated_address(context_ptr);
    let memory = test_crate.memory();
    let room = memory.data_size(test_crate.store_mut()) - highest_address - ALLOCATION_MAP_OVERHEAD;
    let data = vec![0xaa; room + 1];
    test_crate.allocate(data.clone());
}

fn build_test_crate() -> TestCrate {
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

    TestCrate::new(wasm_path)
}

type AllocParam = u32;
type AllocReturn = u32;
type AllocFn = TypedFunc<AllocParam, AllocReturn>;
type UserDefinedFnParam = u32;
type UserDefinedFnReturn = ();
type UserDefinedFn = TypedFunc<UserDefinedFnParam, UserDefinedFnReturn>;
type StateKey = Box<[u8]>;
type StateValue = Box<[u8]>;
type StoreData = (Option<Vec<u8>>, HashMap<StateKey, StateValue>);

struct TestCrate {
    store: Store<StoreData>,
    instance: Instance,
    allocate_func: AllocFn,
    highest_address_func: UserDefinedFn,
    always_true_func: UserDefinedFn,
    combine_last_bit_of_each_id_byte_func: UserDefinedFn,
}

impl TestCrate {
    fn new(wasm_path: impl AsRef<Path>) -> Self {
        let mut config = Config::new();
        let mut allocation_strategy = PoolingAllocationConfig::default();
        allocation_strategy.memory_pages(18); // not sure why, but this seems to be the min

        let allocation_strategy = InstanceAllocationStrategy::Pooling(allocation_strategy);
        config.allocation_strategy(allocation_strategy);

        let engine = Engine::new(&config).expect("engine failure");
        let mut store: Store<StoreData> = Store::new(&engine, Default::default());
        let mut linker = Linker::new(store.engine());
        let module = Module::from_file(store.engine(), wasm_path).expect("failed to load wasm");

        linker
            .func_wrap(
                "contract",
                "set_call_result",
                |mut caller: Caller<'_, StoreData>, ptr: u32, len: u32| {
                    let Extern::Memory(memory) = caller
                        .get_export("memory")
                        .expect("memory should be exported")
                    else {
                        panic!("export `memory` should be of type `Memory`");
                    };

                    let (ptr, len) = (ptr as usize, len as usize);

                    let result = memory
                        .data(&caller)
                        .get(ptr..ptr + len)
                        .expect("data should exist")
                        .to_vec();

                    let store_result = &mut caller.data_mut().0;
                    *store_result = Some(result.to_vec())
                },
            )
            .expect("failed to link `contract.set_call_result` function");

        linker
            .func_wrap(
                "log",
                "write",
                |mut caller: Caller<'_, StoreData>, ptr: u32, len: u32| {
                    let Extern::Memory(memory) = caller
                        .get_export("memory")
                        .expect("memory should be exported")
                    else {
                        panic!("export `memory` should be of type `Memory`");
                    };

                    let (ptr, len) = (ptr as usize, len as usize);

                    let message = memory
                        .data(&caller)
                        .get(ptr..ptr + len)
                        .expect("data should exist");

                    println!("{}", String::from_utf8_lossy(message));
                },
            )
            .expect("failed to link `log.write` function");

        linker
            .func_wrap(
                "state",
                "put",
                move |mut caller: Caller<'_, StoreData>, ptr: u32, len: u32| {
                    let Extern::Memory(memory) = caller
                        .get_export("memory")
                        .expect("memory should be exported")
                    else {
                        panic!("export `memory` should be of type `Memory`");
                    };

                    let (ptr, len) = (ptr as usize, len as usize);

                    let serialized_args = memory
                        .data(&caller)
                        .get(ptr..ptr + len)
                        .expect("data should exist");

                    let args: Vec<(StateKey, StateValue)> =
                        borsh::from_slice(serialized_args).expect("failed to deserialize args");

                    let state = &mut caller.data_mut().1;

                    state.extend(args);
                },
            )
            .expect("failed to link `state.put` function");

        let instance = linker
            .instantiate(&mut store, &module)
            .expect("failed to instantiate wasm");

        let allocate_func = instance
            .get_typed_func(&mut store, "alloc")
            .expect("failed to find `alloc` function");

        let highest_address_func = instance
            .get_typed_func(&mut store, "highest_allocated_address")
            .expect("failed to find `highest_allocated_address` function");

        let always_true_func = instance
            .get_typed_func(&mut store, "always_true")
            .expect("failed to find `always_true` function");
        let combine_last_bit_of_each_id_byte_func = instance
            .get_typed_func(&mut store, "combine_last_bit_of_each_id_byte")
            .expect("combine_last_bit_of_each_id_byte should be a function");

        Self {
            store,
            instance,
            allocate_func,
            highest_address_func,
            always_true_func,
            combine_last_bit_of_each_id_byte_func,
        }
    }

    fn write_context(&mut self) -> AllocReturn {
        let contract_id = vec![1; Address::LEN];
        let mut actor = vec![0; Address::LEN];
        let height: u64 = 0;
        let timestamp: u64 = 0;
        let mut action_id = vec![1; ID_LEN];

        // this is a hack to create a context since the constructor is private
        let mut serialized_context = contract_id;
        serialized_context.append(&mut actor);
        serialized_context.append(&mut height.to_le_bytes().to_vec());
        serialized_context.append(&mut timestamp.to_le_bytes().to_vec());
        serialized_context.append(&mut action_id);

        self.allocate(serialized_context)
    }

    fn store_mut(&mut self) -> &mut Store<StoreData> {
        &mut self.store
    }

    fn memory(&mut self) -> Memory {
        self.instance
            .get_memory(&mut self.store, "memory")
            .expect("failed to get memory")
    }

    fn allocate(&mut self, data: Vec<u8>) -> AllocReturn {
        let offset = self
            .allocate_func
            .call(&mut self.store, data.len() as u32)
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

    fn highest_allocated_address(&mut self, ptr: UserDefinedFnParam) -> usize {
        self.highest_address_func
            .call(&mut self.store, ptr)
            .expect("failed to call `highest_allocated_address` function");
        let result = self
            .store
            .data_mut()
            .0
            .take()
            .expect("highest_allocated_address should always return something");

        borsh::from_slice(&result).expect("failed to deserialize result")
    }

    fn always_true(&mut self, ptr: UserDefinedFnParam) -> bool {
        self.always_true_func
            .call(&mut self.store, ptr)
            .expect("failed to call `always_true` function");
        let result = self
            .store
            .data_mut()
            .0
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
            .0
            .take()
            .expect("combine_last_bit_of_each_id_byte should always return something");
        borsh::from_slice(&result).expect("failed to deserialize result")
    }
}
