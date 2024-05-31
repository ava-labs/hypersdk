use serde::{Deserialize, Serialize};

#[derive(Clone, Copy, Debug, PartialEq)]
pub struct Id(usize);

impl From<usize> for Id {
    fn from(id: usize) -> Self {
        Id(id)
    }
}

impl Serialize for Id {
    fn serialize<S>(&self, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        let s = format!("step_{}", self.0);
        serializer.serialize_str(&s)
    }
}

impl<'de> Deserialize<'de> for Id {
    fn deserialize<D>(deserializer: D) -> Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        let s = String::deserialize(deserializer)?;

        if !s.starts_with("step_") {
            return Err(serde::de::Error::custom(r#"missing "step_" prefix"#));
        }

        let s = s.trim_start_matches("step_");
        let id = s.parse().map_err(serde::de::Error::custom)?;
        Ok(Id(id))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn id_serde() {
        let id = Id(42);
        let s = serde_json::to_string(&id).unwrap();
        assert_eq!(s, "\"step_42\"");

        let id: Id = serde_json::from_str(&s).unwrap();
        assert_eq!(id, Id(42));
    }
}
