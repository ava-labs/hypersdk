// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use borsh::BorshSerialize;
use std::{
    cell::RefCell,
    collections::{HashMap, HashSet},
    path::PathBuf,
    process::Command,
};
use wasmlanche::{Address, ID_LEN};
use wasmtime::{
    Caller, Config, Engine, Extern, Instance, Linker, Memory, Module, OptLevel, Store, StoreLimits,
    StoreLimitsBuilder, TypedFunc,
};

const WASM_TARGET: &str = "wasm32-unknown-unknown";
const PROFILE: &str = "release";
const ALLOC_FN_NAME: &str = "alloc";

type AllocParam = u32;
type AllocReturn = u32;
type AllocFn = TypedFunc<AllocParam, AllocReturn>;
pub type UserDefinedFnParam = u32;
pub type UserDefinedFnReturn = ();
pub type UserDefinedFn = TypedFunc<UserDefinedFnParam, UserDefinedFnReturn>;
type StateKey = Box<[u8]>;
type StateValue = Box<[u8]>;

pub struct StoreData {
    call_result: Option<Vec<u8>>,
    state: HashMap<StateKey, StateValue>,
    limiter: StoreLimits,
}

impl StoreData {
    #[inline]
    fn set_result(&mut self, result: Vec<u8>) {
        self.call_result = Some(result);
    }

    #[inline]
    fn set<Iter: IntoIterator<Item = (StateKey, StateValue)>>(&mut self, iter: Iter) {
        self.state.extend(iter);
    }

    #[inline]
    fn get(&self, key: &[u8]) -> Option<&StateValue> {
        self.state.get(key)
    }

    #[inline]
    pub fn take_result(&mut self) -> Option<Vec<u8>> {
        self.call_result.take()
    }
}

pub struct Builder {
    engine: Engine,
    module: Module,
    linker: Linker<StoreData>,
}

impl Builder {
    pub fn new(crate_name: &'static str) -> Self {
        thread_local! {
            static BUILD_SET: RefCell<HashSet<String>> = RefCell::new(HashSet::new());
            static WASM_CACHE: RefCell<HashMap<PathBuf, Box<[u8]>>> = RefCell::new(HashMap::new());
        }

        let wasm_path = {
            let manifest_dir = std::env::var("CARGO_MANIFEST_DIR").unwrap();
            let manifest_dir = std::path::Path::new(&manifest_dir);
            let target_dir = std::env::var("CARGO_TARGET_DIR")
                .map(PathBuf::from)
                .unwrap_or_else(|_| manifest_dir.join("target"));

            BUILD_SET.with_borrow_mut(|build_set| {
                if build_set.contains(crate_name) {
                    return;
                }

                let status = Command::new("cargo")
                    .arg("rustc")
                    .arg("--crate-type")
                    .arg("cdylib")
                    .arg("--package")
                    .arg(crate_name)
                    .arg("--target")
                    .arg(WASM_TARGET)
                    .arg("--profile")
                    .arg(PROFILE)
                    .arg("--target-dir")
                    .arg(&target_dir)
                    .status()
                    .expect("cargo build failed");

                if !status.success() {
                    panic!("cargo build failed");
                }

                build_set.insert(crate_name.to_string());
            });

            target_dir
                .join(WASM_TARGET)
                .join(PROFILE)
                .join(crate_name.replace('-', "_"))
                .with_extension("wasm")
        };

        let mut config = Config::new();

        config.cranelift_opt_level(OptLevel::Speed);
        config.consume_fuel(true);
        config.wasm_threads(false);
        config.wasm_multi_memory(false);
        config.wasm_memory64(false);
        config.epoch_interruption(true); // I think this is redundant when there's fuel
        config.cranelift_nan_canonicalization(true);
        // config.static_memory_maximum_size(0); // 0 is dynamic-only

        let engine = Engine::new(&config).expect("engine failure");

        let module = WASM_CACHE.with_borrow_mut(|cache| {
            let bytes = cache.entry(wasm_path).or_insert_with_key(|wasm_path| {
                std::fs::read(wasm_path)
                    .expect("failed to read wasm")
                    .into_boxed_slice()
            });

            Module::from_binary(&engine, bytes).expect("failed to compile wasm")
        });

        let mut linker = Linker::new(&engine);

        linker
            .func_wrap("contract", "set_call_result", contract_set_call_result)
            .expect("failed to link `contract.set_call_result` function")
            .func_wrap("log", "write", log_write)
            .expect("failed to link `log.write` function")
            .func_wrap("state", "put", state_put)
            .expect("failed to link `state.put` function")
            .func_wrap("state", "get", state_get)
            .expect("failed to link `state.get` function");

        Self {
            engine,
            module,
            linker,
        }
    }

