use crate::{
    context::{CacheKey, CacheValue},
    memory::HostPtr,
    types::Address,
    Context,
};
use borsh::{from_slice, BorshDeserialize, BorshSerialize};
use bytemuck::NoUninit;
use sdk_macros::impl_to_pairs;
use std::{
    cell::RefCell,
    collections::{hash_map::Entry, HashMap},
    hash::Hash,
};

#[derive(Clone, thiserror::Error, Debug)]
pub enum Error {
    #[error("invalid byte length: {0}")]
    InvalidByteLength(usize),

    #[error("failed to serialize bytes")]
    Serialization,

    #[error("failed to deserialize bytes")]
    Deserialization,
}

/// Gets the balance for the specified address
/// # Panics
/// Panics if there was an issue deserializing the balance
#[must_use]
pub fn get_balance(account: Address) -> u64 {
    #[link(wasm_import_module = "balance")]
    extern "C" {
        #[link_name = "get"]
        fn get(ptr: *const u8, len: usize) -> HostPtr;
    }
    let ptr = borsh::to_vec(&account).expect("failed to serialize args");
    let bytes = unsafe { get(ptr.as_ptr(), ptr.len()) };

    borsh::from_slice(&bytes).expect("failed to deserialize the balance")
}

pub struct State<'a> {
    cache: &'a RefCell<HashMap<CacheKey, Option<CacheValue>>>,
}

/// Key trait for program state keys
/// # Safety
/// This trait should only be implemented using the [`state_keys`](crate::state_keys) macro.
pub unsafe trait Key: Copy + PartialEq + Eq + Hash + BorshSerialize {}

#[derive(Clone, Copy)]
#[repr(C, packed)]
pub(crate) struct PrefixedKey<K: NoUninit> {
    prefix: u8,
    key: K,
}

impl<K: NoUninit> AsRef<[u8]> for PrefixedKey<K> {
    fn as_ref(&self) -> &[u8] {
        bytemuck::bytes_of(self)
    }
}

// TODO: deal wiht padding?
unsafe impl<K: NoUninit> NoUninit for PrefixedKey<K> {}

// TODO: use bytemuck::must_cast (behind feature flag)
pub unsafe trait Schema: NoUninit {
    type Value: BorshSerialize + BorshDeserialize;

    fn prefix() -> u8;

    fn get(self, context: &mut Context) -> Result<Option<Self::Value>, Error> {
        let key = to_key(self);
        context.get_with_raw_key(key.as_ref())
    }
}

pub(crate) fn to_key<K: Schema>(key: K) -> PrefixedKey<K> {
    PrefixedKey {
        prefix: K::prefix(),
        key,
    }
}

impl<'a> State<'a> {
    #[must_use]
    pub fn new(cache: &'a RefCell<HashMap<CacheKey, Option<CacheValue>>>) -> Self {
        Self { cache }
    }

    /// Store a list of tuple of key and value to the host storage.
    /// # Errors
    /// Returns an [`Error`] if the key or value cannot be
    /// serialized or if the host fails to handle the operation.
    pub fn store<Pairs: IntoPairs>(self, pairs: Pairs) -> Result<(), Error> {
        let cache = &mut self.cache.borrow_mut();

        pairs.into_pairs().into_iter().try_for_each(|result| {
            result.map(|(k, v)| {
                cache.insert(k, Some(v));
            })
        })?;

        Ok(())
    }

    /// Store a key and value to the host storage. If the key already exists,
    /// the value will be overwritten.
    /// # Errors
    /// Returns an [`Error`] if the key or value cannot be
    /// serialized or if the host fails to handle the operation.
    pub fn store_by_key<K>(self, key: K, value: K::Value) -> Result<(), Error>
    where
        K: Schema,
    {
        self.store(((key, value),))
    }

    /// Get a value from the host's storage.
    ///
    /// Note: The pointer passed to the host are only valid for the duration of this
    /// function call. This function will take ownership of the pointer and free it.
    ///
    /// # Errors
    /// Returns an [`Error`] if the key cannot be serialized or if
    /// the host fails to read the key and value.
    /// # Panics
    /// Panics if the value cannot be converted from i32 to usize.
    pub fn get<K: Schema>(self, key: K) -> Result<Option<K::Value>, Error> {
        self.get_mut_with(key, |x| from_slice(x))
    }

