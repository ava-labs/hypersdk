use sdk_macros::public;

struct Foo;

impl Foo {
    #[public]
    pub fn test(&self) {}
}

fn main() {}