    #[inline]
    pub fn build(&self) -> TestCrate {
        let Self {
            engine,
            module,
            linker,
        } = self;

        // 0x10000 is 1 << 16 which is the default page-size.
        // this means we're setting the 18 pages.
        let limiter = StoreLimitsBuilder::new().memory_size(18 * 0x10000).build();

        let store_data = StoreData {
            call_result: None,
            state: HashMap::default(),
            limiter,
        };

        let mut store = Store::new(engine, store_data);
        store.set_epoch_deadline(1);
        store
            .set_fuel(u64::MAX)
            .expect("should be able to set fuel");
        store.limiter(|data| &mut data.limiter);

        let instance = linker
            .instantiate(&mut store, module)
            .expect("failed to instantiate wasm");

        let allocate_func = instance
            .get_typed_func(&mut store, ALLOC_FN_NAME)
            .expect("failed to find `alloc` function");

        TestCrate {
            store,
            instance,
            allocate_func,
        }
    }
}

fn contract_set_call_result(mut caller: Caller<'_, StoreData>, ptr: u32, len: u32) {
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

    caller.data_mut().set_result(result);
}

fn log_write(mut caller: Caller<'_, StoreData>, ptr: u32, len: u32) {
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
}

fn state_put(mut caller: Caller<'_, StoreData>, ptr: u32, len: u32) {
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

    caller.data_mut().set(args);
}

fn state_get(mut caller: Caller<'_, StoreData>, ptr: u32, len: u32) -> u32 {
    let Extern::Memory(memory) = caller
        .get_export("memory")
        .expect("memory should be exported")
    else {
        panic!("export `memory` should be of type `Memory`");
    };

    let value = {
        let (ptr, len) = (ptr as usize, len as usize);

        let state_key = memory
            .data(&caller)
            .get(ptr..ptr + len)
            .expect("data should exist");

        caller.data().get(state_key).cloned()
    };

    let Some(value) = value else {
        return 0;
    };

    let Some(Extern::Func(f)) = caller.get_export(ALLOC_FN_NAME) else {
        panic!("export `{ALLOC_FN_NAME}` should exist");
    };

    // TODO:
    // should able to call this unchecked as it is checked
    // immediately after instantiation of any instance
    let alloc = f
        .typed::<AllocParam, AllocReturn>(&mut caller)
        .expect("allocate function should always exist");

    let offset = alloc
        .call(&mut caller, value.len() as u32)
        .expect("failed to allocate memory");

    memory
        .write(&mut caller, offset as usize, &value)
        .expect("failed to write value to memory");

    offset
}

pub struct TestCrate {
    store: Store<StoreData>,
    instance: Instance,
    allocate_func: AllocFn,
}

impl TestCrate {
    #[inline]
    pub fn allocate_context(&mut self) -> AllocReturn {
        self.allocate_params(&())
    }

    // I don't think inlining is actually necessary here since it's a generic method
    #[inline]
    pub fn allocate_params<T: BorshSerialize>(&mut self, params: &T) -> u32 {
        let contract_id = [1; Address::LEN].into_iter();
        let actor = [2; Address::LEN].into_iter();
        let height = 0u64.to_le_bytes().into_iter();
        let timestamp = 0u64.to_le_bytes().into_iter();
        let action_id = [1; ID_LEN].into_iter();

        // this is a hack to create a context since the constructor is private
        let mut ctx = contract_id
            .chain(actor)
            .chain(height)
            .chain(timestamp)
            .chain(action_id)
            .collect();

        params
            .serialize(&mut ctx)
            .expect("failed to serialize params");

        self.allocate(ctx)
    }

    pub fn store_mut(&mut self) -> &mut Store<StoreData> {
        &mut self.store
    }

    pub fn instance(&self) -> &Instance {
        &self.instance
    }

    #[inline]
    pub fn memory(&mut self) -> Memory {
        self.instance
            .get_memory(&mut self.store, "memory")
            .expect("failed to get memory")
    }

    #[inline]
    pub fn allocate(&mut self, data: Vec<u8>) -> AllocReturn {
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

    pub fn get_user_defined_typed_func(&mut self, name: &str) -> UserDefinedFn {
        self.instance
            .get_typed_func::<UserDefinedFnParam, UserDefinedFnReturn>(&mut self.store, name)
            .unwrap_or_else(|_| panic!("failed to find `{name}` function"))
    }
}
