# Features
## Efficient State Management
All `hypersdk` state is stored using [`x/merkledb`](https://github.com/ava-labs/avalanchego/blob/master/x/merkledb/README.md),
a path-based merkelized radix tree implementation provided by `avalanchego`. This high-performance
data structure minimizes the on-disk footprint of any `hypervm` out-of-the-box by deleting
any data that is no longer part of the current state (without performing any costly reference counting).

The use of this type of data structure in the blockchain context was pioneered
by the [go-ethereum](https://github.com/ethereum/go-ethereum) team in an effort
to minimize the on-disk footprint of the EVM. We wanted to give a Huge shoutout
to that team for all the work they put into researching this approach.

### Dynamic State Sync
Instead of requiring nodes to execute all previous transactions when joining
any `hyperchain` (which may not be possible if there is very high throughput on a Subnet),
the `hypersdk` just syncs the most recent state from the network. To avoid falling
behind the network while syncing this state, the `hypersdk` acts as an Avalanche Lite
Client and performs consensus on newly processed blocks without verifying them (updating its
state sync target whenever a new block is accepted).

The `hypersdk` relies on [`x/sync`](https://github.com/ava-labs/avalanchego/tree/master/x/sync),
a bandwidth-aware dynamic sync implementation provided by `avalanchego`, to
sync to the tip of any `hyperchain`.

### Block Pruning
The `hypersdk` defaults to only storing what is necessary to build/verify the next block
and to help new nodes sync the current state (not execute historical state transitions).
If the `hypersdk` did not limit block storage growth, the disk requirements for validators
would grow at an alarming rate each day (making running any `hypervm` impractical).
Consider the simple example where we process 25k transactions per second (assume each
transaction is ~400 bytes); this would require the `hypersdk` to store 10MB per
second (not including any overhead in the database for doing so). **This works out to
864GB per day or 315.4TB per year.**

When `MinimumBlockGap=250ms` (minimum time between blocks), the `hypersdk` must store at
least ~240 blocks to allow for the entire `ValidityWindow` to be backfilled (otherwise
a fully-synced, restarting `hypervm` will not become "ready" until it accepts a block at
least `ValidityWindow` after the last accepted block). To provide some room for error during
disaster recovery (network outage), however, it is recommended to configure the `hypersdk` to
store the last >= ~50,000 accepted blocks (~3.5 hours of activity with a 250ms `MinimumBlockGap`).
This allows archival nodes that become disconnected from the network (due to a data center outage or bug)
to ensure they can persist all historical blocks (which would otherwise be deleted by all participants and
unindexable).

_The number of blocks that the `hypersdk` stores on-disk, the `AcceptedBlockWindow`, can be tuned by any `hypervm`
to an arbitrary depth (or set to `MaxInt` to keep all blocks). To limit disk IO used to serve blocks over
the P2P network, `hypervms` can configure `AcceptedBlockWindowCache` to store recent blocks in memory._

## WASM-Based Smart Contracts
In the `hypersdk`, smart contracts are referred to simply as `contracts`. `Contracts`
are [WASM-based](https://webassembly.org/) binaries that can be invoked during block
execution to perform arbitrary state transitions. This is a more flexible, yet less performant,
alternative to defining all `Auth` and/or `Actions` that can be invoked in the `hypervm` in the
`hypervm's` code.

Because the `hypersdk` can execute arbitrary WASM, any language (Rust, C, C++, Zig, etc.) that can
be compiled to WASM can be used to write `contracts`. You can view a collection of
Rust-based `contracts` [here](../../x/contracts/).

## Account Abstraction
The `hypersdk` provides out-of-the-box support for arbitrary transaction authorization logic.
Each `hypersdk` transaction includes an `Auth` object that implements an
`Actor` function (identity that participates in an `Action`) and a `Sponsor` function (identity
that pays fees). These two identities could be the same (if using a simple signature
verification `Auth` module) but may be different (if using a "gas relayer" `Auth` module).

`Auth` modules may be hardcoded, like in
[`morpheusvm`](https://github.com/ava-labs/hypersdk/tree/main/examples/morpheusvm/auth), or execute
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

## Optimized Block Execution Out-of-the-Box
The `hypersdk` is primarily about an obsession with hyper-speed and
hyper-scalability (and making it easy for developers to achieve both by
wrapping their work in opinionated and performant abstractions).
Developers don't care how easy it is to launch or maintain their own
blockchain if it can't process thousands of transactions per second with low
time-to-finality. For this reason, most development time on the `hypersdk`
thus far has been dedicated to making block verification and state management
as fast and efficient as possible, which both play a large role in making this
happen.

### Parallel Transaction Execution
`hypersdk` transactions must specify the keys they will access in state (read
and/or write) during authentication and execution so that non-conflicting transactions
can be processed in parallel. To do this efficiently, the `hypersdk` uses
the [`executor`](https://github.com/ava-labs/hypersdk/tree/main/executor) package, which
can generate an execution plan for a set of transactions on-the-fly (no preprocessing required).
`executor` is used to parallelize execution in both block building and in block verification.

When a `hypervm's` `Auth` and `Actions` are simple and pre-specified (like in the `morpheusvm`),
the primary benefit of parallel execution is to concurrently fetch the state needed for execution
(actual execution of precompiled golang only takes nanoseconds). However, parallel execution
massively speeds up the E2E execution of a block of `Actions`, which may each take a few milliseconds
to process. Consider the simple scenario where an `Action` takes 2 milliseconds; processing 1000 `Actions`
in serial would take 2 seconds (far too long for a high-throughput blockchain). The same execution, however,
would only take 125 milliseconds if run over 16 cores (assuming no conflicts).

_The number of cores that the `hypersdk` allocates to execution can be tuned by
any `hypervm` using the `TransactionExecutionCores` configuration._

### Deferred Root Generation
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
type StatelessBlock struct {
	Prnt   ids.ID `json:"parent"`
	Tmstmp int64  `json:"timestamp"`
	Hght   uint64 `json:"height"`

	Txs []*Transaction `json:"txs"`

	StateRoot   ids.ID     `json:"stateRoot"`
}
```

Most blockchains that store a state root in the block use the root of a merkle tree
of state post-execution, however, this requires waiting for state merklization to complete
before block verification can finish. If merklization was fast, this wouldn't be an
issue, however, this process is typically the most time consuming aspect of block
verification.

`hypersdk` blocks instead include the merkle root of the post-execution state of a block's
parent rather than a merkle root of their own post-execution state. This design enables the
`hypersdk` to generate the merkle root of a block's post-execution state asynchronously
while the consensus engine is working on other tasks that typically are network-bound rather
than CPU-bound, like merklization, making better use of all available resources.

### [Optional] Parallel Signature Verification
The `Auth` interface (detailed below) exposes a function called `AsyncVerify` that
the `hypersdk` may call concurrently (may invoke on other transactions in the same
block) at any time prior to/during block execution. Most `hypervms` perform signature
verification in this function and save any state lookups for the full `Auth.Verify`
(which has access to state, unlike `AsyncVerify`). The generic support for performing certain
stateless activities during execution can greatly reduce the e2e verification
time of a block when running on powerful hardware.

### [Optional] Batch Signature Verification
Some public-key signature systems, like [Ed25519](https://ed25519.cr.yp.to/), provide
support for verifying batches of signatures (which can be much more efficient than
verifying each signature individually). The `hypersdk` generically supports this
capability for any `Auth` module that implements the `AuthBatchVerifier` interface,
even parallelizing batch computation for systems that only use a single-thread to
verify a batch.

## Multidimensional Fee Pricing
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

### Invisible Support
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

### Avoiding Complex Construction
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
pessimistic (i.e. assumes the worst) and can lead to the maximum fee
for a transaction being ~2x as large as the fee it uses on-chain (depending
on the usage of cold/warm storage, as discussed later). In practice,
this means that accounts may need a larger balance than they otherwise
would to issue transactions (as the `MaxFee` must be payable during
execution). In the future, it will also be possible to optionally
specify a max usage of each unit dimension to better bound this pessimism.

### No Priority Fees
Transactions are executed in FIFO order by each validator and there is no
way for a user to specify some "priority" fee to have their transaction
included in a block sooner. If a transaction cannot be executed when
it is pulled from the mempool (because its `MaxFee` is insufficient), it will
be dropped and must be reissued.

Aside from FIFO handling being dramatically more efficient for each validator,
price-sorted mempools are not particularly useful in high-throughput
blockchains where the expected mempool size is ~0 or there is a bounded transaction
lifetime (60 seconds by default on the `hypersdk`).

### Separate Metering for Storage Reads, Allocates, Writes
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

### Size-Encoded Storage Keys
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

## Nonce-less and Expiring Transactions
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

## Action Batches and Arbitrary Outputs
Each `hypersdk` transaction specifies an array of `Actions` that
must all execute successfully for any state changes to be committed.
Additionally, each `Action` is permitted to return an array of outputs (each
output is arbitrary bytes defined by the `hypervm`) upon successful execution.

The `tokenvm` uses `Action` batches to offer complex, atomic interactions over simple
primitives (i.e. create order, fill order, and cancel order). For example, a user
can create a transaction that fills 8 orders. If any of the fills fail, all pending
state changes in the transaction are rolled back. The `tokenvm` uses `Action` outputs to
return the remaining units on any partially filled order to power an in-memory orderbook.

The outcome of execution is not stored/indexed by the `hypersdk`. Unlike most other
blockchains/blockchain frameworks, which provide an optional "archival mode" for historical access,
the `hypersdk` only stores what is necessary to validate the next valid block and to help new nodes
sync to the current state. Rather, the `hypersdk` invokes the `hypervm` with all execution
results whenever a block is accepted for it to perform arbitrary operations (as
required by a developer's use case). In this callback, a `hypervm` could store
results in a SQL database or write to a Kafka stream.

## Easy Functionality Upgrades
Every object that can appear on-chain (i.e. `Actions` and/or `Auth`) and every chain
parameter (i.e. `Unit Price`) is scoped by block timestamp. This makes it
possible to easily modify existing rules (like how much people pay for certain
types of transactions) or even disable certain types of `Actions` altogether.

Launching your own blockchain is the first step of a long journey of continuous
evolution. Making it straightforward and explicit to activate/deactivate any
feature or config is critical to making this evolution safely.

## Proposer-Aware Gossip
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

## Support for Generic Storage Backends
When initializing a `hypervm`, the developer explicitly specifies which storage backends
to use for each object type (state vs blocks vs metadata). As noted above, this
defaults to CockroachDB's `pebble` but can be swapped with experimental storage
backends and/or traditional cloud infrastructure. For example, a `hypervm`
developer may wish to manage state objects (for the Path-Based Merkelized Radix
Tree) on-disk but use S3 to store blocks and PostgreSQL to store transaction metadata.

## Continuous Block Production
`hypervms` produce blocks continuously (even if empty). `hypervms` produce produce empty blocks
only after `MinEmptyBlockGap` has passed. This improves the "worst case" AWM verification cost (AWM verification
requires creating a reverse diff to the last referenced P-Chain block), prevents a fallback to leaderless
block production (which can lead to more rejected blocks), and avoids a prolonged post-bootstrap
readiness wait (`hypersdk` waits to mark itself as ready until it has seen a `ValidityWindow` of blocks).

Looking ahead, support for continuous block production paves the way for the introduction
of [chain/validator-driven actions](https://github.com/ava-labs/hypersdk/issues/336), which should
be included on-chain every X seconds (like a price oracle update) regardless of how many user-submitted
transactions are present.

## Unified Metrics, Tracing, and Logging
It is functionally impossible to improve the performance of any runtime without
detailed metrics and comprehensive tracing. For this reason, the `hypersdk`
provides both to any `hypervm` out-of-the-box. These metrics and traces are
aggregated by avalanchego and can be accessed using the
[`/ext/metrics`](https://docs.avax.network/apis/avalanchego/apis/metrics)
endpoint. Additionally, all logs in the `hypersdk` use the standard `avalanchego` logger
and are stored alongside all other runtime logs. The unification of all of
these functions with avalanchego means existing avalanchego monitoring tools
work out of the box on your `hypervm`.
