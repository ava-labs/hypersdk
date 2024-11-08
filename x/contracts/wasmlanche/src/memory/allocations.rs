// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

//! Safety:
//! Only safe to use in a single-threaded environment

use cfg_if::cfg_if;
use hashbrown::HashMap;

// Map of pointer to the length of its content on the heap
type LenMap = HashMap<*const u8, usize>;

/// Get size of allocation at `key`
#[inline]
pub fn get(key: *const u8) -> Option<usize> {
    ALLOCATIONS.with_borrow(|map| map.get(&key).copied())
}

/// Insert size of allocation at `key`
#[inline]
pub fn insert(key: *const u8, value: usize) {
    ALLOCATIONS.with_borrow_mut(|map| map.insert(key, value));
}

/// Remove size of allocation at `key` in preparation of deallocation or move
#[inline]
pub fn remove(key: *const u8) -> Option<usize> {
    ALLOCATIONS.with_borrow_mut(|map| map.remove(&key))
}

cfg_if! {
    if #[cfg(feature = "test")] {
        use std::cell::RefCell;
        std::thread_local! {
            static ALLOCATIONS: RefCell<LenMap> = RefCell::new(HashMap::new());
        }
    } else {
        // Here, we don't have access to std
        // this code is only thread safe in that it will panic upon concurrent access
        extern crate alloc;

        use alloc::boxed::Box;
        use core::{
            sync::atomic::{AtomicPtr, AtomicBool, Ordering::{Acquire, Relaxed, Release}},
            ops::{Deref, DerefMut},
        };

        static ALLOCATIONS: SingletonLenMap = SingletonLenMap::new();

        struct Guard<'a> {
            map: *mut LenMap,
            lock: &'a AtomicBool,
        }

        impl Drop for Guard<'_> {
            fn drop(&mut self) {
                self.lock.store(false, Release);
            }
        }

        impl Deref for Guard<'_> {
            type Target = LenMap;

            fn deref(&self) -> &Self::Target {
                // Safety:
                // Can't create a guard without initializing first
                unsafe { self.map.as_ref().expect("uninitialized") }
            }
        }

        impl DerefMut for Guard<'_> {
            fn deref_mut(&mut self) -> &mut Self::Target {
                // Safety:
                // Can't create a guard without initializing first
                unsafe { self.map.as_mut().expect("uninitialized") }
            }
        }

        /// A singleton wrapping a pointer to the [LenMap]
        struct SingletonLenMap {
            map: AtomicPtr<LenMap>,
            lock: AtomicBool,
        }

        impl SingletonLenMap {
            const fn new() -> Self {
                let map = AtomicPtr::new(core::ptr::null_mut());
                let lock = AtomicBool::new(false);
                Self { map, lock }
            }

            /// Initialize is for lazy-laoding the map.
            /// Safety:
            /// Data-races are prevented with a lock
            #[must_use]
            #[inline]
            fn init_and_lock(&self) -> Guard<'_> {
                assert!(!self.lock.swap(true, Acquire), "already accessed");

                let mut map = self.map.load(Relaxed);

                if map.is_null() {
                    map = Box::into_raw(Box::new(HashMap::new()));
                    self.map.store(map, Relaxed);
                }

                Guard { map, lock: &self.lock }
            }

            /// Matches the `LocalKey` API
            #[inline]
            fn with_borrow<F: FnOnce(&LenMap) -> R, R>(&self, f: F) -> R {
                let map = self.init_and_lock();
                f(&map)
            }

            /// Matches the `LocalKey` API
            #[inline]
            fn with_borrow_mut<F: FnOnce(&mut LenMap) -> R, R>(&self, f: F) -> R {
                let mut map = self.init_and_lock();
                f(&mut map)
            }
        }

        #[cfg(test)]
        mod tests {
            use super::*;
            use core::sync::atomic::Ordering::SeqCst;

            #[test]
            #[should_panic(expected = "uninitialized")]
            fn deref_null_panics() {
                let map = core::ptr::null_mut();
                let lock = &AtomicBool::new(false);
                let guard = Guard { map, lock };
                let _ = &*guard;
            }

            #[test]
            #[should_panic(expected = "uninitialized")]
            fn deref_mut_null_panics() {
                let map = core::ptr::null_mut();
                let lock = &AtomicBool::new(false);
                let mut guard = Guard { map, lock };
                let _ = &mut *guard;
            }

            #[test]
            #[should_panic(expected = "already accessed")]
            fn cannot_initialize_singleton_twice() {
                let singleton = SingletonLenMap::new();
                let _first = singleton.init_and_lock();
                let _second = singleton.init_and_lock();
            }

            #[test]
            fn assure_lock_is_dropped() {
                let singleton = SingletonLenMap::new();
                let first = singleton.init_and_lock();
                drop(first);
                let _second = singleton.init_and_lock();
            }

            // assure initialization
            #[test]
            fn is_initialized() {
                let singleton = SingletonLenMap::new();

                let inner = singleton.map.load(SeqCst);
                assert!(inner.is_null());

                let guard = singleton.init_and_lock();
                drop(guard);

                let inner = singleton.map.load(SeqCst);
                assert!(!inner.is_null());
            }

            // make sure code is still sound in a multi-threaded environment
            #[cfg(not(target_arch = "wasm32"))]
            #[test]
            #[should_panic(expected = "already accessed")]
            fn panic_on_concurrent_access() {
                fn test_with_iterations(n: usize) -> impl FnOnce() {
                    static SINGLETON: SingletonLenMap = SingletonLenMap::new();

                    move || {
                        for _ in 0..n {
                            SINGLETON.with_borrow(|map| {
                                assert!(map.is_empty());
                            });
                        }
                    }
                }

                std::thread::scope(|scope| {
                    // run enough times to make sure there's a collision
                    let t1 = scope.spawn(test_with_iterations(10_000));
                    let t2 = scope.spawn(test_with_iterations(100));
                    let (t1, t2) = (t1.join(), t2.join());

                    if let Err(e) = t1.and(t2) {
                        std::panic::resume_unwind(e);
                    }
                });
            }

            #[test]
            #[should_panic(expected = "already accessed")]
            fn cannot_borrow_twice() {
                let singleton = SingletonLenMap::new();
                singleton.with_borrow(|map| {
                    assert!(map.is_empty());
                    singleton.with_borrow(|map| {
                        assert!(map.is_empty());
                    });
                });
            }

            #[test]
            #[should_panic(expected = "already accessed")]
            fn cannot_borrow_mut_twice() {
                let singleton = SingletonLenMap::new();
                singleton.with_borrow_mut(|map| {
                    assert!(map.is_empty());
                    singleton.with_borrow_mut(|map| {
                        assert!(map.is_empty());
                    });
                });
            }

            #[test]
            #[should_panic(expected = "already accessed")]
            fn cannot_borrow_then_borrow_mut() {
                let singleton = SingletonLenMap::new();
                singleton.with_borrow(|map| {
                    assert!(map.is_empty());
                    singleton.with_borrow_mut(|map| {
                        assert!(map.is_empty());
                    });
                });
            }


            #[test]
            #[should_panic(expected = "already accessed")]
            fn cannot_borrow_mut_then_borrow() {
                let singleton = SingletonLenMap::new();
                singleton.with_borrow_mut(|map| {
                    assert!(map.is_empty());
                    singleton.with_borrow(|map| {
                        assert!(map.is_empty());
                    });
                });
            }
        }
    }
}
