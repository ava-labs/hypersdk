<p align="center">
  <img width="90%" alt="tokenvm" src="assets/logo.png">
</p>
<p align="center">
  Mint, Transfer, and Trade User-Generated Tokens, All On-Chain
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
### Arbitrary Token Minting
The basis of the `tokenvm` is the ability to create, mint, and transfer user-generated
tokens with ease. When creating an asset, the owner is given "admin control" of
the asset functions and can later mint more of an asset, update its metadata
(during a reveal for example), or transfer/revoke ownership (if rotating their
key or turning over to their community).

Assets are a native feature of the `tokenvm` and the storage engine is
optimized specifically to support their efficient usage (each balance entry
requires only 72 bytes of state = `assetID|publicKey=>balance(uint64)`). This
storage format makes it possible to parallelize the execution of any transfers
that don't touch the same accounts. This parallelism will take effect as soon
as it is re-added upstream by the `hypersdk` (no action required in the
`tokenvm`).

### Trade Any 2 Tokens
What good are custom assets if you can't do anything with them? To showcase the
raw power of the `hypersdk`, the `tokenvm` also provides support for fully
on-chain trading. Anyone can create an "offer" with a rate/token they are
willing to accept and anyone else can fill that "offer" if they find it
interesting. The `tokenvm` also maintains an in-memory order book to serve over
RPC for clients looking to interact with these orders.

Orders are a native feature of the `tokenvm` and the storage engine is
optimized specifically to support their efficient usage (just like balances
above). Each order requires only 152 bytes of
state = `orderID=>inAsset|inTick|outAsset|outTick|remaining|owner`. This
storage format also makes it possible to parallelize the execution of any fills
that don't touch the same order (there may be hundreds or thousands of orders
for the same pair, so this stil allows parallelization within a single pair
unlike a pool-based trading mechanism like an AMM). This parallelism will take
effect as soon as it is re-added upstream by the `hypersdk` (no action required
in the `tokenvm`).

#### In-Memory Order Book
To make it easier for clients to interact with the `tokenvm`, it comes bundled
with an in-memory order book that will listen for orders submitted on-chain for
any specified list of pairs (or all if you prefer). Behind the scenes, this
uses the `hypersdk's` support for feeding accepted transactions to any
`hypervm` (where the `tokenvm`, in this case, uses the data to keep its
in-memory record of order state up to date). The implementation of this is
a simple max heap per pair where we arrange best on the best "rate" for a given
asset (in/out).

#### Sandwich-Resistant
Because any fill must explicitly specify an order (it is up the client/CLI to
implement a trading agent to perform a trade that may span multiple orders) to
interact with, it is not possible for a bot to jump ahead of a transaction to
negatively impact the price of your execution (all trades with an order occur
at the same price). The worst they can do is to reduce the amount of tokens you
may be able to trade with the order (as they may consume some of the remaining
supply).

Not allowing the chain or block producer to have any control over what orders
a transaction may fill is a core design decision of the `tokenvm` and is a big
part of what makes its trading support so interesting/useful in a world where
producers are willing to manipulate transactions for their gain.

#### Partial Fills and Fill Refunds
Anyone filling an order does not need to fill an entire order. Likewise, if you
attempt to "overfill" an order the `tokenvm` will refund you any extra input
that you did not use. This is CRITICAL to get right in a blockchain-context
because someone may interact with an order just before you attempt to acquire
any remaining tokens...it would not be acceptable for all the assets you
pledged for the fill that weren't used to disappear.

#### Expiring Fills
Because of the format of `hypersdk` transactions, you can scope your fills to
be valid only until a particular time. This enables you to go for orders as you
see fit at the time and not have to worry about your fill sitting around until you
explicitly cancel it/replace it.

## Mint and Trade Demo
Someone: "Seems cool but I need to see it to really get it."
Me: "Look no further."

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
To make it easy to interact with the `tokenvm`, we implemented the `token-cli`.
You can build it using the following command from this location:
```bash
./scripts/build.sh
```

This command will put the compiled CLI in `./build/token-cli`.

### Step 2: Create Your Asset
First up, let's create our own asset. You can do so by running the following
command from this location:
```bash
./build/token-cli create-asset
```

When you are done, the output should look something like this:
```
loaded address: token1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97nsjzf3yp

metadata (can be changed later): MarioCoin
continue (y/n): y
transaction succeeded
assetID: 2617QeL3K4DTa1yXP8eicUu2YCDP38XJUUPv1KbQZxyDvBhHBF
```

The "loaded address" here is the address of the default private key (`demo.pk`). We
use this key to authenticate all interactions with the `tokenvm`.

### Step 3: Mint Your Asset
After we've created our own asset, we can now mint some of it. You can do so by
running the following command from this location:
```bash
./build/token-cli mint-asset
```

When you are done, the output should look something like this (usually easiest
just to mint to yourself).
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

### Step 4: View Your Balance
Now, let's check that the mint worked right by checking our balance. You can do
so by running the following command from this location:
```bash
./build/token-cli balance
```

When you are done, the output should look something like this:
```
loaded address: token1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97nsjzf3yp

address: token1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97nsjzf3yp
assetID (use TKN for native token): 2617QeL3K4DTa1yXP8eicUu2YCDP38XJUUPv1KbQZxyDvBhHBF
balance: 10000 2617QeL3K4DTa1yXP8eicUu2YCDP38XJUUPv1KbQZxyDvBhHBF
```

### Step 5: Create an Order
So, we have some of our token (`MarioCoin`)...now what? Let's put an order
on-chain that will allow someone to trade the native token (`TKN`) for some.
You can do so by running the following command from this location:
```bash
./build/token-cli create-order
```

When you are done, the output should look something like this:
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

The "in tick" is how much of the "in assetID" that someone must trade to get
"out tick" of the "out assetID". Any fill of this order must send a multiple of
"in tick" to be considered valid (this avoid ANY sort of precision issues with
computing decimal rates on-chain).

### Step 6: Fill Part of the Order
Now that we have an order on-chain, let's fill it! You can do so by running the
following command from this location:
```bash
./build/token-cli fill-order
```

When you are done, the output should look something like this:
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

Note how all available orders for this pair are listed by the CLI (these come
from the in-memory order book maintained by the `tokenvm`).

### Step 5: Close Order
Let's say we now changed our mind and no longer want to allow others to fill
our order. You can cancel it by running the following command from this
location:
```bash
./build/token-cli close-order
```

When you are done, the output should look something like this:
```
loaded address: token1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97nsjzf3yp

orderID: DZK5sQGk8jTyAfcPDxfHwdx5z9WFEFeqKQPgpNevLkeRV52xq
out assetID (use TKN for native token): 2617QeL3K4DTa1yXP8eicUu2YCDP38XJUUPv1KbQZxyDvBhHBF
continue (y/n): y
transaction succeeded
txID: 2iTnmhJUiUvC3wrwx8KLkV4aCJJCWAwZVRE8YVp8i6LdpDTyqg
```

Any funds that were locked up in the order will be returned to the creator's
account.

### Bonus: Watch Activity in Real-Time
To provide a better sense of what is actually happening on-chain, the
`index-cli` comes bundled with a simple explorer that logs all blocks/txs that
occur on-chain. You can run this utility by running the following command from
this location:
```bash
./build/token-cli watch
```

If you run it correctly, you'll see the following input (will run until the
network shuts down or you exit):
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
