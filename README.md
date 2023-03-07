<p align="center">
  <img width="90%" alt="hypersdk" src="assets/logo.png">
</p>
<p align="center">
  Opinionated Framework for Building Hyper-Scalable Blockchains on Avalanche
</p>
<p align="center">
  <a href="https://goreportcard.com/report/github.com/ava-labs/hypersdk"><img src="https://goreportcard.com/badge/github.com/ava-labs/hypersdk" /></a>
  <a href="https://github.com/ava-labs/hypersdk/actions/workflows/unit-tests.yml"><img src="https://github.com/ava-labs/hypersdk/actions/workflows/unit-tests.yml/badge.svg" /></a>
  <a href="https://github.com/ava-labs/hypersdk/actions/workflows/static-analysis.yml"><img src="https://github.com/ava-labs/hypersdk/actions/workflows/static-analysis.yml/badge.svg" /></a>
</p>

The freedom to create your own [Virtual Machine (VM)](https://docs.avax.network/subnets#virtual-machines),
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
code to add custom interaction patterns (and don't need to copy-pasta code from upstream
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
behind the network while syncing this state, the `hypersdk` acts as an Avalanche Light
Client and performs consensus on newly processed blocks without verifying them (updating its
state sync target whenever a new block is accepted).

The `hypersdk` relies on [`x/sync`](https://github.com/ava-labs/avalanchego/tree/master/x/sync),
a bandwidth-aware dynamic sync implementation provided by `avalanchego`, to
sync to the tip of any `hyperchain`.

#### Pebble as Default
Instead of employing [`goleveldb`](https://github.com/syndtr/goleveldb), the
`hypersdk` uses CockroachDB's [`pebble`](https://github.com/cockroachdb/pebble) database for
on-disk storage. This database is inspired by LevelDB/RocksDB but offers [a few
improvements](https://github.com/cockroachdb/pebble#advantages).

Unlike other Avalanche VMs, which store data inside `avalanchego's` root
database, `hypervms` store different types of data (state, blocks, metadata, etc.) under
a set of distinct paths in `avalanchego's` provided `chainData` directory.
This structure enables anyone running a `hypervm` to employ multiple logical disk
drives to increase a `hyperchain's` throughput (which may otherwise be capped by a single disk's IO).

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

#### State Pre-Fetching
`hypersdk` transactions must specify the keys they will touch in state (read
or write) during execution and authentication so that all relevant data can be
pre-fetched before block execution starts, which ensures all data accessed during
verification of a block is done so in memory). Notably, the keys specified here
are not keys in a merkle trie (which may be quite volatile) but are instead the
actual keys used to access data by the storage engine (like your address, which
is much less volatile and not as cumbersome of a UX barrier).

This restriction also enables transactions to be processed in parallel as distinct,
ordered transaction sets can be trivially formed by looking at the overlap of keys
that transactions will touch.

_Parallel transaction execution was originally included in `hypersdk` but
removed because the overhead of the naïve mechanism used to group transactions
into execution sets prior to execution was slower than just executing transactions
serially with state pre-fetching. Rewriting this mechanism has been moved to the
`Future Work` section and we expect to re-enable this functionality soon._

#### Parallel Signature Verification
The `Auth` interface (detailed below) exposes a function called `AsyncVerify` that
the `hypersdk` may call concurrently (may invoke on other transactions in the same
block) at any time prior/during block execution. Most `hypervms` perform signature
verification in this function and save any state lookups for the full `Auth.Verify`
(which has access to state, unlike `AsyncVerify`). The generic support for performing certain
stateless activities during execution can greatly reduce the e2e verification
time of a block when running on powerful hardware.

### Account Abstraction
The `hypersdk` makes no assumptions about how `Actions` (the primitive for
interactions with any `hyperchain`, as explained below) are verified. Rather,
`hypervm's` provide the `hypersdk` with a registry of supported `Auth` modules
that can be used to validate each type of transaction. These `Auth` modules can
perform simple things like signature verification or complex tasks like
executing a WASM blob.

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
### Beginner: `tokenvm`
We created the [`tokenvm`](./examples/tokenvm) to showcase how to use the
`hypersdk` in an application most readers are already familiar with, token minting
and token trading. The `tokenvm` lets anyone create any asset, mint more of
their asset, modify the metadata of their asset (if they reveal some info), and
burn their asset. Additionally, there is an embedded on-chain exchange that
allows anyone to create orders and fill (partial) orders of anyone else. To
make this example easy to play with, the `tokenvm` also bundles a powerful CLI
tool and serves RPC requests for trades out of an in-memory order book it
maintains by syncing blocks. If you are interested in the intersection of
exchanges and blockchains, it is definitely worth a read (the logic for filling
orders is < 100 lines of code!).

To ensure the `hypersdk` stays reliable as we optimize and evolve the codebase,
we also run E2E tests in the `tokenvm` on each PR to the `hypersdk` core modules.

### Expert: `indexvm`
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

> _Note: `hypersdk` requires a minimum Go version of 1.20_

### Controller
```golang
type Controller interface {
	Initialize(
		inner *VM,
		snowCtx *snow.Context,
		gatherer metrics.MultiGatherer,
		genesisBytes []byte,
		upgradeBytes []byte,
		configBytes []byte,
	) (
		config Config,
		genesis Genesis,
		builder builder.Builder,
		gossiper gossiper.Gossiper,
		blockDB KVDatabase,
		stateDB database.Database,
		handler Handlers,
		actionRegistry chain.ActionRegistry,
		authRegistry chain.AuthRegistry,
		err error,
	)

	Rules(t int64) chain.Rules

	Accepted(ctx context.Context, blk *chain.StatelessBlock) error
	Rejected(ctx context.Context, blk *chain.StatelessBlock) error
}
```

The `Controller` is the entry point of any `hypervm`. It initializes the data
structures utilized by the `hypersdk` and handles both `Accepted` and
`Rejected` block callbacks. Most `hypervms` use the default `Builder`,
`Gossiper`, `Handlers`, and `Database` packages so this is typically a lot of
boilerplate code.

You can view what this looks like in the `tokenvm` by clicking this
[link](./examples/tokenvm/controller/controller.go).

### Genesis
```golang
type Genesis interface {
	GetHRP() string
	Load(context.Context, atrace.Tracer, chain.Database) error
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
	MaxUnits(Rules) uint64
	ValidRange(Rules) (start int64, end int64)

	StateKeys(Auth) [][]byte
	Execute(ctx context.Context, r Rules, db Database, timestamp int64, auth Auth, txID ids.ID) (result *Result, err error)

	Marshal(p *codec.Packer)
}
```

`Actions` are the heart of any `hypervm`. They define how users interact with
the blockchain runtime. Specifically, they are "user-defined" element of
any `hypersdk` transaction that is processed by all participants of any
`hyperchain`.

You can view what a simple transfer `Action` looks like [here](./examples/tokenvm/actions/transfer.go)
and what a more complex "fill order" `Action` looks like [here](./examples/tokenvm/actions/fill_order.go).

### Auth
```golang
type Auth interface {
	MaxUnits(Rules) uint64
	ValidRange(Rules) (start int64, end int64)

	StateKeys() [][]byte
	AsyncVerify(msg []byte) error
	Verify(ctx context.Context, r Rules, db Database, action Action) (units uint64, err error)

	Payer() []byte
	CanDeduct(ctx context.Context, db Database, amount uint64) error
	Deduct(ctx context.Context, db Database, amount uint64) error
	Refund(ctx context.Context, db Database, amount uint64) error

	Marshal(p *codec.Packer)
}
```

`Auth` shares many similarities with `Action` (recall that authentication is
abstract and defined by the `hypervm`) but adds the notion of some abstract
"payer" that must pay fees for the operations that occur in an `Action`. Any
fees that are not consumed can be returned to said "payer" if specified in the
corresponding `Action` that was authenticated.

The `Auth` mechanism is arguably the most powerful core module of the
`hypersdk` because it lets the builder create arbitrary authentication rules
that align with their goals. The `indexvm`, for example, allows users to rotate
their keys and to enable others to perform specific actions on their behalf. It also
lets accounts natively pay for the fees of other accounts. These features are particularly
useful for server-based accounts that want to implement a periodic key rotation
scheme without losing the history of their rating activity on-chain (which
determines their reputation).

You can view what direct (simple account signature) `Auth` looks like
[here](https://github.com/ava-labs/indexvm/blob/main/auth/direct.go) and what
delegate (acting on behalf of another account) `Auth` looks like
[here](https://github.com/ava-labs/indexvm/blob/main/auth/delegate.go). The
`indexvm` provides an ["authorize" `Action`](https://github.com/ava-labs/indexvm/blob/main/actions/authorize.go)
that an account owner can call to perform any ACL modifications.

### Rules
```golang
type Rules interface {
	GetChainID() ids.ID

	GetMaxBlockTxs() int
	GetMaxBlockUnits() uint64

	GetValidityWindow() int64
	GetBaseUnits() uint64

	GetMinUnitPrice() uint64
	GetUnitPriceChangeDenominator() uint64
	GetWindowTargetUnits() uint64

	GetMinBlockCost() uint64
	GetBlockCostChangeDenominator() uint64
	GetWindowTargetBlocks() uint64

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

## Future Work
_If you want to take the lead on any of these items, please
[start a discussion](https://github.com/ava-labs/hypersdk/discussions) or reach
out on the Avalanche Discord._

* Use pre-specified state keys to process transactions in parallel (txs with no
  overlap can be processed at the same time, create conflict sets on-the-fly
  instead of before execution)
* Add support for Avalanche Warp Messaging (AWM) so any deployed hypervms
  (hyperchains) can communicate with each other ([see ava-labs/xsvm](https://github.com/ava-labs/xsvm))
* Add a WASM runtime module to allow developers to embed smart contract
  functionality in their hypervms
* Overhaul streaming RPC (properly heartbeat and close connections)
* Implement concurrent state pre-fetching in `chain/processor` (blocked on
  `x/merkledb` locking improvements)
* Create an embedded explorer and wallet that is compatible with any hypervm
* Add support for Fixed-Fee Accounts (pay set unit price no matter what)
* Add a state processing loop that always prioritizes access by `Verify` and
  `Build` over handing `Gossip` and `Submit` requests (can cause starvation of
  consensus process under load)
* Pre-fetch state during block production loop (currently 30-40% slower than
  normal execution)
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
  handled by the network layer)
