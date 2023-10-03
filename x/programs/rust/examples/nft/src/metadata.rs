//! NFT schema
//! See https://nftschool.dev/reference/metadata-schemas/#ethereum-and-evm-compatible-chains for more information
//! on the ERC-721 NFT metadata schema.
use serde::{Deserialize, Serialize};

#[derive(Serialize, Deserialize)]
pub struct NFT {
    pub title: String,
    pub r#type: String,
    pub properties: Properties,
}

impl Default for NFT {
    fn default() -> Self {
        Self {
            title: "".to_string(),
            r#type: "".to_string(),
            properties: Properties {
                name: TypeDescription {
                    r#type: "".to_string(),
                    description: "".to_string(),
                },
                description: TypeDescription {
                    r#type: "".to_string(),
                    description: "".to_string(),
                },
                image: TypeDescription {
                    r#type: "".to_string(),
                    description: "".to_string(),
                },
            },
        }
    }
}

impl NFT {
    pub fn with_title(mut self, title: String) -> Self {
        self.title = title;
        self
    }

    pub fn with_type(mut self, r#type: String) -> Self {
        self.r#type = r#type;
        self
    }

    pub fn with_properties(mut self, properties: Properties) -> Self {
        self.properties = properties;
        self
    }
}

#[derive(Serialize, Deserialize)]
pub struct Properties {
    pub name: TypeDescription,
    pub description: TypeDescription,
    pub image: TypeDescription,
}

impl Default for Properties {
    fn default() -> Self {
        Self {
            name: TypeDescription {
                r#type: "".to_string(),
                description: "".to_string(),
            },
            description: TypeDescription {
                r#type: "".to_string(),
                description: "".to_string(),
            },
            image: TypeDescription {
                r#type: "".to_string(),
                description: "".to_string(),
            },
        }
    }
}
impl Properties {
    pub fn with_name(mut self, name: TypeDescription) -> Self {
        self.name = name;
        self
    }

    pub fn with_description(mut self, description: TypeDescription) -> Self {
        self.description = description;
        self
    }

    pub fn with_image(mut self, image: TypeDescription) -> Self {
        self.image = image;
        self
    }
}

#[derive(Serialize, Deserialize)]
pub struct TypeDescription {
    pub r#type: String,
    pub description: String,
}

impl Default for TypeDescription {
    fn default() -> Self {
        Self {
            r#type: "".to_string(),
            description: "".to_string(),
        }
    }
}

impl TypeDescription {
    pub fn with_type(mut self, r#type: String) -> Self {
        self.r#type = r#type;
        self
    }

    pub fn with_description(mut self, description: String) -> Self {
        self.description = description;
        self
    }
}

#[test]
fn test_schema() {
    let metadata = NFT {
        title: "My NFT Metadata".to_string(),
        r#type: "object".to_string(),
        properties: Properties {
            name: TypeDescription {
                r#type: "MNFT".to_string(),
                description: "Identifies the asset to which this NFT represents".to_string(),
            },
            description: TypeDescription {
                r#type: "My NFT".to_string(),
                description: "Describes the asset to which this NFT represents".to_string(),
            },
            image: TypeDescription {
                r#type: "ipfs://bafybeidlkqhddsjrdue7y3dy27pu5d7ydyemcls4z24szlyik3we7vqvam/nft-image.png".to_string(),
                description: "A URI pointing to a resource with mime type image/* representing the asset to which this NFT represents.".to_string(),
            },
        },
    };

    assert!(metadata.properties.name.r#type == "MNFT");
    assert!(metadata.properties.description.r#type == "My NFT");

    let nft_metadata = serde_json::to_string(&metadata).unwrap();
    let raw_nft_data = r#"{"title":"My NFT Metadata","type":"object","properties":{"name":{"type":"MNFT","description":"Identifies the asset to which this NFT represents"},"description":{"type":"My NFT","description":"Describes the asset to which this NFT represents"},"image":{"type":"ipfs://bafybeidlkqhddsjrdue7y3dy27pu5d7ydyemcls4z24szlyik3we7vqvam/nft-image.png","description":"A URI pointing to a resource with mime type image/* representing the asset to which this NFT represents."}}}"#;

    assert_eq!(nft_metadata, raw_nft_data);
}
