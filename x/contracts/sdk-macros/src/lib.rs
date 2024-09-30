// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

extern crate proc_macro;

use proc_macro::TokenStream;
use syn::{parse_macro_input, punctuated::Punctuated, Expr, Token};

mod public;
mod state_schema;
mod to_pairs;

use public::{impl_public, PublicFn};
use state_schema::{impl_state_schema, KeyPair};
use to_pairs::to_pairs;

/// The `public` attribute macro will make the function you attach it to an entry-point for your smart-contract.
/// `#[public]` functions must have `pub` visibility and the first parameter must be of type `Context`.
/// They can have any number of additional parameters that implement `BorshSerialize` + `BorshDeserialize`.
/// The return type must also implement `BorshSerialize` + `BorshDeserialize`.
#[proc_macro_attribute]
pub fn public(_attr: TokenStream, item: TokenStream) -> TokenStream {
    let input = parse_macro_input!(item as PublicFn);

    match impl_public(input) {
        Ok(token_stream) => token_stream,
        Err(err) => err.to_compile_error(),
    }
    .into()
}

/// A procedural macro that generates a state schema for a smart contract.
/// ```
/// # use wasmlanche::{state_schema, Address};
/// #
/// state_schema! {
///     /// Unit-struct style key with a cardinality of 1
///     Key1 => u32,
///     /// Newtype wrapper style key with a cardinality of N
///     /// where N is the carginality of the inner type
///     Key2(Address) => u32,
///     /// Tuple-struct style key with a cardinality of M * N
///     /// where M and N are the cardinalities of the inner types (can be the same)
///     Allowance(Address, Address) => u32,
/// }
/// ```
///
/// The above example will create the following types: `Key1`, `Key2`, and `Allowance`.
///
/// `Key1` is a [ZST (zero sized type)](https://doc.rust-lang.org/nomicon/exotic-sizes.html#zero-sized-types-zsts).
///
/// `Key2` is a [newtype wrapper](https://doc.rust-lang.org/book/ch19-04-advanced-types.html#using-the-newtype-pattern-for-type-safety-and-abstraction) around `Address`.
///
/// `Allowance` is a tuple struct with two fields, both of type `Address`
/// and can be used similar manner to a composite key in a relational database.
///
/// All Keys must be stack values and pass the rules for implementing the `bytemuck::NoUninit` trait.
///
/// The types to the right of the `=>` are the value types associated with the given key.
/// The macro does not create types here, they have to be valid types already and have to implement the `BorshSerialize` and `BorshDeserialize` traits.
///
/// Each pair is used to automatically generate a `Schema` implementation for the key type with the value-type ass the associated `Schema::Value`.
/// For now, the keys are prefixed with `u8` to prevent collisions. This will likely change in the future, but that means you
/// absolutely should not use [state_schema!] more than one for a particular smart contract.
#[proc_macro]
pub fn state_schema(input: TokenStream) -> TokenStream {
    let key_pairs =
        parse_macro_input!(input with Punctuated::<KeyPair, Token![,]>::parse_terminated);

    match impl_state_schema(key_pairs) {
        Ok(token_stream) => token_stream,
        Err(err) => err.to_compile_error(),
    }
    .into()
}

#[doc(hidden)]
#[proc_macro]
pub fn impl_to_pairs(inputs: TokenStream) -> TokenStream {
    let inputs = parse_macro_input!(inputs with Punctuated::<Expr, Token![,]>::parse_terminated);

    match to_pairs(inputs) {
        Ok(token_stream) => token_stream,
        Err(err) => err.to_compile_error(),
    }
    .into()
}
