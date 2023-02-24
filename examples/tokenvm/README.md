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
The first step to running this demo is to launch your own `tokenvm` Subnet. You
can do so by running the following command from this location:
```bash
./scripts/run.sh;
```

By default, this allocates all funds on the network to
`token1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97nsjzf3yp`. The private
key for this address is
`0x323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7`.
For convenience, this key has is also stored at `demo.pk`.

### Step 0: Build the CLI
./scripts/build.sh

### Step 1: Mint Your Asset

### Step X: View Your Balance

### Step 2: Transfer Your Asset
token18ccm7a2uadj8mctjghkg3fkrneqpptxceykvwm8x7vxyrwmxqf8qmxyzft

### Step 2: Mint Another Asset

### Step 3: Create an Order

### Step 4: Fill Part of the Order (view orders)

### Step 5: Close Order

### Can watch in real-time
/build/token-cli watch

## Future Work
_If you want to take the lead on any of these items, please
[start a discussion](https://github.com/ava-labs/hypersdk/discussions) or reach
out on the Avalanche Discord._

* Add support for Avalanche Warp Messaging
* Add more config options for determining which order books to store in-memory
* Add option to CLI to fill up to some amount of an asset as long as it is
  under some exchange rate (trading agent command to provide better UX)
* Add expiring order support (can't fill an order after some point in time but
  still need to explicitly close it to get your funds back -> async cleanup is
  not a good idea)
