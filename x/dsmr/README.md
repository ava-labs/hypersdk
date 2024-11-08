# DSMR

## Chunks

Chunks wrap a group of containers (transactions) submitted by a single node. Each validator is responsible for building chunks locally with their own view of the mempool, request signatures from a threshold of the network to mark them as replicated, and distribute chunk certificates with >= 2f + 1 signatures that are eligible to be included in a block.

The network runs consensus over chunk-based blocks, where a block is valid if it contains only chunks with valid certificates (>=2f + 1 signatures). This enables validators to defer fetching the full chunk contents and execution until after the block has been accepted by the network.

### Chunk Rate Limiting

Chunks include a `Slot` (typically a timestamp), so that chunk verification (prior to chunk signing) can rate limit the number of chunks it's willing to sign within a certain period for a given validator. This requires the chunk server component to maintain some state about the amount of resources consumed by chunks it has signed for a given validator.

A simple example may be to allow each validator to produce a single chunk per `Slot`, so that if a validator attempts to create two conflicting chunks, they can produce at most one valid chunk certificate. Alternatively, chunk verification could utilize `fees.Dimensions` to describe the total resources consumed by chunks that it's produced and limit the amount of resources consumed by chunks from a single validator over a given period of time.

Note: conflicting chunks are a provable fault, so we could implement slashing here, but we omit it for now since they do not cause the network any harm.

### Chunk Expiry

After validators sign a chunk, they commit to store the chunk contents to guarantee availability if it's included in a block. However, there's no guarantee a signed chunk will be eventually included, so we handle garbage collection by marking chunks as expired once the blockchain's time has moved past the expiration (configurable) of the chunk slot.

## Breakdown

### Chunk Storage

- Store non-persistent pool of chunk certificates
- Verify chunks, distribute signature shares, and commit to persist chunks we've signed
- Handle chunk acceptance / expiry by committing chunks to disk

TODO
- Switch from using Delete to DeleteRange where applicable
- Add retention limit for accepted chunk storage (can use DeleteRange here as well)
- Upper bound memory consumption
- Consider switching emap from `int64` to `uint64` to avoid unnecessary type conversions
- Support returning chunks in expiry order from `GatherChunkCertificates`

### P2P Client/Server w/ ACP-118 and Chunk Builder

- Implement P2P client/server functions for: `getChunk`, `putChunk`, `getSignatureShare`, `putSignatureShare`, and `putChunkCertificate`.
- Implement chunk builder that triggers chunk building and distribution via a channel (can be triggered via either sufficient transactions or timer)
- Migrate testing from storage to P2P layer, so that the tests run against the interface exported by DSMR

### Block Builder

- Implement `BuildBlock` that uses `GatherChunkCerts() []*ChunkCertificate` from the storage struct
- Return a `ChunkBlock` type that implements `snowman.Block`
- Verify verifies every chunk certificate
- Reject is a no-op that abandons the block (will need to handle duplicate chunks across processing chain of blocks)
- Accept should be a no-op until we've implemented the chunk block executor in the next stage

### Chunk Block Executor

- fetch and store any chunks that are not stored locally
- Filter duplicate transactions
- define block assembler interface to assemble, execute, and accept a block

```golang
type Assembler[T Tx, State any, Block any, Result any] interface {
	AssembleBlock(ctx context.Context, parentState State, parentBlock Block, timestamp int64, blockHeight uint64, txs []T) (Block, Result, State, error)
}
```

The chunk block executor's only dependency is the ability to fetch chunks by ID. Since the client/server will wrap the storage backend, it should provide at least this interface:

```golang
type Backend interface {
    GetChunks(chunkID []ids.ID) ([]*Chunk[T], error)
}
```

If `GetChunk` fails, it should be considered a fatal error since we must be able to fetch chunks for already accepted blocks. This means that the client implementation will need provide its own retry logic to fetch the chunk from the network.

Future TODOs:
- backpressure if the chain is moving faster than we can backfill chunks from accepted blocks
- apply fortification fees

Qs
- Should the assembled block use the parentID of the last assembled block (internal block) or the chunk block (wrapper block)?
- How should we handled failed chunk requests for an accepted block?
- Can the chunk certificate provide a hint to the p2p client/server to indicate where it can fetch the chunks from?

### Block Assembler + Executor / Integrate into HyperSDK w/ *chain.Block type

Define the `Assembler` and `Executor` types for the current `*chain.Block` type. The `Result` should be `*chain.ExecutedBlock`, so that we can pipe the result through to our current APIs that require `event.Subscription[*chain.ExecutedBlock]`.

### Swap Ghost Signatures / Certs for Warp Verification

- Switch from using empty implementations of `ChunkSignatureShare` and `ChunkCertificate` to using Warp signatures
- Add epoch'ed validator sets to improve stability (might be added directly to ProposerVM)

