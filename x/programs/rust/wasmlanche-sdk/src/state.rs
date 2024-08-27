// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use crate::{
    context::{CacheKey, CacheValue},
    memory::HostPtr,
    types::Address,
};
use borsh::{from_slice, BorshDeserialize, BorshSerialize};
use bytemuck::NoUninit;
use core::mem::{self, size_of};
use hashbrown::HashMap;
use sdk_macros::impl_to_pairs;

// maximum number of chunks that can be stored at the key as big endian u16
pub const STATE_MAX_CHUNKS: [u8; 2] = 4u16.to_be_bytes();

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

pub struct Cache {
    cache: HashMap<CacheKey, Option<CacheValue>>,
}

impl Drop for Cache {
    fn drop(&mut self) {
        self.flush();
    }
}

pub type PrefixType = u8;
pub type MaxChunksType = [u8; size_of::<u16>()];

#[doc(hidden)]
#[derive(Clone, Copy)]
#[repr(C, packed)]
pub struct PrefixedKey<K: NoUninit> {
    prefix: PrefixType,
    key: K,
    max_chunks: MaxChunksType,
}

impl<K: NoUninit> AsRef<[u8]> for PrefixedKey<K> {
    fn as_ref(&self) -> &[u8] {
        bytemuck::bytes_of(self)
    }
}

// # Safety:
// this is safe because we generate a compile type check for every Key
// It's also fine as long as we use `repr(C, packed)` for the struct
unsafe impl<K: NoUninit> NoUninit for PrefixedKey<K> {}

#[doc(hidden)]
#[macro_export]
macro_rules! prefixed_key_size_check {
    ($($module:ident)::*, $ty:ty) => {
        const _: fn() = || {
            type TypeForTypeTest = $ty;
            type PrefixedKey = $($module::)*PrefixedKey<TypeForTypeTest>;
            const SIZE: usize = ::core::mem::size_of::<$($module::)*PrefixType>()
                + ::core::mem::size_of::<$($module::)*MaxChunksType>()
                + ::core::mem::size_of::<TypeForTypeTest>();
            #[doc(hidden)]
            struct TypeWithoutPadding([u8; SIZE]);
            let _ = ::core::mem::transmute::<PrefixedKey, TypeWithoutPadding>;
        };
    };
}

prefixed_key_size_check!(self::macro_types, u32);

/// A trait for defining the associated value for a given state-key.
/// This trait is not meant to be implemented manually but should instead be implemented with the [`state_schema!`](crate::state_schema) macro.
/// # Safety
/// Do not implement this trait manually. Use the [`state_schema`](crate::state_schema) macro instead.
pub unsafe trait Schema: NoUninit {
    type Value: BorshSerialize + BorshDeserialize;

    fn prefix() -> u8;

    /// # Errors
    /// Will return an error when there's an issue with deserialization
    fn get(self, state_cache: &mut Cache) -> Result<Option<Self::Value>, Error> {
        let key = to_key(self);
        state_cache.get_with_raw_key(key.as_ref())
    }
}

pub(crate) fn to_key<K: Schema>(key: K) -> PrefixedKey<K> {
    PrefixedKey {
        prefix: K::prefix(),
        key,

        // TODO: don't use this const and instead adjust this per stored type
        max_chunks: STATE_MAX_CHUNKS,
    }
}

impl Default for Cache {
    fn default() -> Self {
        Self::new()
    }
}

impl Cache {
    #[must_use]
    pub fn new() -> Self {
        Self {
            cache: HashMap::default(),
        }
    }

    pub fn store<Pairs: IntoPairs>(&mut self, pairs: Pairs) -> Result<(), Error> {
        let cache = &mut self.cache;

        pairs.into_pairs().into_iter().try_for_each(|result| {
            result.map(|(k, v)| {
                cache.insert(k, Some(v));
            })
        })?;

        Ok(())
    }

    pub fn store_by_key<K>(&mut self, key: K, value: K::Value) -> Result<(), Error>
    where
        K: Schema,
    {
        self.store(((key, value),))
    }

    pub(crate) fn get_with_raw_key<V>(&mut self, key: &[u8]) -> Result<Option<V>, Error>
    where
        V: BorshDeserialize,
    {
        let cache = &mut self.cache;

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

    pub fn delete<K: Schema>(&mut self, key: K) -> Result<Option<K::Value>, Error> {
        let cache = &mut self.cache;
        let key = to_key(key);

        let cache_entry = if let Some(value) = cache.get_mut(key.as_ref()) {
            value
        } else {
            let key = CacheKey::from(key.as_ref());
            let value_bytes = get_bytes(&key);
            cache.entry(key).or_insert(value_bytes)
        };

        match cache_entry {
            None => Ok(None),
            Some(val) if val.is_empty() => Ok(None),
            Some(val) => from_slice(&mem::take(val))
                .map_err(|_| Error::Deserialization)
                .map(Some),
        }
    }

    /// Apply all pending operations to storage and mark the cache as flushed
    pub(super) fn flush(&mut self) {
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

        let cache = &mut self.cache;

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

pub trait Sealed {}

pub trait IntoPairs: Sealed {
    fn into_pairs(self) -> impl IntoIterator<Item = Result<(CacheKey, CacheValue), Error>>;
}

impl_to_pairs!(10, Schema);
impl_to_pairs!(9, Schema);
impl_to_pairs!(8, Schema);
impl_to_pairs!(7, Schema);
impl_to_pairs!(6, Schema);
impl_to_pairs!(5, Schema);
impl_to_pairs!(4, Schema);
impl_to_pairs!(3, Schema);
impl_to_pairs!(2, Schema);
impl_to_pairs!(1, Schema);

#[doc(hidden)]
pub mod macro_types {
    pub use super::{MaxChunksType, PrefixType, PrefixedKey, Schema};
}
