<p align="center">
  <img width="90%" alt="hypersdk" src="assets/logo.png">
</p>
<p align="center">
  Opinionated Framework for Building Hyper-Scalable Blockchains on Avalanche
</p>
<p align="center">
  <a href="https://goreportcard.com/report/github.com/ava-labs/hypersdk"><img src="https://goreportcard.com/badge/github.com/ava-labs/hypersdk" /></a>
  <a href="https://github.com/ava-labs/hypersdk/actions/workflows/hypersdk-unit-tests.yml"><img src="https://github.com/ava-labs/hypersdk/actions/workflows/hypersdk-unit-tests.yml/badge.svg" /></a>
  <a href="https://github.com/ava-labs/hypersdk/actions/workflows/hypersdk-static-analysis.yml"><img src="https://github.com/ava-labs/hypersdk/actions/workflows/hypersdk-static-analysis.yml/badge.svg" /></a>
<a href="./LICENSE" ><img src="https://img.shields.io/badge/License-Ecosystem-blue.svg" /></a>
</p>

---

The freedom to create your own [Virtual Machine (VM)](https://docs.avax.network/learn/avalanche/virtual-machines),
or blockchain runtime, is one of the most exciting and powerful aspects of building
on Avalanche, however, it is difficult and time-intensive to do from scratch. Forking
existing Avalanche VMs makes it easier to get started, like [spacesvm](https://github.com/ava-labs/spacesvm)
or [subnet-evm](https://github.com/ava-labs/subnet-evm), but is time-consuming
and complex to ensure correctness as changes occur upstream (in repos which
often weren't meant to be used as a library).

The `hypersdk` is the first (of many) frameworks dedicated to making it
faster, safer, and easier to launch your own optimized blockchain on an Avalanche
Subnet. By hiding much of the complexity of building your own blockchain
runtime behind Avalanche-optimized data structures and algorithms, the
`hypersdk` enables builders to focus their attention on the aspects of their
runtime that make their project unique (and override the defaults only if needed).
For example, a DEX-based project should focus on implementing a novel trading
system and not on transaction serialization, assuming that is already done
efficiently for them.

This opinionated design methodology means that most runtimes built on the
`hypersdk`, called a `hypervm`, only need to implement 500-1000 lines of their own
code to add custom interaction patterns (and don't need to copy-paste code from upstream
that they need to keep up-to-date). However, if you do want to provide your own
mechanism, you can always override anything you are using upstream if you can
compose something better suited for your application. That same DEX-based
project may wish to implement custom block building logic that prioritizes the
inclusions of trades of certain partners or that interact with certain order
books.

Last but certainly not least, the usage of these Avalanche-optimized data structures
and algorithms means that your `hyperchain` can process thousands of transactions per second
without needing to hire a team of engineers to optimize it or understanding
anything about how it works under the hood...but you can certainly achieve
higher throughput if you do ;).

### Terminology
* `hypersdk`: framework for building high-performance blockchains on Avalanche
* `hypervm`: Avalanche Virtual Machine built using the `hypersdk`
* `hyperchain`: `hypervm` deployed on the Avalanche Network

## Status
`hypersdk` is considered **ALPHA** software and is not safe to use in
production. The framework is under active development and may change
significantly over the coming months as its modules are optimized and
audited.

## Features
### Efficient State Management
All `hypersdk` state is stored using [`x/merkledb`](https://github.com/ava-labs/avalanchego/blob/master/x/merkledb/README.md),
a path-based merkelized radix tree implementation provided by `avalanchego`. This high-performance
data structure minimizes the on-disk footprint of any `hypervm` out-of-the-box by deleting
any data that is no longer part of the current state (without performing any costly reference counting).

The use of this type of data structure in the blockchain context was pioneered
by the [go-ethereum](https://github.com/ethereum/go-ethereum) team in an effort
to minimize the on-disk footprint of the EVM. We wanted to give a Huge shoutout
to that team for all the work they put into researching this approach.

#### Dynamic State Sync
Instead of requiring nodes to execute all previous transactions when joining
any `hyperchain` (which may not be possible if there is very high throughput on a Subnet),
the `hypersdk` just syncs the most recent state from the network. To avoid falling
behind the network while syncing this state, the `hypersdk` acts as an Avalanche Lite
Client and performs consensus on newly processed blocks without verifying them (updating its
state sync target whenever a new block is accepted).

The `hypersdk` relies on [`x/sync`](https://github.com/ava-labs/avalanchego/tree/master/x/sync),
a bandwidth-aware dynamic sync implementation provided by `avalanchego`, to
sync to the tip of any `hyperchain`.

#### Block Pruning
The `hypersdk` defaults to only storing what is necessary to build/verify the next block
and to help new nodes sync the current state (not execute historical state transitions).
If the `hypersdk` did not limit block storage growth, the disk requirements for validators
would grow at an alarming rate each day (making running any `hypervm` impractical).
Consider the simple example where we process 25k transactions per second (assume each
transaction is ~400 bytes); this would would require the `hypersdk` to store 10MB per
second (not including any overhead in the database for doing so). **This works out to
864GB per day or 315.4TB per year.**

When `MinimumBlockGap=250ms` (minimum time betweem blocks), the `hypersdk` must store at
least ~240 blocks to allow for the entire `ValidityWindow` to be backfilled (otherwise
a fully-synced, restarting `hypervm` will not become "ready" until it accepts a block at
least `ValidityWindow` after the last accepted block). To provide some room for error during
disaster recovery (network outage), however, it is recommened to configure the `hypersdk` to
store the last >= ~50,000 accepted blocks (~3.5 hours of activity with a 250ms `MinimumBlockGap`).
This allows archival nodes that become disconnected from the network (due to a data center outage or bug)
to ensure they can persist all historical blocks (which would otherwise be deleted by all participants and
unindexable).

_The number of blocks that the `hypersdk` stores on-disk, the `AcceptedBlockWindow`, can be tuned by any `hypervm`
to an arbitrary depth (or set to `MaxInt` to keep all blocks). To limit disk IO used to serve blocks over
the P2P network, `hypervms` can configure `AcceptedBlockWindowCache` to store recent blocks in memory._

### WASM-Based Programs
In the `hypersdk`, [smart contracts](https://ethereum.org/en/developers/docs/smart-contracts/)
(e.g. programs that run on blockchains) are referred to simply as `programs`. `Programs`
are [WASM-based](https://webassembly.org/) binaries that can be invoked during block
execution to perform arbitrary state transitions. This is a more flexible, yet less performant,
alternative to defining all `Auth` and/or `Actions` that can be invoked in the `hypervm` in the
`hypervm's` code (like the `tokenvm`).

Because the `hypersdk` can execute arbitrary WASM, any language (Rust, C, C++, Zig, etc.) that can
be compiled to WASM can be used to write `programs`. You can view a collection of
Rust-based `programs` [here](https://github.com/ava-labs/hypersdk/tree/main/x/programs/rust/examples).

### Account Abstraction
The `hypersdk` provides out-of-the-box support for arbitrary transaction authorization logic.
Each `hypersdk` transaction includes an `Auth` object that implements an
`Actor` function (identity that participates in an `Action`) and a `Sponsor` function (identity
that pays fees). These two identities could be the same (if using a simple signature
verification `Auth` module) but may be different (if using a "gas relayer" `Auth` module).

`Auth` modules may be hardcoded, like in
[`morpheusvm`](https://github.com/ava-labs/hypersdk/tree/main/examples/morpheusvm/auth) and
[`tokenvm`](https://github.com/ava-labs/hypersdk/tree/main/examples/tokenvm/auth), or execute
a `program` (i.e. a custom deployed multi-sig). To allow for easy interaction between different
`Auth` modules (and to ensure `Auth` modules can't interfere with each other), the
`hypersdk` employs a standard, 33-byte addressing scheme: `<typeID><ids.ID>`. Transaction
verification ensures that any `Actor` and `Sponsor` returned by an `Auth` module
must have the same `<typeID>` as the module generating an address. The 32-byte hash (`<ids.ID>`)
is used to uniquely identify accounts within an `Auth` scheme. For `programs`, this
will likely be the `txID` when the `program` was deployed and will be the hash
of the public key for pure cryptographic primitives (the indirect benefit of this
is that account public keys are obfuscated until used).

_Because transaction IDs are used to prevent replay, it is critical that any signatures used
in `Auth` are [not malleable](https://github.com/bitcoin/bips/blob/master/bip-0062.mediawiki).
If malleable signatures are used, it would be trivial for an attacker to generate additional, valid
transactions from an existing transaction and submit it to the network (duplicating whatever `Action` was
specified by the sender)._

_It is up to each `Auth` module to limit the computational complexity of `Auth.Verify()` to prevent a DoS
(invalid `Auth` will not charge `Auth.Sponsor()`)._

### Optimized Block Execution Out-of-the-Box
The `hypersdk` is primarily about an obsession with hyper-speed and
hyper-scalability (and making it easy for developers to achieve both by
wrapping their work in opinionated and performant abstractions).
Developers don't care how easy it is to launch or maintain their own
blockchain if it can't process thousands of transactions per second with low
time-to-finality. For this reason, most development time on the `hypersdk`
thus far has been dedicated to making block verification and state management
as fast and efficient as possible, which both play a large role in making this
happen.

#### Parallel Transaction Execution
`hypersdk` transactions must specify the keys they will access in state (read
and/or write) during authentication and execution so that non-conflicting transactions
can be processed in parallel. To do this efficiently, the `hypersdk` uses
the [`executor`](https://github.com/ava-labs/hypersdk/tree/main/executor) package, which
can generate an execution plan for a set of transactions on-the-fly (no preprocessing required).
`executor` is used to parallelize execution in both block building and in block verification.

When a `hypervm's` `Auth` and `Actions` are simple and pre-specified (like in the `morpheusvm`),
the primary benefit of parallel execution is to concurrently fetch the state needed for execution
(actual execution of precompiled golang only takes nanoseconds). However, parallel execution
massively speeds up the E2E execution of a block of `programs`, which may each take a few milliseconds
to process. Consider the simple scenario where a `program` takes 2 milliseconds; processing 1000 `programs`
in serial would take 2 seconds (far too long for a high-throughput blockchain). The same execution, however,
would only take 125 milliseconds if run over 16 cores (assuming no conflicts).

_The number of cores that the `hypersdk` allocates to execution can be tuned by
any `hypervm` using the `TransactionExecutionCores` configuration._

#### Deferred Root Generation
All `hypersdk` blocks include a state root to support dynamic state sync. In dynamic
state sync, the state target is updated to the root of the last accepted block while
the sync is ongoing instead of staying pinned to the last accepted root when the sync
started. Root block inclusion means consensus can be used to select the next state
target to sync to instead of using some less secure, out-of-consensus mechanism (i.e.
Avalanche Lite Client).

Dynamic state sync is required for high-throughput blockchains because it relieves
the nodes that serve state sync queries from storing all historical state revisions
(if a node doesn't update its sync target, any node serving requests would need to
store revisions for at least as long as it takes to complete a sync, which may
require significantly more storage).

```golang
type StatefulBlock struct {
	Prnt   ids.ID `json:"parent"`
	Tmstmp int64  `json:"timestamp"`
	Hght   uint64 `json:"height"`

	Txs []*Transaction `json:"txs"`

	StateRoot   ids.ID     `json:"stateRoot"`
}
```

Most blockchains that store a state root in the block use the root of a merkle tree
of state post-exectution, however, this requires waiting for state merklization to complete
before block verification can finish. If merklization was fast, this wouldn't be an
issue, however, this process is typically the most time consuming aspect of block
verification.

`hypersdk` blocks instead include the merkle root of the post-execution state of a block's
parent rather than a merkle root of their own post-execution state. This design enables the
`hypersdk` to generate the merkle root of a block's post-execution state anchronously
while the consensus engine is working on other tasks that typically are network-bound rather
than CPU-bound, like merklization, making better use of all available resources.

#### [Optional] Parallel Signature Verification
The `Auth` interface (detailed below) exposes a function called `AsyncVerify` that
the `hypersdk` may call concurrently (may invoke on other transactions in the same
block) at any time prior/during block execution. Most `hypervms` perform signature
verification in this function and save any state lookups for the full `Auth.Verify`
(which has access to state, unlike `AsyncVerify`). The generic support for performing certain
stateless activities during execution can greatly reduce the e2e verification
time of a block when running on powerful hardware.

#### [Optional] Batch Signature Verification
Some public-key signature systems, like [Ed25519](https://ed25519.cr.yp.to/), provide
support for verifying batches of signatures (which can be more much efficient than
verifying each signature individually). The `hypersdk` generically supports this
capability for any `Auth` module that implements the `AuthBatchVerifier` interface,
even parallelizing batch computation for systems that only use a single-thread to
verify a batch.

### Multidimensional Fee Pricing
Instead of mapping transaction resource usage to a one-dimensional unit (i.e. "gas"
or "fuel"), the `hypersdk` utilizes five independently parameterized unit dimensions
(bandwidth, compute, storage[read], storage[allocate], storage[write]) to meter
activity on each `hypervm`. Each unit dimension has a unique metering schedule
(i.e. how many units each resource interaction costs), target, and max utilization
per rolling 10 second window.

When network resources are independently metered, they can be granularly priced
and thus better utilized by network participants. Consider a simple example of a
one-dimensional fee mechanism where each byte is 2 units, each compute cycle is 5 units,
each storage operation is 10 units, target usage is 7,500 units per block, and the max
usage in any block is 10,000 units. If someone were to use 5,000 bytes of block data
without utilizing any CPU/storing data in state, they would exhaust the block capacity
without using 2 of the 3 available resources. This block would also increase the price
of each unit because usage is above the target. As a result, the price to use compute
and storage in the next block would be more expensive although neither has been used.
In the `hypersdk`, only the price of bandwidth would go up and the price of CPU/storage
would stay constant, a better reflection of supply/demand for each resource.

So, why go through all this trouble? Accurate and granular resource metering is required to
safely increase the throughput of a blockchain. Without such an approach, designers
need to either overprovision the network to allow for one resource to be utilized to maximum
capacity (max compute unit usage may also allow unsustainable state growth) or bound capacity
to a level that leaves most resources unused. If you are interested in reading more analysis
of multidimensional fee pricing, [Dynamic Pricing for Non-fungible Resources: Designing
Multidimensional Blockchain Fee Markets](https://arxiv.org/abs/2208.07919) is a great resource.

#### Invisible Support
Developers must have to implement a ton of complex code to take advantage of this
fee mechanism, right? Nope!

Multidimensional fees are abstracted away from `hypervm` developers and managed
entirely by the `hypersdk`. `hypervm` designers return the fee schedule, targets,
and max usage to use in `Rules` (allows values to change depending on timestamp) and
the `hypersdk` will handle the rest:
```golang
GetMinUnitPrice() Dimensions
GetUnitPriceChangeDenominator() Dimensions
GetWindowTargetUnits() Dimensions
GetMaxBlockUnits() Dimensions

GetBaseComputeUnits() uint64

GetStorageKeyReadUnits() uint64
GetStorageValueReadUnits() uint64 // per chunk
GetStorageKeyAllocateUnits() uint64
GetStorageValueAllocateUnits() uint64 // per chunk
GetStorageKeyWriteUnits() uint64
GetStorageValueWriteUnits() uint64 // per chunk
```

An example configuration may look something like:
```golang
MinUnitPrice:               chain.Dimensions{100, 100, 100, 100, 100},
UnitPriceChangeDenominator: chain.Dimensions{48, 48, 48, 48, 48},
WindowTargetUnits:          chain.Dimensions{20_000_000, 1_000, 1_000, 1_000, 1_000},
MaxBlockUnits:              chain.Dimensions{1_800_000, 2_000, 2_000, 2_000, 2_000},

BaseComputeUnits:          1,

StorageKeyReadUnits:       5,
StorageValueReadUnits:     2,
StorageKeyAllocateUnits:   20,
StorageValueAllocateUnits: 5,
StorageKeyWriteUnits:      10,
StorageValueWriteUnits:    3,
```

#### Avoiding Complex Construction
Historically, one of the largest barriers to supporting
multidimensional fees has been the complex UX it can impose
on users. Setting a one-dimensional unit price and max unit usage
already confuses most, how could you even consider adding more?

The `hypersdk` takes a unique approach and requires users to set a
single `Base.MaxFee` field, denominated in tokens rather than usage.
The `hypersdk` uses this fee to determine whether or not a transaction
can be executed and then only charges what it actually used. For
example, a user may specify to use up to 5 TKN but may only be charged
1 TKN, depending on their transaction's unit usage and the price of
each unit dimension during execution. This approach is only possible
because the `hypersdk` requires transactions to be "fully specified"
before execution (i.e. an executor can determine the maximum amount
of units that will be used by each resource without simulating the
transaction).

It is important to note that the resource precomputation can be quite
pessimistic (i.e. assumes the worse) and can lead to the maximum fee
for a transaction being ~2x as large as the fee it uses on-chain (depending
on the usage of cold/warm storage, as discussed later). In practice,
this means that accounts may need a larger balance than they otherwise
would to issue transactions (as the `MaxFee` must be payable during
execution). In the future, it will also be possible to optionally
specify a max usage of each unit dimension to better bound this pessimism.

#### No Priority Fees
Transactions are executed in FIFO order by each validator and there is no
way for a user to specify some "priority" fee to have their transaction
included in a block sooner. If a transaction cannot be executed when
it is pulled from the mempool (because its `MaxFee` is insufficient), it will
be dropped and must be reissued.

Aside from FIFO handling being dramatically more efficient for each validator,
price-sorted mempools are not particularly useful in high-throughput
blockchains where the expected mempool size is ~0 or there is a bounded transaction
lifetime (60 seconds by default on the `hypersdk`).

#### Separate Metering for Storage Reads, Allocates, Writes
To make the multidimensional fee implementation for the `hypersdk` simpler,
it would have been possible to unify all storage operations (read, allocate,
write) into a single unit dimension. We opted not to go this route, however,
because `hypervm` designers often wish to regulate state growth much differently
than state reads or state writes.

Fundamentally, it makes sense to combine resource usage into a single unit dimension
if different operations are scaled substitutes of each other (an executor could translate
between X units of one operation to Y units of another). It is not clear how to compare,
for example, the verification of a signature with the storage of a new key in state
but is clear how to compare the verification of a signature with the addition of two numbers
(just different CPU cycle counts).

Although more nuanced, the addition of new data to state is a categorically different operation
than reading data from state and cannot be compared on a single plane. In other words,
it is not clear how many reads a developer would or should trade for writes and/or that
they are substitutes for each other in some sort of disk resource (by mapping to a
single unit dimension, performing a bunch of reads would make writes more expensive).

#### Size-Encoded Storage Keys
To compute the maximum amount of storage units that a transaction could use,
it must be possible to determine how much data a particular key can read/write
from/to state. The `hypersdk` requires that all state keys are suffixed with
a big-endian encoded `uint16` of the number of "chunks" (each chunk is 64 bytes)
that can be read/stored to satisfy this requirement.  This appended size suffix is part of the key,
so the same key with different size suffixes would be considered distinct keys.

This constraint is equivalent to deciding whether to use a `uint8`, `uint16`, `uint32`,
`uint64`, etc. when storing an unsigned integer value in memory. The tighter a
`hypervm` developer bounds the max chunks to the chunks they will store, the cheaper
the estimate will be for a user to interact with state. Users are only charged, however,
based on the amount of chunks actually read/written from/to state.

### Nonce-less and Expiring Transactions
`hypersdk` transactions don't use [nonces](https://help.myetherwallet.com/en/articles/5461509-what-is-a-nonce)
to protect against replay attack like many other account-based blockchains. This means users
can submit transactions concurrently from a single account without worrying about ordering them properly
or getting stuck on a transaction that was dropped by the mempool.

Additionally, `hypersdk` transactions contain a time past which they can no longer be included inside
of a `hypersdk` block. This makes it straightforward to take advantage of temporary situations on a
`hyperchain` (if you only wanted your transaction to be valid for a few seconds) and removes
the need to broadcast replacement transactions (if the fee changes or you want
to cancel a transaction).

On the performance side of things, a lack of transaction nonces makes the
mempool more performant (as we no longer need to maintain multiple transactions
for a single account and ensure they are ordered) and makes the network layer
more efficient (we can gossip any valid transaction to any node instead of just
the transactions for each account that can be executed at the moment).

### Easy Functionality Upgrades
Every object that can appear on-chain (i.e. `Actions` and/or `Auth`) and every chain
parameter (i.e. `Unit Price`) is scoped by block timestamp. This makes it
possible to easily modify existing rules (like how much people pay for certain
types of transactions) or even disable certain types of `Actions` altogether.

Launching your own blockchain is the first step of a long journey of continuous
evolution. Making it straightforward and explicit to activate/deactivate any
feature or config is critical to making this evolution safely.

### Proposer-Aware Gossip
Unlike the Virtual Machines live on the Avalanche Primary Network (which gossip
transactions uniformly to all validators), the `hypersdk` only gossips
transactions to the next few preferred block proposers ([using Snowman++'s
lookahead logic](https://github.com/ava-labs/avalanchego/blob/master/vms/proposervm/README.md)).
This change greatly reduces the amount of unnecessary transaction gossip
(which we define as gossiping a transaction to a node that will not produce
a block during a transaction's validity period) for any `hyperchain` out-of-the-box.

If you prefer to employ a different gossiping mechanism (that may be more
aligned with the `Actions` you define in your `hypervm`), you can always
override the default gossip technique with your own. For example, you may wish
to not have any node-to-node gossip and just require validators to propose
blocks only with the transactions they've received over RPC.

### Transaction Results and Execution Rollback
The `hypersdk` allows for any `Action` to return a result from execution
(which can be any arbitrary bytes), the amount of fee units it consumed, and
whether or not it was successful (if unsuccessful, all state changes are rolled
back). This support is typically required by anyone using the `hypersdk` to
implement a smart contract-based runtime that allows for cost-effective
conditional execution (exiting early if a condition does not hold can be much
cheaper than the full execution of the transaction).

The outcome of execution is not stored/indexed by the `hypersdk`. Unlike most other
blockchains/blockchain frameworks, which provide an optional "archival mode" for historical access,
the `hypersdk` only stores what is necessary to validate the next valid block and to help new nodes
sync to the current state. Rather, the `hypersdk` invokes the `hypervm` with all execution
results whenever a block is accepted for it to perform arbitrary operations (as
required by a developer's use case). In this callback, a `hypervm` could store
results in a SQL database or write to a Kafka stream.

### Support for Generic Storage Backends
When initializing a `hypervm`, the developer explicitly specifies which storage backends
to use for each object type (state vs blocks vs metadata). As noted above, this
defaults to CockroachDB's `pebble` but can be swapped with experimental storage
backends and/or traditional cloud infrastructure. For example, a `hypervm`
developer may wish to manage state objects (for the Path-Based Merkelized Radix
Tree) on-disk but use S3 to store blocks and PostgreSQL to store transaction metadata.

### Continuous Block Production
Unlike other VMs on Avalanche, `hypervms` produce blocks continuously (even if empty).
While this may sound wasteful, it improves the "worst case" AWM verification cost (AWM verification
requires creating a reverse diff to the last referenced P-Chain block), prevents a fallback to leaderless
block production (which can lead to more rejected blocks), and avoids a prolonged post-bootstrap
readiness wait (`hypersdk` waits to mark itself as ready until it has seen a `ValidityWindow` of blocks).

Looking ahead, support for continuous block production paves the way for the introduction
of [chain/validator-driven actions](https://github.com/ava-labs/hypersdk/issues/336), which should
be included on-chain every X seconds (like a price oracle update) regardless of how many user-submitted
transactions are present.

### Unified Metrics, Tracing, and Logging
It is functionally impossible to improve the performance of any runtime without
detailed metrics and comprehensive tracing. For this reason, the `hypersdk`
provides both to any `hypervm` out-of-the-box. These metrics and traces are
aggregated by avalanchego and can be accessed using the
[`/ext/metrics`](https://docs.avax.network/apis/avalanchego/apis/metrics)
endpoint. Additionally, all logs in the `hypersdk` use the standard `avalanchego` logger
and are stored alongside all other runtime logs. The unification of all of
these functions with avalanchego means existing avalanchego monitoring tools
work out of the box on your `hypervm`.

## Examples
We've created three `hypervm` examples, of increasing complexity, that demonstrate what you
can build with the `hypersdk` (with more on the way).

When you are ready to build your own `hypervm`, we recommend using the `morpheusvm` as a template!

### Beginner: `morpheusvm`
_[Who is Morpheus ("The Matrix")?](https://www.youtube.com/watch?v=zE7PKRjrid4)_

The [`morpheusvm`](./examples/morpheusvm) provides the first glimpse into the world of the `hypersdk`.
After learning how to implement native token transfers in a `hypervm` (one of the simplest Custom VMs
you could make), you will have the choice to go deeper (red pill) or to turn back to the VMs that you
already know (blue pill).

_To ensure the `hypersdk` remains reliable as we optimize and evolve the codebase,
we also run E2E tests in the `morpheusvm` on each PR to the `hypersdk` core modules._

### Moderate: `tokenvm`
We created the [`tokenvm`](./examples/tokenvm) to showcase how to use the
`hypersdk` in an application most readers are already familiar with, token minting
and token trading.

The `tokenvm` lets anyone create any asset, mint more of
their asset, modify the metadata of their asset (if they reveal some info), and
burn their asset. Additionally, there is an embedded on-chain exchange that
allows anyone to create orders and fill (partial) orders of anyone else. To
make this example easy to play with, the `tokenvm` also bundles a powerful CLI
tool and serves RPC requests for trades out of an in-memory order book it
maintains by syncing blocks. If you are interested in the intersection of
exchanges and blockchains, it is definitely worth a read (the logic for filling
orders is < 100 lines of code!).

_To ensure the `hypersdk` remains reliable as we optimize and evolve the codebase,
we also run E2E tests in the `tokenvm` on each PR to the `hypersdk` core modules._

### Expert: `indexvm` [DEPRECATED]
_The `indexvm` will be rewritten using the new WASM Programs module._

The [`indexvm`](https://github.com/ava-labs/indexvm) is much more complex than
the `tokenvm` (more elaborate mechanisms and a new use case you may not be
familiar with). It was built during the design of the `hypersdk` to test out the
limits of the abstractions for building complex on-chain mechanisms. We recommend
taking a look at this `hypervm` once you already have familiarity with the `hypersdk` to gain an
even deeper understanding of how you can build a complex runtime on top of the `hypersdk`.

The `indexvm` is dedicated to increasing the usefulness of the world's
content-addressable data (like IPFS) by enabling anyone to "index it" by
providing useful annotations (i.e. ratings, abuse reports, etc.) on it.
Think up/down vote on any static file on the decentralized web.

The transparent data feed generated by interactions on the `indexvm` can
then be used by any third-party (or yourself) to build an AI/recommender
system to curate things people might find interesting, based on their
previous interactions/annotations.

Less technical plz? Think TikTok/StumbleUpon over arbitrary IPFS data (like NFTs) but
all your previous likes (across all services you've ever used) can be used to
generate the next content recommendation for you.

The fastest way to expedite the transition to a decentralized web is to make it
more fun and more useful than the existing web. The `indexvm` hopes to play
a small part in this movement by making it easier for anyone to generate
world-class recommendations for anyone on the internet, even if you've never
interacted with them before.

We'll use both of these `hypervms` to explain how to use the `hypersdk` below.

## How It Works
To use the `hypersdk`, you must import it into your own `hypervm` and implement the
required interfaces. Below, we'll cover some of the ones that your
`hypervm` must implement.

> _Note: `hypersdk` requires a minimum Go version of 1.21_

### Controller
```golang
type Controller interface {
	Initialize(
		inner *VM, // hypersdk VM
		snowCtx *snow.Context,
		gatherer ametrics.MultiGatherer,
		genesisBytes []byte,
		upgradeBytes []byte,
		configBytes []byte,
	) (
		config Config,
		genesis Genesis,
		builder builder.Builder,
		gossiper gossiper.Gossiper,
		vmDB database.Database,
		stateDB database.Database,
		handler Handlers,
		actionRegistry chain.ActionRegistry,
		authRegistry chain.AuthRegistry,
		authEngines map[uint8]AuthEngine,
		err error,
	)

	Rules(t int64) chain.Rules // ms

	// StateManager is used by the VM to request keys to store required
	// information in state (without clobbering things the Controller is
	// storing).
	StateManager() chain.StateManager

	// Anything that the VM wishes to store outside of state or blocks must be
	// recorded here
	Accepted(ctx context.Context, blk *chain.StatelessBlock) error
	Rejected(ctx context.Context, blk *chain.StatelessBlock) error

	// Shutdown should be used by the [Controller] to terminate any async
	// processes it may be running in the background. It is invoked when
	// `vm.Shutdown` is called.
	Shutdown(context.Context) error
}
```

The `Controller` is the entry point of any `hypervm`. It initializes the data
structures utilized by the `hypersdk` and handles both `Accepted` and
`Rejected` block callbacks. Most `hypervms` use the default `Builder`,
`Gossiper`, `Handlers`, and `Database` packages so this is typically a lot of
boilerplate code.

You can view what this looks like in the `tokenvm` by clicking this
[link](./examples/tokenvm/controller/controller.go).

#### Registry
```golang
ActionRegistry *codec.TypeParser[Action, bool]
AuthRegistry   *codec.TypeParser[Auth, bool]
```

The `ActionRegistry` and `AuthRegistry` inform the `hypersdk` how to
marshal/unmarshal bytes on-the-wire. If the `Controller` did not provide these,
the `hypersdk` would not know how to extract anything from the bytes it was
provided by the Avalanche Consensus Engine.

_In the future, we will provide an option to automatically marshal/unmarshal
objects if an `ActionRegistry` and/or `AuthRegistry` is not provided using
a default codec._

### Genesis
```golang
type Genesis interface {
	Load(context.Context, atrace.Tracer, state.Mutable) error
}
```

`Genesis` is typically the list of initial balances that accounts have at the
start of the network and a list of default configurations that exist at the
start of the network (fee price, enabled txs, etc.). The serialized genesis of
any `hyperchain` is persisted on the P-Chain for anyone to see when the network
is created.

You can view what this looks like in the `tokenvm` by clicking this
[link](./examples/tokenvm/genesis/genesis.go).

### Action
```golang
type Action interface {
	Object

	// MaxComputeUnits is the maximum amount of compute a given [Action] could use. This is
	// used to determine whether the [Action] can be included in a given block and to compute
	// the required fee to execute.
	//
	// Developers should make every effort to bound this as tightly to the actual max so that
	// users don't need to have a large balance to call an [Action] (must prepay fee before execution).
	MaxComputeUnits(Rules) uint64

	// StateKeysMaxChunks is used to estimate the fee a transaction should pay. It includes the max
	// chunks each state key could use without requiring the state keys to actually be provided (may
	// not be known until execution).
	StateKeysMaxChunks() []uint16

	// StateKeys is a full enumeration of all database keys that could be touched during execution
	// of an [Action]. This is used to prefetch state and will be used to parallelize execution (making
	// an execution tree is trivial).
	//
	// All keys specified must be suffixed with the number of chunks that could ever be read from that
	// key (formatted as a big-endian uint16). This is used to automatically calculate storage usage.
	//
	// If any key is removed and then re-created, this will count as a creation instead of a modification.
	StateKeys(actor codec.Address, txID ids.ID) state.Keys

	// Execute actually runs the [Action]. Any state changes that the [Action] performs should
	// be done here.
	//
	// If any keys are touched during [Execute] that are not specified in [StateKeys], the transaction
	// will revert and the max fee will be charged.
	//
	// An error should only be returned if a fatal error was encountered, otherwise [success] should
	// be marked as false and fees will still be charged.
	Execute(
		ctx context.Context,
		r Rules,
		mu state.Mutable,
		timestamp int64,
		actor codec.Address,
		txID ids.ID,
	) (success bool, computeUnits uint64, output []byte, err error)
}
```

`Actions` are the heart of any `hypervm`. They define how users interact with
the blockchain runtime. Specifically, they are "user-defined" element of
any `hypersdk` transaction that is processed by all participants of any
`hyperchain`.

You can view what a simple transfer `Action` looks like [here](./examples/tokenvm/actions/transfer.go)
and what a more complex "fill order" `Action` looks like [here](./examples/tokenvm/actions/fill_order.go).

#### Result
```golang
type Result struct {
	Success bool
	Output  []byte

	Consumed Dimensions
	Fee      uint64
}
```

`Actions` emit a `Result` at the end of their execution. This `Result`
indicates if the execution was a `Success` (if not, all effects are rolled
back), how many `Units` were used (failed execution may not use all units an
`Action` requested), an `Output` (arbitrary bytes specific to the `hypervm`).

### Auth
```golang
type Auth interface {
	Object

	// ComputeUnits is the amount of compute required to call [Verify]. This is
	// used to determine whether [Auth] can be included in a given block and to compute
	// the required fee to execute.
	ComputeUnits(Rules) uint64

	// Verify is run concurrently during transaction verification. It may not be run by the time
	// a transaction is executed but will be checked before a [Transaction] is considered successful.
	// Verify is typically used to perform cryptographic operations.
	Verify(ctx context.Context, msg []byte) error

	// Actor is the subject of the [Action] signed.
	//
	// To avoid collisions with other [Auth] modules, this must be prefixed
	// by the [TypeID].
	Actor() codec.Address

	// Sponsor is the fee payer of the [Action] signed.
	//
	// If the [Actor] is not the same as [Sponsor], it is likely that the [Actor] signature
	// is wrapped by the [Sponsor] signature. It is important that the [Actor], in this case,
	// signs the [Sponsor] address or else their transaction could be replayed.
	//
	// TODO: add a standard sponsor wrapper auth (but this does not need to be handled natively)
	//
	// To avoid collisions with other [Auth] modules, this must be prefixed
	// by the [TypeID].
	Sponsor() codec.Address
}
```

The `Auth` mechanism is arguably the most powerful core module of the
`hypersdk` because it lets the builder create arbitrary authentication rules
that align with their goals.

### Rules
```golang
type Rules interface {
	// Should almost always be constant (unless there is a fork of
	// a live network)
	NetworkID() uint32
	ChainID() ids.ID

	GetMinBlockGap() int64      // in milliseconds
	GetMinEmptyBlockGap() int64 // in milliseconds
	GetValidityWindow() int64   // in milliseconds

	GetMinUnitPrice() Dimensions
	GetUnitPriceChangeDenominator() Dimensions
	GetWindowTargetUnits() Dimensions
	GetMaxBlockUnits() Dimensions

	GetBaseComputeUnits() uint64

	// Invariants:
	// * Controllers must manage the max key length and max value length (max network
	//   limit is ~2MB)
	// * Creating a new key involves first allocating and then writing
	// * Keys are only charged once per transaction (even if used multiple times), it is
	//   up to the controller to ensure multiple usage has some compute cost
	//
	// Interesting Scenarios:
	// * If a key is created and then modified during a transaction, the second
	//   read will be a read of 0 chunks (reads are based on disk contents before exec)
	// * If a key is removed and then re-created with the same value during a transaction,
	//   it doesn't count as a modification (returning to the current value on-disk is a no-op)
	GetSponsorStateKeysMaxChunks() []uint16
	GetStorageKeyReadUnits() uint64
	GetStorageValueReadUnits() uint64 // per chunk
	GetStorageKeyAllocateUnits() uint64
	GetStorageValueAllocateUnits() uint64 // per chunk
	GetStorageKeyWriteUnits() uint64
	GetStorageValueWriteUnits() uint64 // per chunk

	FetchCustom(string) (any, bool)
}
```

`Rules` govern block validity and are requested from the `Controller` prior to
executing any block. The `hypersdk` performs this request so that the
`Controller` can modify any `Rules` on-the-fly. Many common rules are provided
directly in the interface but there is also an option to provide custom rules
that can be accessed during `Auth` or `Action` execution.

You can view what this looks like in the `indexvm` by clicking
[here](https://github.com/ava-labs/indexvm/blob/main/genesis/rules.go). In the
case of the `indexvm`, the custom rule support is used to set the cost for
adding anything to state (which is a very `hypervm-specific` value).

You can view what the import `Action` associated with the above examples looks like
[here](./examples/tokenvm/actions/import_asset.go)

_As mentioned above, it is up to the `hypervm` to implement a message format
that it can understand (so that it can parse inbound AWM messages). In the
future, we expect that there will be common message definitions that will be
compatible with most `hypervms` (and maintained in the `hypersdk`)._

## Star History
[![Star History](https://starchart.cc/ava-labs/hypersdk.svg)](https://starchart.cc/ava-labs/hypersdk)

## Community Posts
_This is a collection of posts from the community about the `hypersdk` and how to use it in your own `hypervm`._

* [Introducing HyperSDK](https://twitter.com/_patrickogrady/status/1628109791267819520)
* [HyperSDK - Chorus One](https://twitter.com/ChorusOne/status/1628404359381024775)
* [Building blockchains in days w/ HyperSDK](https://0xronin.substack.com/p/building-blockchains-in-days-w-hypersdk?r=1qbgyb&utm_campaign=post&utm_medium=web)
* [An Analysis of the Developing State of Avalanche’s Technology](https://www.thetie.io/insights/research/an-analysis-of-the-developing-state-of-avalanches-technology/)
* [Launching Custom Tokens With HyperSDK By Avalanche](https://pythontony.hashnode.dev/launching-custom-tokens-with-hypersdk-by-avalanche)
* [Avalanche VMs deep-dive #1: HyperSDK/tokenvm](https://ashavax.hashnode.dev/avalanche-vms-deep-dive-1-hypersdktokenvm)
* [Avalanche – Building High Performance VMs With HyperSDK](https://epicenter.tv/episodes/506/)
* [Avalanche’s HyperSDK blockchain upgrade hits 143K TPS on testnet](https://cointelegraph.com/news/avalanche-hyper-sdk-blockchain-upgrade-hits-143000-tps-on-testnet)
* [Avalanche’s HyperSDK](https://simpleswap.io/blog/avalanches-hyper-sdk)

## Community Projects
_This is a collection of community projects building on top of the `hypersdk`._

* [NodeKit: Decentralizing The L2 Sequencer on a Subnet](https://github.com/AnomalyFi/nodekit-seq)
* [OracleVM: Providing OffChain Data to the Avalanche Ecosystem](https://github.com/bianyuanop/oraclevm)

## Future Work
_If you want to take the lead on any of these items, please
[start a discussion](https://github.com/ava-labs/hypersdk/discussions) or reach
out on the Avalanche Discord._

* Add support for Fixed-Fee Accounts (pay set unit price no matter what)
* Use a memory arena (pre-allocated memory) to avoid needing to dynamically
  allocate memory during block  and transaction parsing
* Add a module that does Data Availability sampling on top of the networking
  interface exposed by AvalancheGo (only store hashes in blocks but leave VM to
  fetch pieces as needed on its own)
* Implement support for S3 and PostgreSQL storage backends
* Provide optional auto-serialization/deserialization of `Actions` and `Auth`
  if only certain types are used in their definition
* Add a module that could be used to track the location of various pieces
  of data across a network ([see consistent
  hasher](https://github.com/ava-labs/avalanchego/tree/master/utils/hashing/consistent))
  of `hypervm` participants (even better if this is made abstract to any implementer
  such that they can just register and request data from it and it is automatically
  handled by the network layer). This module should make it possible for an
  operator to use a single backend (like S3) to power storage for multiple
  hosts.
* Only set `export CGO_CFLAGS="-O -D__BLST_PORTABLE__"` when running on
  MacOS/Windows (will make Linux much more performant)

## Troubleshooting
### `undefined: Message`
If you get the following error, make sure to install `gcc` before running
`./scripts/build.sh`:
```
# github.com/supranational/blst/bindings/go
../../../go/pkg/mod/github.com/supranational/blst@v0.3.11-0.20220920110316-f72618070295/bindings/go/rb_tree.go:130:18: undefined: Message
```
