//! Temporary storage allocated during the Program runtime.
//! The general pattern for handling memory is to have the
//! host allocate a block of memory and return a pointer to
//! the program. These methods are unsafe as should be used
//! with caution.

use crate::errors::StateError;

/// Represents a pointer to a block of memory allocated by the global allocator.
#[derive(Clone, Copy)]
pub struct Pointer(*const u8);

impl Pointer {
    #[must_use]
    pub fn new(ptr: u32) -> Self {
        Self(ptr as *const u8)
    }

    pub fn is_null(&self) -> bool {
        self.0.is_null()
    }
}

impl From<Pointer> for *const u8 {
    fn from(pointer: Pointer) -> Self {
        pointer.0
    }
}

impl From<Pointer> for *mut u8 {
    fn from(pointer: Pointer) -> Self {
        pointer.0.cast_mut()
    }
}

/// Represents a block of memory allocated by the global allocator.
pub struct Memory {
    ptr: Pointer,
    length: usize,
}

impl Memory {
    #[must_use]
    pub fn new(ptr: Pointer, length: usize) -> Self {
        Self { ptr, length }
    }

    /// Attempts return a copy of the bytes from a pointer created by the global allocator.
    /// # Safety
    /// `ptr` must be a pointer to a block of memory created using alloc.
    /// `length` must be the length of the block of memory.
    #[must_use]
    pub unsafe fn to_vec(&self) -> Vec<u8> {
        unsafe { std::slice::from_raw_parts(self.ptr.into(), self.length).to_vec() }
    }

    /// Returns ownership of the bytes and frees the memory block created by the
    /// global allocator once it goes out of scope. Can only be called once.
    /// # Safety
    /// `ptr` must be a pointer to a block of memory created using alloc.
    /// `length` must be the length of the block of memory.
    #[must_use]
    pub unsafe fn into_vec(&self) -> Vec<u8> {
        unsafe { Vec::from_raw_parts(self.ptr.into(), self.length, self.length) }
    }
}

/// converts an i64 from the host to the value
impl TryFrom<i64> for Memory {
    type Error = StateError;

    fn try_from(value: i64) -> Result<Self, Self::Error> {
        if value.is_negative() {
            return Err(StateError::Underflow);
        }
        let value_bytes = value.to_be_bytes();
        let (ptr, len) = try_parse_raw_parts(value)?;
        Ok(Self::new(ptr, len))
    }
}

impl From<Memory> for Vec<u8> {
    fn from(memory: Memory) -> Self {
        unsafe { memory.to_vec() }
    }
}

/// converts the value to BigEndian bytes and then extracting the first 4 bytes as length and the later as a pointer.
pub fn try_parse_raw_parts(value: i64) -> Result<(Pointer, usize), StateError> {
    let data = value.to_be_bytes();
    // extract lower 4 bytes as length
    let len = usize::try_from(u32::from_be_bytes(
        value.to_be_bytes()[0..4]
            .try_into()
            .map_err(|e| StateError::TryFromSlice(e))?,
    ))
    .map_err(|e| StateError::TryFromInt(e))?;

    // extract upper 4 bytes as pointer
    let ptr_u32 = u32::from_be_bytes(
        data[4..8]
            .try_into()
            .map_err(|e| StateError::TryFromSlice(e))?,
    );

    let ptr = Pointer::new(ptr_u32);
    if ptr.is_null() {
        return Err(StateError::NullPointer);
    }

    Ok((ptr, len))
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
