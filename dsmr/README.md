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

## Components

### Chunk Building

Chunk building takes the place of block building in this model. This means that we replace the current block building code, which produces a valid block from the preferred tip of the chain and the contents of the mempool, with each validator building a local chunk from its local view of the mempool.

Once the node builds a chunk, it's responsible for requesting signatures from the network and broadcasting the chunk certificate to the network.

### Chunk Server

The chunk server is responsible for handling p2p requests for chunks and chunk signatures. After the chunk server has signed a chunk, it's responsible to store that chunk until it has either been included in an accepted block, or it has been marked as expired because the chain has moved forward to a point where it's no longer valid to be included.

Since the chunk server provides chunk storage, it also replaces the transaction mempool in block building. Validators build blocks by referencing valid verifiable chunk certificates, which it pulls in from the chunk server.

### Block Building

Block building iterates over the currently valid chunks in the chunk server to build a block. To support additional chunk verification at build time, we can include a verifier with the following function signature:

```golang
type BuildChunkVerifier interface {
    VerifyChunk(chunk ChunkCertificate, timestamp int64) (bool, error)
}
```

If the verifier returns a non-nil error, we drop the chunk certificate. This could be the case if a chunk has not expired yet, but would be invalid at the provided timestamp.

If the verifier returns `false`, we return early with the chunk certificates that have already been included.

This simplifies the existing block building code, since we've moved transaction pre-verification into chunk building and separated out execution by executing the block after it's been accepted.

### Block Verification

DSMR changes the block verification condition from executing the block to verifying chunk certificates within the given block's context.

We require that all chunks have received a valid 2f + 1 signatures from the validator set according to the block context and that all chunks are considered valid at the given timestamp (no expired chunk certificates).

In Avalanche VMs, the validator set changes based on the P-Chain height included in the ProposerVM, which is out of our control. To handle this, we can add an epoch-ing mechanism to make chunk certificates more stable for a changing validator set.

### Block Processing

Block processing occurs only AFTER we have already verified and accepted a block. After a block has been accepted, we:

- fetch and store any missing chunks included in the block
- filter out duplicate transactions and re-assemble the full block
- execute the assembled block

We should add a configurable `assembler` component that's responsible for assembling a block from the parameters included in the chunk block (height, timestamp, previously assembled block, etc) and executing it. This should decouple the `chain.Transaction` and `chain.Block` type from DSMR

## Implementation

- Implement network handlers (`requestChunkSignatureShare`, `chunkSignature`, `requestChunk`, `chunk`)
- Implement chunk + local verification context + chunk builder
- Implement chunk server
- Implement chunk block builder
- Implement chunk block verifier
- Implement chunk block executor
