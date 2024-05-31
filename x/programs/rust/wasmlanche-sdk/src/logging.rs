#[macro_export]
macro_rules! dbg {
    () => {
        #[cfg(debug_assertions)]
        {
            let as_string = format!("[{}:{}:{}]", file!(), line!(), column!());
            $crate::log(as_string.as_str());
        }
    };
    ($val:expr $(,)?) => {{
        match $val {
            tmp => {
                #[cfg(debug_assertions)]
                {
                    let as_string = format!(
                        "[{}:{}:{}] {} = {:#?}",
                        file!(),
                        line!(),
                        column!(),
                        stringify!($val),
                        &tmp
                    );
                    $crate::log(as_string.as_str());
                }
                tmp
            }
        }
    }};
    ($($val:expr),+ $(,)?) => {
        ($($crate::dbg!($val)),+,)
    };
}

pub fn register_panic() {
    #[cfg(debug_assertions)]
    {
        use std::panic;
        use std::sync::Once;

        static START: Once = Once::new();
        START.call_once(|| {
            panic::set_hook(Box::new(|info| {
                log(&format!("program {info}"));
            }));
        });
    }
}

/// # Panics
/// Panics if there was an issue regarding memory allocation on the host
pub fn log(text: &str) {
    log_bytes(text.as_bytes());
}

/// Logging facility for debugging purposes
pub(super) fn log_bytes(bytes: &[u8]) {
    #[link(wasm_import_module = "log")]
    extern "C" {
        #[link_name = "write"]
        fn ffi(ptr: *const u8, len: usize);
    }

    unsafe { ffi(bytes.as_ptr(), bytes.len()) };
}
