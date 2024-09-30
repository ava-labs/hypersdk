// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Here, we import the `public` attribute-macro, the `state_schema` macro,
// and the `Address` and `Context` types
use wasmlanche::{public, state_schema, Address, Context};

// Count is a type alias. It makes it easy to change the type of the counter
// without having to update multiple places. For instance, if you wanted to
// be more space-efficient, you could use a u32 instead of a u64.
pub type Count = u64;

// We use the `state_schema` macro to define the state-keys and value types.
// The macro will create types such as `struct Counter(Address);`. The type
// is what's called a newtype wrapper around an Address. Keys must be defined as
// unit-structs, `Key`, or tuple-structs `Key(u32, u64)`. Tuple-structs can wrap one
// or more values forming something akin to a composite key in a relational database.
// Keys must be a fixed size (stack value). A good rule of thumb is that if they
// implement the `Copy` trait, they are a good candidate for a key. Try using a
// Heap allocated Type like a `String` or a `Vec` and see what the compiler tells you.
state_schema! {
    /// Counter for each address.
    Counter(Address) => Count,
}

// NOTE: use `wasmlanche::dbg!` for debugging. It's works like the `std::dbg!` macro.

// The `///` syntax is used to document the function. This documentation
// will be visible when the documentation is generated with `cargo doc`
//
// Every function that is callable form the outside world must be marked with the
// `#[public]` attribute and must follow a specific signature. Try writing a function
// marked with the `#[public]` attribute without any arguemnts. See if the compiler can
// guide you to the correct signature.
//
/// Gets the count at the address.
#[public]
pub fn get_value(context: &mut Context, of: Address) -> Count {
    // `Context::get` returns a Result<Option<T>, Error>
    // where T is the value type associated with the key. In the example below
    // T is `Count` which is just an alias for a `u64` (unless you changed it).
    // Note:
    // `expect` will cause the code to panic if there's a deserialization error
    // otherwise it will take a `Result<T, E>` and return the `T`.
    context
        .get(Counter(of))
        .expect("state corrupt")
        .unwrap_or_default()
}

/// Increments the count at the address by the amount.
#[public]
pub fn inc(context: &mut Context, to: Address, amount: Count) {
    let counter = amount + get_value(context, to);

    context
        .store_by_key(Counter(to), counter)
        .expect("serialization failed");
}

#[public]
pub fn inc_me_by_one(context: &mut Context) {
    let caller = context.actor();
    inc(context, caller, 1);
}
