# DSMR

## Chunks

Chunks wrap a group of containers (transactions) submitted by a single node. Each validator is responsible for building chunks locally with their own view of the mempool, request signatures from a threshold of the network to mark them as replicated, and distribute chunk certificates with >= 2f + 1 signatures that are eligible to be included in a block.

The network runs consensus over chunk-based blocks, where a block is valid if it contains only chunks with valid certificates (>=2f + 1 signatures). This enables validators to defer fetching the full chunk contents and execution until after the block has been accepted by the network.

### Chunk Rate Limiting

Chunks include a `Slot` (typically a timestamp), so that chunk verification (prior to chunk signing) can rate limit the number of chunks it's willing to sign within a certain period for a given validator. 

A simple example may be to allow each validator to produce a single chunk per `Slot`, so that if a validator attempts to create two conflicting chunks, they can produce at most one valid chunk certificate.

Note: conflicting chunks are a provable fault, so we could implement slashing here, but we omit it since they do not cause the network any harm.

### Chunk Expiry

After validators sign a chunk, they commit to store the chunk contents to guarantee availability if it's included in a block. However, there's no guarantee a signed chunk will be eventually included, so we handle garbage collection by marking chunks as expired once the blockchain's time has moved past the expiration (configurable) of the chunk slot.

## Implementation

### Chunk Building

Chunk building takes the place of block building in this model. This means that we replace the current block building code, which produces a valid block from the preferred tip of the chain and the contents of the mempool, with each validator building a local chunk from its local view of the mempool.

Once the node builds a chunk, it's responsible for requesting signatures from the network and distributing a chunk certificate.

### Block Building

After chunks have been built, nodes should store both chunks that they have signed and committed to persisting until they've either been accepted or expired and chunk certificates that it's heard about and are therefore eligible to include in a block.

Block building should be very simple at this point because we simply need to fit as many chunks as we can that fit our criteria (ex. fit the maximum resources consumed per block).

This should be much simpler than the current block building code, since we've moved most of that complexity into chunk building.

### Block Verification

Chunk based block verification needs to verify a block given a certain parent. In the DSMR / chunk world, the only check we need to apply here is that each included chunk has received 2f + 1 signatures according to our current view of the validator set. Since the validator set received from AvalancheGo can change every block, this requires either using a static view of the validator set or adding epochs to our view of the validator set.

### Block Processing

Block processing occurs only AFTER we have already verified and accepted a block. When we accept a block, we need to make sure to fetch any chunks that we have not already received, re-assemble the block, filter out any duplicate transactions, and execute the block.

### Breakdown

#### P2P Client/Server w/ ACP-118 and Chunk Builder

- Implement P2P client/server functions for: `getChunk`, `putChunk`, `getSignatureShare`, `putSignatureShare`, and `putChunkCertificate`.
- Implement chunk builder that triggers chunk building and distribution via a channel (can be triggered via either sufficient transactions or timer)
- Migrate testing from storage to P2P layer, so that the tests run against the interface exported by DSMR

#### Block Builder

- Implement `BuildBlock` that uses `GatherChunkCerts() []*ChunkCertificate` from the storage struct
- Return a `ChunkBlock` type that implements `snowman.Block`
- Verify verifies every chunk certificate
- Reject is a no-op that abandons the block (will need to handle duplicate chunks across processing chain of blocks)
- Accept should be a no-op until we've implemented the chunk block executor in the next stage

#### Chunk Block Executor

- fetch and store any chunks that are not stored locally
- Filter duplicate transactions
- define block assembler interface to assemble an executable block

```golang
type Assembler[T Tx] interface {
    AssembleBlock(parentID ids.ID, blockID ids.ID, timestamp uint64, blockHeight uint64, txs []T) (Block, error)
}

type Executor[Block any, Result any] interface {
    ExecuteBlock(b Block) (Result, error)
}
```

Future TODOs:
- backpressure if the chain is moving faster than we can backfill chunks from accepted blocks
- apply fortification fees

#### Block Assembler + Executor / Integrate into HyperSDK w/ *chain.Block type

Define the `Assembler` and `Executor` types for the current `*chain.Block` type. The `Result` should be `*chain.ExecutedBlock`, so that we can pipe the result through to our current APIs that require `event.Subscription[*chain.ExecutedBlock]`.

#### Swap Ghost Signatures / Certs for Warp Verification

- Switch from using empty implementations of `ChunkSignatureShare` and `ChunkCertificate` to using Warp signatures
- Add epoch'ed validator sets to improve stability

