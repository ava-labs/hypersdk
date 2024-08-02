use std::env;
use std::path::Path;
use std::path::PathBuf;

// build the go library via
// go build -buildmode=c-shared -o libsimulator.so ffi.go
fn main() {
    let dir = env::var("CARGO_MANIFEST_DIR").unwrap();
    println!("cargo:rustc-link-search=native={}", dir);
    println!("cargo:rustc-link-lib=dylib=simulator");
    
    // Rerun the script if simulator.so changes
    println!(
        "cargo:rerun-if-changed={}",
        Path::new(&dir).join("simulator.so").display()
    );


    // Tell cargo to tell rustc to link the system bzip2
    // shared library in /common.
    // println!("cargo:rustc-link-lib=ctypes");
    let bindings = bindgen::Builder::default()
    .ctypes_prefix("libc")
    // .use_core()
    .header("./common/types.h")
    .parse_callbacks(Box::new(bindgen::CargoCallbacks::new()))
    .generate().expect("unable to generate bindings");
     let out_path = PathBuf::from(env::var("OUT_DIR").unwrap());
    bindings.write_to_file(out_path.join("bindings.rs"))
    .expect("Couldn't write bindings!");
}
