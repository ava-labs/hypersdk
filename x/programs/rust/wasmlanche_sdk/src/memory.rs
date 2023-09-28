//! Temporary storage allocated during the Program runtime.

/// Represents a pointer to a block of memory allocated by the global allocator.
pub struct Pointer {
    raw: *mut u8,
}

impl Pointer {
    #[must_use]
    pub fn new(ptr: i64) -> Self {
        Self {
            raw: ptr as *mut u8,
        }
    }
}

/// Represents a block of memory allocated by the global allocator.
pub struct Memory {
    ptr: Pointer,
}

impl Memory {
    #[must_use]
    pub fn new(ptr: Pointer) -> Self {
        Self { ptr }
    }
    /// Attempts return owned bytes from a pointer created by the global allocator.
    ///
    /// # Safety
    /// `ptr` must be a pointer to a block of memory created using alloc.
    /// `length` must be the length of the block of memory.
    #[must_use]
    pub unsafe fn range(&self, length: usize) -> Vec<u8> {
        unsafe { Vec::from_raw_parts(self.ptr.raw, length, length) }
    }

    /// Attempts to write the bytes to the programs shared memory.
    ///
    /// # Safety
    /// `ptr` must be a pointer to a block of memory created using alloc.
    /// `bytes` must be a slice of bytes with length <= `capacity`.
    pub unsafe fn write(&self, bytes: &[u8]) {
        unsafe { self.ptr.raw.copy_from(bytes.as_ptr(), bytes.len()) }
    }
}

/// Attempts to allocate a block of memory of size `len` and returns a pointer
/// to the start of the block.
#[must_use]
pub fn allocate(len: usize) -> *mut u8 {
    alloc(len)
}

/// Attempts to deallocates the memory block at `ptr` with a given `capacity`.
///
/// # Safety
/// `ptr` must be a valid pointer to a block of memory created using alloc.
pub unsafe fn deallocate(ptr: *mut u8, capacity: usize) {
    unsafe { dealloc(ptr, capacity) }
}

/* memory functions ------------------------------------------- */
// https://radu-matei.com/blog/practical-guide-to-wasm-memory/

/// Allocate memory into the instance of Program and return the offset to the
/// start of the block.
#[no_mangle]
pub extern "C" fn alloc(len: usize) -> *mut u8 {
    // create a new mutable buffer with capacity `len`
    let mut buf = Vec::with_capacity(len);
    // take a mutable pointer to the buffer
    let ptr = buf.as_mut_ptr();
    // take ownership of the memory block and
    // ensure that its destructor is not
    // called when the object goes out of scope
    // at the end of the function
    std::mem::forget(buf);
    // return the pointer so the runtime
    // can write data at this offset
    ptr
}

/// # Safety
/// `ptr` must be a pointer to a block of memory.
///
/// deallocates the memory block at `ptr` with a given `capacity`.
#[no_mangle]
pub unsafe extern "C" fn dealloc(ptr: *mut u8, capacity: usize) {
    // always deallocate the full capacity, initialize vs uninitialized memory is irrelevant here
    let data = Vec::from_raw_parts(ptr, capacity, capacity);
    std::mem::drop(data);
}