    pub(crate) fn get_with_raw_key<V>(self, key: &[u8]) -> Result<Option<V>, Error>
    where
        V: BorshDeserialize,
    {
        let mut cache = self.cache.borrow_mut();

        if let Some(value) = cache.get(key) {
            value
                .as_deref()
                .map(from_slice)
                .transpose()
                .map_err(|_| Error::Deserialization)
        } else {
            let key = CacheKey::from(key);
            let bytes = get_bytes(&key);
            cache
                .entry(key)
                .or_insert(bytes)
                .as_deref()
                .map(from_slice)
                .transpose()
                .map_err(|_| Error::Deserialization)
        }
    }

    /// Delete a value from the hosts's storage.
    /// # Errors
    /// Returns an [Error] if the value is inexistent
    /// or if the key cannot be serialized
    /// or if the host fails to delete the key and the associated value
    pub fn delete<K: Schema>(self, key: K) -> Result<Option<K::Value>, Error> {
        self.get_mut_with(key, |val| from_slice(&std::mem::take(val)))
    }

    /// Only used internally
    /// The closure is only called if the key already exists in the cache
    /// The closure may mutate the value and return anything it likes for deserializaiton
    fn get_mut_with<K: Schema, F>(self, key: K, f: F) -> Result<Option<K::Value>, Error>
    where
        F: FnOnce(&mut CacheValue) -> borsh::io::Result<K::Value>,
    {
        let mut cache = self.cache.borrow_mut();
        // TODO: should only need to allocate on cache-miss
        let key = CacheKey::from(to_key(key).as_ref());

        let cache_entry = match cache.entry(key) {
            Entry::Occupied(entry) => entry.into_mut(),
            Entry::Vacant(entry) => {
                let bytes = {
                    let key = entry.key();
                    get_bytes(key)
                };
                entry.insert(bytes)
            }
        };

        match cache_entry {
            None => Ok(None),
            Some(val) if val.is_empty() => Ok(None),
            Some(val) => f(val).map_err(|_| Error::Deserialization).map(Some),
        }
    }

    /// Apply all pending operations to storage and mark the cache as flushed
    pub(super) fn flush(&self) {
        #[link(wasm_import_module = "state")]
        extern "C" {
            #[link_name = "put"]
            fn put(ptr: *const u8, len: usize);
        }

        #[derive(BorshSerialize)]
        struct PutArgs<Key> {
            key: Key,
            value: CacheValue,
        }

        let mut cache = self.cache.borrow_mut();

        let args: Vec<_> = cache
            .drain()
            .filter_map(|(key, val)| val.map(|value| PutArgs { key, value }))
            .collect();

        if !args.is_empty() {
            let serialized_args = borsh::to_vec(&args).expect("failed to serialize");
            unsafe { put(serialized_args.as_ptr(), serialized_args.len()) };
        }
    }
}

fn get_bytes(key: &[u8]) -> Option<CacheValue> {
    #[link(wasm_import_module = "state")]
    extern "C" {
        #[link_name = "get"]
        fn get_bytes(ptr: *const u8, len: usize) -> HostPtr;
    }

    #[derive(BorshSerialize)]
    struct GetArgs<'a> {
        key: &'a [u8],
    }

    let key = borsh::to_vec(&GetArgs { key }).expect("failed to serialize args");

    let ptr = unsafe { get_bytes(key.as_ptr(), key.len()) };

    if ptr.is_null() {
        None
    } else {
        Some(ptr.into())
    }
}

trait Sealed {}

#[allow(private_bounds)]
pub trait IntoPairs: Sealed {
    fn into_pairs(self) -> impl IntoIterator<Item = Result<(CacheKey, CacheValue), Error>>;
}

impl_to_pairs!(10, Schema, BorshSerialize);
impl_to_pairs!(9, Schema, BorshSerialize);
impl_to_pairs!(8, Schema, BorshSerialize);
impl_to_pairs!(7, Schema, BorshSerialize);
impl_to_pairs!(6, Schema, BorshSerialize);
impl_to_pairs!(5, Schema, BorshSerialize);
impl_to_pairs!(4, Schema, BorshSerialize);
impl_to_pairs!(3, Schema, BorshSerialize);
impl_to_pairs!(2, Schema, BorshSerialize);
impl_to_pairs!(1, Schema, BorshSerialize);
