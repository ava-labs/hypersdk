//! NFT schema
use serde::{Deserialize, Serialize};

#[derive(Serialize, Deserialize)]
pub struct Metadata {
    pub title: String,
    pub r#type: String,
    pub properties: AssetMetadata,
}

#[derive(Serialize, Deserialize)]
pub struct AssetMetadata {
    pub name: TypeDescription,
    pub description: TypeDescription,
    pub image: TypeDescription,
}

#[derive(Serialize, Deserialize)]
pub struct TypeDescription {
    pub r#type: String,
    pub description: String,
}

#[test]
fn test_schema() {
    let metadata = Metadata {
        title: "My NFT Metadata".to_string(),
        r#type: "object".to_string(),
        properties: AssetMetadata {
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
