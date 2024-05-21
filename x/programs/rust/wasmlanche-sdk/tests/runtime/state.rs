use wasmtime::{Caller, Engine, Linker, Module, Store, TypedFunc};
use crate::runtime::utils;

#[test]
fn put_get() {
    let (_tmp_dir, path) = utils::compile_wasm("storage");
    let engine = Engine::default();
    let module = Module::from_file(&engine, &path).expect("failed to create module");
    let mut linker = Linker::new(&engine);
    linker
        .func_wrap(
            "program",
            "set_call_result",
            // |caller: Caller<'_, u32>, param: (*const u8, usize)| {
            |caller: Caller<'_, u32>, ptr: i32, len: i32| {
                let (ptr, len) = (ptr as *const u8, len as usize);
                println!("my host state is: {}", caller.data());
            },
        )
        .expect("failed to link set_call_result function");
    linker
        .func_wrap(
            "state",
            "put",
            |caller: Caller<'_, u32>, ptr: i32, len: i32| -> i32 {
                let (ptr, len) = (ptr as *const u8, len as usize);
                println!("my host state is: {}", caller.data());
                0
            },
        )
        .expect("failed to link set_call_result function");
    linker
        .func_wrap(
            "state",
            "get",
            |caller: Caller<'_, u32>, ptr: i32, len: i32| -> i32 {
                let (ptr, len) = (ptr as *const u8, len as usize);
                println!("my host state is: {}", caller.data());
                0
            },
        )
        .expect("failed to link set_call_result function");
    let mut store: Store<u32> = Store::new(&engine, 4);
    let instance = linker
        .instantiate(&mut store, &module)
        .expect("failed to instantiate linker");
    // let store_func = instance
    //     .get_typed_func::<u32, u32>(&mut store, "store_value_guest")
    //     .expect("failed to get store_func");
    let get_func = instance
        .get_typed_func::<i32, ()>(&mut store, "get_value_guest")
        .expect("failed to get get_func");

    // And finally we can call the wasm!
    // hello.call(&mut store, ())?;
}
