# single_nft

The `single_nft` HyperSDK Program is an example of an NFT (non-fungible token).

For more information on NFTs, see the [resources at nft school](https://nftschool.dev/concepts/non-fungible-tokens/#a-bit-of-history). 

This program specifically is an example of a single 1/1 NFT. It could represent a unique object like a building or a singular work of art. 

## Usage

The program exposes `mint_to` and `burn` methods which are publicly accessible. Building a `Plan` with `Steps` to invoke these methods is the standard way to interact with `single_nft`. See [the example](./src/example.rs) for a simple invocation of `single_nft` via the WASM runtime. 

## Testing

Use the [simulator](../../wasmlanche_sdk/src/simulator.rs) provided to run a custom program. Detailed documentation on the simulator will soon be available. 
