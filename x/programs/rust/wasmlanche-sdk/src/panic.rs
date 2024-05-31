pub fn register_panic() {
    use crate::log;
    use std::panic;
    use std::sync::Once;

    static START: Once = Once::new();
    START.call_once(|| {
        panic::set_hook(Box::new(|info| {
            log(&format!("program {}", info));
        }));
    });
}
