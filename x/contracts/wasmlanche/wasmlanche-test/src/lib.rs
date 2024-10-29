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
const PROFILE: &str = "release";

type AllocParam = u32;
type AllocReturn = u32;
type AllocFn = TypedFunc<AllocParam, AllocReturn>;
pub type UserDefinedFnParam = u32;
pub type UserDefinedFnReturn = ();
pub type UserDefinedFn = TypedFunc<UserDefinedFnParam, UserDefinedFnReturn>;
type StateKey = Box<[u8]>;
type StateValue = Box<[u8]>;
type StoreData = (Option<Vec<u8>>, HashMap<StateKey, StateValue>);

pub struct Builder<'a> {
    crate_name: &'a str,
}

impl<'a> Builder<'a> {
    pub fn new(crate_name: &'a str) -> Self {
        Self { crate_name }
    }

    pub fn build(self) -> TestCrate {
        let Self { crate_name } = self;

        let wasm_path = {
            let manifest_dir = std::env::var("CARGO_MANIFEST_DIR").unwrap();
            let manifest_dir = std::path::Path::new(&manifest_dir);
            let test_crate_dir = manifest_dir.join("tests").join(crate_name);
            let target_dir = std::env::var("CARGO_TARGET_DIR")
                .map(PathBuf::from)
                .unwrap_or_else(|_| manifest_dir.join("target"));

            let status = Command::new("cargo")
                .arg("build")
                .arg("--package")
                .arg(crate_name)
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
                .join(crate_name.replace('-', "_"))
                .with_extension("wasm")
        };

        TestCrate::new(wasm_path)
    }
}

pub struct TestCrate {
    store: Store<StoreData>,
    instance: Instance,
    allocate_func: AllocFn,
}

impl TestCrate {
    fn new(wasm_path: impl AsRef<Path>) -> Self {
        let mut config = Config::new();
        let mut allocation_strategy = PoolingAllocationConfig::default();
        // not sure why, but this seems to be the min
        allocation_strategy.max_memory_size(18 * 0x10000);

        let allocation_strategy = InstanceAllocationStrategy::Pooling(allocation_strategy);
        config.allocation_strategy(allocation_strategy);

        let engine = Engine::new(&config).expect("engine failure");
        let mut store: Store<StoreData> = Store::new(&engine, Default::default());
        let mut linker = Linker::new(store.engine());
        let module =
            Module::from_file(store.engine(), wasm_path.as_ref()).expect("failed to load wasm");

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

        // let highest_address_func = instance
        //     .get_typed_func(&mut store, "highest_allocated_address")
        //     .expect("failed to find `highest_allocated_address` function");

        // let always_true_func = instance
        //     .get_typed_func(&mut store, "always_true")
        //     .expect("failed to find `always_true` function");
        // let combine_last_bit_of_each_id_byte_func = instance
        //     .get_typed_func(&mut store, "combine_last_bit_of_each_id_byte")
        //     .expect("combine_last_bit_of_each_id_byte should be a function");

        Self {
            store,
            instance,
            allocate_func,
            // highest_address_func,
            // always_true_func,
            // combine_last_bit_of_each_id_byte_func,
        }
    }

    pub fn write_context(&mut self) -> AllocReturn {
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

    pub fn store_mut(&mut self) -> &mut Store<StoreData> {
        &mut self.store
    }

    pub fn instance(&self) -> &Instance {
        &self.instance
    }

    pub fn memory(&mut self) -> Memory {
        self.instance
            .get_memory(&mut self.store, "memory")
            .expect("failed to get memory")
    }

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
            .expect(&format!("failed to find `{name}` function"))
    }
}
