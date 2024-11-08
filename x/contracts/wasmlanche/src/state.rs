// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

extern crate alloc;

use crate::{
    context::{CacheKey, CacheValue},
    host::StateAccessor,
};
use alloc::{boxed::Box, vec::Vec};
use borsh::{from_slice, BorshDeserialize, BorshSerialize};
use bytemuck::NoUninit;
use core::{
    mem::{self, size_of},
    ops::Deref,
};
use displaydoc::Display;
use hashbrown::HashMap;
use sdk_macros::impl_to_pairs;

// maximum number of chunks that can be stored at the key as big endian u16
pub const STATE_MAX_CHUNKS: [u8; 2] = 4u16.to_be_bytes();

#[derive(Clone, Debug, Display)]
pub enum Error {
    /// invalid byte length {0}
    InvalidByteLength(usize),
    /// failed to serialize bytes
    Serialization,
    /// failed to deserialize bytes
    Deserialization,
}

enum Query<V> {
    Found(V),
    Changed(V),
    NotFound,
}

impl<V> From<Option<V>> for Query<V> {
    fn from(value: Option<V>) -> Self {
        match value {
            Some(value) => Query::Found(value),
            None => Query::NotFound,
        }
    }
}

impl<V> Query<V> {
    fn to_option(&self) -> Option<&V> {
        match self {
            Query::Found(value) | Query::Changed(value) => Some(value),
            Query::NotFound => None,
        }
    }

    fn to_option_mut(&mut self) -> Option<&mut V> {
        match self {
            Query::Found(value) | Query::Changed(value) => Some(value),
            Query::NotFound => None,
        }
    }
}

pub struct Cache {
    #[allow(clippy::struct_field_names)]
    cache: HashMap<CacheKey, Query<CacheValue>>,
    byte_count: usize,
    change_count: usize,
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
    #[inline]
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
            byte_count: usize::default(),
            change_count: usize::default(),
        }
    }

    #[inline]
    pub fn store<Pairs: IntoPairs>(&mut self, pairs: Pairs) -> Result<(), Error> {
        let cache = &mut self.cache;

        pairs.into_pairs().into_iter().try_for_each(|result| {
            result.map(|(k, v)| {
                self.change_count += 1;
                self.byte_count += size_of::<u32>() + k.as_ref().len() + size_of::<u32>() + v.len();
                cache.insert(k, Query::Changed(v));
            })
        })?;

        Ok(())
    }

    #[inline]
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
                .to_option()
                .map(Deref::deref)
                .map(from_slice)
                .transpose()
                .map_err(|_| Error::Deserialization)
        } else {
            let key = CacheKey::from(key);
            let bytes = get_bytes(&key).into();
            let value = cache.entry(key).or_insert(bytes);

            value
                .to_option()
                .map(Deref::deref)
                .map(from_slice)
                .transpose()
                .map_err(|_| Error::Deserialization)
        }
    }

    #[inline]
    pub fn delete<K: Schema>(&mut self, key: K) -> Result<Option<K::Value>, Error> {
        let cache = &mut self.cache;
        let key = to_key(key);
        let key = key.as_ref();

        let cache_entry = if let Some(value) = cache.get_mut(key) {
            match value {
                Query::Found(v) => {
                    self.change_count += 1;
                    self.byte_count -= size_of::<u32>() + key.len() + size_of::<u32>();
                    *value = Query::Changed(mem::take(v));
                    value
                }
                Query::Changed(v) if v.is_empty() => value,
                Query::Changed(v) => {
                    self.byte_count -= v.len();
                    value
                }
                Query::NotFound => value,
            }
        } else {
            let key = CacheKey::from(key);

            let value_bytes = if let Some(value_bytes) = get_bytes(&key) {
                self.change_count += 1;
                self.byte_count += size_of::<u32>() + key.len() + size_of::<u32>();

                Query::Changed(value_bytes)
            } else {
                Query::NotFound
            };

            cache.entry(key).or_insert(value_bytes)
        };

        match cache_entry.to_option_mut() {
            None => Ok(None),
            Some(val) if val.is_empty() => Ok(None),
            Some(val) => from_slice(&mem::take(val))
                .map_err(|_| Error::Deserialization)
                .map(Some),
        }
    }

    /// Apply all pending operations to storage and mark the cache as flushed
    pub(super) fn flush(&mut self) {
        let Self {
            cache,
            byte_count,
            change_count,
        } = self;

        if *change_count == 0 {
            return;
        }

        let mut to_delete = Vec::with_capacity(size_of::<u32>() + mem::take(byte_count));
        to_delete.extend(mem::take(change_count).to_le_bytes());

        cache
            .drain()
            .filter_map(|(key, value)| match value {
                Query::Found(_) | Query::NotFound => None,
                Query::Changed(value) => Some((key, value)),
            })
            .for_each(|(key, value)| {
                to_delete.extend(&key.len().to_le_bytes());
                to_delete.extend(key.as_ref());
                to_delete.extend(&value.len().to_le_bytes());
                to_delete.extend(&value);
            });

        StateAccessor::put(&to_delete);
    }
}

fn get_bytes(key: &[u8]) -> Option<CacheValue> {
    #[derive(BorshSerialize)]
    struct GetArgs<'a> {
        key: &'a [u8],
    }

    let key = borsh::to_vec(&GetArgs { key }).expect("failed to serialize args");

    let ptr = StateAccessor::get_bytes(&key);

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
