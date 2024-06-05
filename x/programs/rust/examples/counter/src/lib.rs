mod counter;
#[cfg(feature = "bindings")]
pub use counter::bindings::*;

#[cfg(not(feature = "bindings"))]
pub use counter::*;
