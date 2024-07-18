use std::path::Path;
pub(crate) fn get_path() -> &'static str {
    let path = env!("SIMULATOR_PATH");

    if !Path::new(path).exists() {
        eprintln!();
        eprintln!("Simulator binary not found at path: {path}");
        eprintln!();
        eprintln!("Please run `cargo clean -p simulator` and rebuild your dependent crate.");
        eprintln!();

        panic!("Simulator binary not found, must rebuild simulator");
    }

    path
}
