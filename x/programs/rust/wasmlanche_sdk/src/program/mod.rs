use crate::errors::StorageError;
use crate::host::init_program_storage;
use crate::store::{to_string, ProgramContext, Store, Tag};
use crate::types::Address;
use std::borrow::Cow;
use std::collections::HashMap;
use thiserror::Error;

#[derive(Clone, Error, Debug)]
pub enum ProgramError {
    #[error("{0}")]
    Store(#[from] StorageError),
}

/// ProgramValue represents a value that can be stored in the host.
#[repr(u8)]
pub enum ProgramValue {
    StringObject(String),
    MapObject,
    IntObject(i64),
    AddressObject(Address),
    ProgramObject(ProgramContext),
}

/// Program represents a program and its associated fields.
pub struct Program {
    fields: HashMap<String, ProgramValue>,
}

impl Default for Program {
    fn default() -> Self {
        Self::new()
    }
}

impl Program {
    pub fn new() -> Self {
        Program {
            fields: HashMap::new(),
        }
    }
    pub fn add_field(&mut self, name: String, val: ProgramValue) {
        self.fields.insert(name, val);
    }
    /// Initializes all the fields in the program and stores them in the host.
    pub fn publish(self) -> Result<ProgramContext, ProgramError> {
        // get the program_id from the host
        let ctx: ProgramContext = init_program_storage();
        // iterate through fields an set them in the host
        for (key, value) in &self.fields {
            ctx.store_value(key, value)?;
        }
        Ok(ctx)
    }
}

/// All program values implement Store. This allows us to store them in the host.
impl Store for ProgramValue {
    /// We use Cow because in the case of i64, we need to own & allocate a new Vec<u8> to store related bytes.
    /// In all other cases we can simply borrow.
    fn as_bytes(&self) -> Cow<'_, [u8]> {
        match self {
            ProgramValue::StringObject(val) => Cow::Borrowed(val.as_bytes()),
            ProgramValue::MapObject => Cow::Borrowed(&[]),
            // hypersdk's codec.Packer uses big endian
            ProgramValue::IntObject(val) => Cow::Owned(val.to_be_bytes().to_vec()),
            ProgramValue::AddressObject(val) => {
                let bytes: &[u8] = val.as_bytes();
                Cow::Borrowed(bytes)
            }
            ProgramValue::ProgramObject(val) => {
                // Since ProgramContext is a wrapper around a u64
                Cow::Owned(val.program_id.to_be_bytes().to_vec())
            }
        }
    }

    /// Converts to a ProgramValue from a byte slice. The byte slice contains the tag
    /// representing the variant and the rest of the bytes make up the value.
    fn from_bytes(bytes: &[u8]) -> Result<Self, StorageError>
    where
        Self: Sized,
    {
        if bytes.is_empty() {
            return Err(StorageError::InvalidByteLength(0));
        }
        // First byte must represent the "tag" of the ProgramValue.
        let tag = Tag::from(bytes[0]);
        let bytes = &bytes[1..];
        match tag.0 {
            1 => match to_string(bytes.to_vec()) {
                Ok(val) => Ok(ProgramValue::StringObject(val)),
                Err(_) => Err(StorageError::InvalidBytes()),
            },
            2 => Ok(ProgramValue::MapObject),
            3 => {
                let num = int_from_bytes(bytes)?;
                Ok(ProgramValue::IntObject(num))
            }
            4 => {
                let address_bytes: [u8; 32] = match bytes.try_into() {
                    Ok(val) => val,
                    Err(_) => {
                        return Err(StorageError::InvalidBytes());
                    }
                };

                Ok(ProgramValue::AddressObject(Address::new(address_bytes)))
            }
            5 => {
                let num = int_from_bytes(bytes)?;
                Ok(ProgramValue::ProgramObject(ProgramContext::from(num)))
            }
            invalid_tag => Err(StorageError::InvalidTag(invalid_tag)),
        }
    }

    /// The tag is used to identify the type of the value, and is prepended when storing in a map.
    fn as_tag(&self) -> Tag {
        match self {
            ProgramValue::StringObject(_) => Tag(1),
            ProgramValue::MapObject => Tag(2),
            ProgramValue::IntObject(_) => Tag(3),
            ProgramValue::AddressObject(_) => Tag(4),
            ProgramValue::ProgramObject(_) => Tag(5),
        }
    }
}

fn int_from_bytes(bytes: &[u8]) -> Result<i64, StorageError> {
    match bytes.len() {
        8 => (),
        len => return Err(StorageError::InvalidByteLength(len)),
    }

    let mut array = [0u8; 8];
    array.copy_from_slice(bytes);
    Ok(i64::from_be_bytes(array))
}
