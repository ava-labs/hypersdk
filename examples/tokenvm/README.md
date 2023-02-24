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

We created the [`tokenvm`](./examples/tokenvm) to showcase how to use the
`hypersdk` in an application most readers are already familiar with, token minting
and token trading. The `tokenvm` lets anyone create any asset, mint more of
their asset, modify the metadata of their asset (if they reveal some info), and
burn their asset. Additionally, there is an embedded on-chain exchange that
allows anyone to create orders and fill (partial) orders of anyone else. To
make this example easy to play with, the `tokenvm` also bundles a powerful CLI
tool and serves RPC requests for trades out of an in-memory order book it
maintains by syncing blocks.

If you are interested in the intersection of exchanges and blockchains, it is
definitely worth a read (the logic for filling orders is < 100 lines of code!).

## Status
`tokenvm` is considered **ALPHA** software and is not safe to use in
production. The framework is under active development and may change
significantly over the coming months as its modules are optimized and
audited.

## Features
### Expiring Fills
Because of the format of `hypersdk` transactions, you can scope your fills to
a particular second. This enables you to go for transactions as you see fit at
the time and not have to worry about your "fill" sitting around until you
replace it (with potentially a much higher tx). This also protects you from fee
volatility.

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

### Step 1: Build the CLI
To interact...
```bash
./scripts/build.sh
```

### Step 1: Create Your Asset
```bash
./build/token-cli create-asset
```

```
loaded address: token1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97nsjzf3yp

metadata (can be changed later): MarioCoin
continue (y/n): y
transaction succeeded
assetID: 2617QeL3K4DTa1yXP8eicUu2YCDP38XJUUPv1KbQZxyDvBhHBF
```

### Step 1: Mint Your Asset
```bash
./build/token-cli mint-asset
```

```
loaded address: token1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97nsjzf3yp

assetID: 2617QeL3K4DTa1yXP8eicUu2YCDP38XJUUPv1KbQZxyDvBhHBF
metadata: MarioCoin supply: 0
recipient: token1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97nsjzf3yp
amount: 10000
continue (y/n): y
transaction succeeded
txID: 2TX47uKj1ax4rS8oFzPLrwBDkRXwAUwPHL6cToDT8BmeAYTANo
```

### Step X: View Your Balance
```bash
./build/token-cli balance
```

```
loaded address: token1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97nsjzf3yp

address: token1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97nsjzf3yp
assetID (use TKN for native token): 2617QeL3K4DTa1yXP8eicUu2YCDP38XJUUPv1KbQZxyDvBhHBF
balance: 10000 2617QeL3K4DTa1yXP8eicUu2YCDP38XJUUPv1KbQZxyDvBhHBF
```

### Step 3: Create an Order
```bash
./build/token-cli create-order
```

```
loaded address: token1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97nsjzf3yp

in assetID (use TKN for native token): TKN
in tick: 1
out assetID (use TKN for native token): 2617QeL3K4DTa1yXP8eicUu2YCDP38XJUUPv1KbQZxyDvBhHBF
metadata: MarioCoin supply: 10000
balance: 10000 2617QeL3K4DTa1yXP8eicUu2YCDP38XJUUPv1KbQZxyDvBhHBF
out tick: 10
out tick: 10
supply (must be multiple of OutTick): 100
continue (y/n): y
transaction succeeded
orderID: DZK5sQGk8jTyAfcPDxfHwdx5z9WFEFeqKQPgpNevLkeRV52xq
```

### Step 4: Fill Part of the Order (view orders)
```bash
./build/token-cli fill-order
```

```
loaded address: token1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97nsjzf3yp

in assetID (use TKN for native token): TKN
balance: 999.999999 TKN
out assetID (use TKN for native token): 2617QeL3K4DTa1yXP8eicUu2YCDP38XJUUPv1KbQZxyDvBhHBF
metadata: MarioCoin supply: 10000
available orders: 1
0) Rate(in/out): 0.1000 InTick: 1.000000 TKN OutTick: 10 2617QeL3K4DTa1yXP8eicUu2YCDP38XJUUPv1KbQZxyDvBhHBF Remaining: 100 2617QeL3K4DTa1yXP8eicUu2YCDP38XJUUPv1KbQZxyDvBhHBF
select order: 0
value (must be multiple of InTick): 2
in: 2 TKN out: 20 2617QeL3K4DTa1yXP8eicUu2YCDP38XJUUPv1KbQZxyDvBhHBF
continue (y/n): y
transaction succeeded
txID: gMPc9DhFLthpb5DEtFBrXTrs8wK7FA31P3xd5w518Xbq76K6q
```

### Step 5: Close Order
```bash
./build/token-cli close-order
```

```
loaded address: token1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97nsjzf3yp

orderID: DZK5sQGk8jTyAfcPDxfHwdx5z9WFEFeqKQPgpNevLkeRV52xq
out assetID (use TKN for native token): 2617QeL3K4DTa1yXP8eicUu2YCDP38XJUUPv1KbQZxyDvBhHBF
continue (y/n): y
transaction succeeded
txID: 2iTnmhJUiUvC3wrwx8KLkV4aCJJCWAwZVRE8YVp8i6LdpDTyqg
```

### Can watch in real-time
```bash
./build/token-cli watch
```

```
watching for new blocks 👀
height:4 txs:1 units:1536 root:2wZfnnPMeUFgEtJdtLbKA1JFpRvZNNbDCCy2gWyEfpqWwL9HpL
✅ gMPc9DhFLthpb5DEtFBrXTrs8wK7FA31P3xd5w518Xbq76K6q actor: token1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97nsjzf3yp units: 1536 summary (*actions.FillOrder): [2.000000 TKN -> 20 2617QeL3K4DTa1yXP8eicUu2YCDP38XJUUPv1KbQZxyDvBhHBF (remaining: 80 2617QeL3K4DTa1yXP8eicUu2YCDP38XJUUPv1KbQZxyDvBhHBF)]
height:5 txs:1 units:464 root:NUNNi2DyXeGL7jPPnWTeNpmjgtv9qgM131xQsNu4fXT9TkQzj
⚠️ iHEK4mjtp86s8miJHsgCVofd7BE8jcGBtnSRZHiA9LRPiQVDw actor: token1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97nsjzf3yp units: 464 summary (*actions.CloseOrder): [wrong out asset]
height:6 txs:1 units:464 root:M5M5ZXNAPoBvkkjRCzpFD8qKkiuKpZKSYCSvdhby3gYA7GKww
✅ 2iTnmhJUiUvC3wrwx8KLkV4aCJJCWAwZVRE8YVp8i6LdpDTyqg actor: token1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97nsjzf3yp units: 464 summary (*actions.CloseOrder): [orderID: DZK5sQGk8jTyAfcPDxfHwdx5z9WFEFeqKQPgpNevLkeRV52xq]
```

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
