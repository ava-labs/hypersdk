mod bindings {
    #![allow(non_upper_case_globals)]
    #![allow(non_camel_case_types)]
    #![allow(non_snake_case)]
    #![allow(dead_code)]

    use wasmlanche_sdk::Address as SdkAddress;
    // include the generated bindings
    // reference: https://rust-lang.github.io/rust-bindgen/tutorial-3.html
    include!(concat!(env!("OUT_DIR"), "/bindings.rs"));

    impl std::ops::Deref for Bytes {
        type Target = [u8];

        fn deref(&self) -> &Self::Target {
            unsafe { std::slice::from_raw_parts(self.data, self.length as usize) }
        }
    }

    impl From<SdkAddress> for Address {
        fn from(value: SdkAddress) -> Self {
            Address {
                // # Safety:
                // Address is a simple wrapper around an array of bytes
                // this will fail at compile time if the size is changed
                address: unsafe { std::mem::transmute::<SdkAddress, [libc::c_uchar; 33]>(value) },
            }
        }
    }
}

mod simulator;
mod state;

pub use simulator::Simulator;
