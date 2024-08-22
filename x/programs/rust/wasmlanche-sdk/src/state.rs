// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use crate::{
    context::{CacheKey, CacheValue},
    memory::HostPtr,
    types::Address,
    Context,
};
use borsh::{from_slice, BorshDeserialize, BorshSerialize};
use bytemuck::NoUninit;
use sdk_macros::impl_to_pairs;
use std::{cell::RefCell, collections::HashMap};

pub const STATE_MAX_CHUNKS: [u8;2] = [0,4];

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

#[doc(hidden)]
#[derive(Clone, Copy)]
#[repr(C, packed)]
pub struct PrefixedKey<K: NoUninit> {
    prefix: u8,
    key: K,
    size: [u8;2],
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

const _: fn() = || {
    #[doc(hidden)]
    struct TypeWithoutPadding([u8; 3 + ::core::mem::size_of::<u32>()]);
    let _ = ::core::mem::transmute::<crate::state::PrefixedKey<u32>, TypeWithoutPadding>;
};

/// A trait for defining the associated value for a given state-key.
/// This trait is not meant to be implemented manually but should instead be implemented with the [`state_schema!`](crate::state_schema) macro.
/// # Safety
/// Do not implement this trait manually. Use the [`state_schema`](crate::state_schema) macro instead.
pub unsafe trait Schema: NoUninit {
    type Value: BorshSerialize + BorshDeserialize;

    fn prefix() -> u8;

    /// # Errors
    /// Will return an error when there's an issue with deserialization
    fn get(self, context: &mut Context) -> Result<Option<Self::Value>, Error> {
        let key = to_key(self);
        context.get_with_raw_key(key.as_ref())
    }
}

pub(crate) fn to_key<K: Schema>(key: K) -> PrefixedKey<K> {
    PrefixedKey {
        prefix: K::prefix(),
        key,
        size: STATE_MAX_CHUNKS,
    }
}

impl<'a> State<'a> {
    #[must_use]
    pub fn new(cache: &'a RefCell<HashMap<CacheKey, Option<CacheValue>>>) -> Self {
        Self { cache }
    }

    pub fn store<Pairs: IntoPairs>(self, pairs: Pairs) -> Result<(), Error> {
        let cache = &mut self.cache.borrow_mut();

        pairs.into_pairs().into_iter().try_for_each(|result| {
            result.map(|(k, v)| {
                cache.insert(k, Some(v));
            })
        })?;

        Ok(())
    }

    pub fn store_by_key<K>(self, key: K, value: K::Value) -> Result<(), Error>
    where
        K: Schema,
    {
        self.store(((key, value),))
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
