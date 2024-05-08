use crate::state::Error;

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

/// # Panics
/// Panics if there was an issue regarding memory allocation on the host
pub fn log(text: &str) {
    log_bytes(text.as_bytes()).expect("failed to log value");
}

/// Logging facility for debugging purposes
pub(super) fn log_bytes(bytes: &[u8]) -> Result<(), Error> {
    #[link(wasm_import_module = "state")]
    extern "C" {
        #[link_name = "log"]
        fn ffi(ptr: *const u8, len: usize) -> i32;
    }

    let result = unsafe { ffi(bytes.as_ptr(), bytes.len()) };
    match result {
        0 => Ok(()),
        _ => Err(Error::Log),
    }
}
