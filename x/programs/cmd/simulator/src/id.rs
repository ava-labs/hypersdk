use serde::Serialize;

#[derive(Clone, Copy, Debug, PartialEq, Serialize)]
pub struct Id(usize);

impl From<usize> for Id {
    fn from(id: usize) -> Self {
        Id(id)
    }
}

impl<'a> From<&'a Id> for &'a usize {
    fn from(val: &'a Id) -> Self {
        &val.0
    }
}
