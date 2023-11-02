# nft

The `nft` HyperSDK Program is an example of an NFT (non-fungible token).

For more information on NFTs, see the [resources at nft
school](https://nftschool.dev/concepts/non-fungible-tokens/#a-bit-of-history).

## Usage

The program exposes `mint` and `burn` methods which are publicly accessible.
Building a `Plan` with `Steps` to invoke these methods is the standard way to
interact with `nft`. See [the integration test](./tests/nft.rs) for a simple
invocation of `nft` via the WASM runtime.

## Testing

Use the [simulator](../../wasmlanche_sdk/src/simulator.rs) provided to run a
custom program. Detailed documentation on the simulator will soon be available.
