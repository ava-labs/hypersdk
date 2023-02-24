<p align="center">
  <img width="90%" alt="tokenvm" src="assets/logo.png">
</p>
<p align="center">
  Mint and Trade User-Generated Tokens, All On-Chain
</p>
<p align="center">
  <a href="https://github.com/ava-labs/hypersdk/actions/workflows/tokenvm-unit-tests.yml"><img src="https://github.com/ava-labs/hypersdk/actions/workflows/tokenvm-unit-tests.yml/badge.svg" /></a>
  <a href="https://github.com/ava-labs/hypersdk/actions/workflows/tokenvm-sync-tests.yml"><img src="https://github.com/ava-labs/hypersdk/actions/workflows/tokenvm-sync-tests.yml/badge.svg" /></a>
  <a href="https://github.com/ava-labs/hypersdk/actions/workflows/tokenvm-static-analysis.yml"><img src="https://github.com/ava-labs/hypersdk/actions/workflows/tokenvm-static-analysis.yml/badge.svg" /></a>
</p>

The `tokenvm` enables anyone to mint their own token and then trade that token
with others, all on-chain. When minting, you can provide up to 256 bytes of
arbitrary data for any token and then change it in the future (without changing
the assetID)...perfect for reveals.

To trade, you can create an offer with a fixed ratio of tokens you'd accept for
yours and a supply. Others can fill parts of this order or you can close it at
some point in the future.

Because of the format of `hypersdk` transactions, you can scope your fills to
a particular second. This enables you to go for transactions as you see fit at
the time and not have to worry about your "fill" sitting around until you
replace it (with potentially a much higher tx). This also protects you from fee
volatility.

## Status
`tokenvm` is considered **ALPHA** software and is not safe to use in
production. The framework is under active development and may change
significantly over the coming months as its modules are optimized and
audited.

## Features
### Arbitrary Token Minting
#### Updateable Metadata
#### Rotate or Revoke Minter
### Trade Any 2 Tokens
#### Sandwich-Resistant
You can frontrun any transaction but the worst you can do is to fill an order
before I can get to it. You can't sell back to an order and you can't modify
the price of an order.
#### Partials Fills
#### In-Memory Order Book
##### Config Scoped to Certain Assets

## Demo
### Step 1: Mint Your Asset

### Step 2: Mint Another Asset

### Step 3: Create an Offer

### Step 4: Fill Part of the Offer

## Future Work
_If you want to take the lead on any of these items, please
[start a discussion](https://github.com/ava-labs/hypersdk/discussions) or reach
out on the Avalanche Discord._

* Add support for Avalanche Warp Messaging
* Add more config options for determining which order books to store in-memory
* Make it possible to fill multiple explicitly specified orders at once
