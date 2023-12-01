//! Temporary storage allocated during the Program runtime.
//! The general pattern for handling memory is to have the
//! host allocate a block of memory and return a pointer to
//! the program. These methods are unsafe as should be used
//! with caution.

/// Represents a pointer to a block of memory allocated by the global allocator.
#[derive(Clone, Copy)]
pub struct Pointer(*mut u8);

impl From<i64> for Pointer {
    fn from(v: i64) -> Self {
        let ptr: *mut u8 = v as *mut u8;
        Pointer(ptr)
    }
}

impl From<Pointer> for *const u8 {
    fn from(pointer: Pointer) -> Self {
        pointer.0.cast_const()
    }
}

impl From<Pointer> for *mut u8 {
    fn from(pointer: Pointer) -> Self {
        pointer.0
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

    /// Attempts return a opy of the bytes from a pointer created by the global allocator.
    /// # Safety
    /// `ptr` must be a pointer to a block of memory created using alloc.
    /// `length` must be the length of the block of memory.
    #[must_use]
    pub unsafe fn range(&self, length: usize) -> Vec<u8> {
        unsafe { std::slice::from_raw_parts(self.ptr.into(), length).to_vec() }
    }

    /// Returns ownership of the bytes and frees the memory block created by the
    /// global allocator once it goes out of scope. Can only be called once.
    /// # Safety
    /// `ptr` must be a pointer to a block of memory created using alloc.
    /// `length` must be the length of the block of memory.
    #[must_use]
    pub unsafe fn range_mut(&self, length: usize) -> Vec<u8> {
        unsafe { Vec::from_raw_parts(self.ptr.into(), length, length) }
    }

    /// Attempts to write the bytes to the programs shared memory.
    /// # Safety
    /// `ptr` must be a pointer to a block of memory created using alloc.
    /// `bytes` must be a slice of bytes with length <= `capacity`.
    pub unsafe fn write<T: AsRef<[u8]>>(&self, bytes: T) {
        self.ptr
            .0
            .copy_from(bytes.as_ref().as_ptr(), bytes.as_ref().len());
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
/// Allocate memory into the instance of Program and return the offset to the
/// start of the block.
/// # Panics
/// Panics if the pointer exceeds the maximum size of an i64.
#[no_mangle]
pub extern "C" fn alloc(len: usize) -> *mut u8 {
    // create a new mutable buffer with capacity `len`
    let mut buf = Vec::with_capacity(len);
    // take a mutable pointer to the buffer
    let ptr = buf.as_mut_ptr();
    // ensure memory pointer is fits in an i64
    // to avoid potential issues when passing
    // across wasm boundary
    assert!(i64::try_from(ptr as u64).is_ok());
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

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_memory() {
        let ptr_len = 5;
        let ptr = allocate(ptr_len);
        let memory = Memory { ptr: Pointer(ptr) };
        let data = vec![1, 2, 3, 4, 5];

        unsafe {
            memory.write(&data);
            let result = memory.range(data.len());
            assert_eq!(result, data);
            let data = vec![5, 1, 2, 4, 5];
            memory.write(&data);
            let result = memory.range(data.len());
            assert_eq!(result, data);
            let data = vec![9, 9, 2];
            memory.write(&data);
            let result = memory.range(ptr_len);
            assert_eq!(result, vec![9, 9, 2, 4, 5]);
        };

        // now out of scope of original range but pointer still valid
        unsafe {
            let result = memory.range(data.len());
            assert_eq!(result, vec![9, 9, 2, 4, 5]);
        }
    }

    #[test]
    fn test_range_owned() {
        let ptr_len = 5;
        let ptr = allocate(ptr_len);
        let memory = Memory { ptr: Pointer(ptr) };
        let data = vec![1, 2, 3, 4, 5];

        unsafe {
            let mut result = memory.range_mut(data.len());
            assert_eq!(result, vec![0; 5]);
            // mutate directly
            result[0] = 1;
            // read from original pointer works in this scope
            let result2 = memory.range(data.len());
            assert_eq!(result2, [1, 0, 0, 0, 0]);
            // write works as expected
            memory.write(&data);
            assert_eq!(result, data);
            // this would panic as the memory is already freed
            // let mut result = memory.range_owned(data.len());

            // ptr allocation dropped here
        };
        // now that we are out of scope ptr is invalid and ub
        unsafe {
            let result = memory.range(data.len());
            assert_ne!(result, data);
        }
    }
}
