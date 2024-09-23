# DSMR

## Chunks

Chunks wrap a group of containers submitted by a single node. Each validator is responsible for building chunks locally with their own view of the mempool, request signatures from a threshold of the network to mark them as replicated, and distribute chunk certificates with >= 2f + 1 signatures that are eligible to be included in a block.

The network runs consensus over chunk-based blocks, where a block is valid if it contains only chunks with valid certificates (>=2f + 1 signatures). This enables validators to defer fetching the full chunk contents and execution until after the block has been accepted by the network.

### Chunk Rate Limiting

Each validator can only produce a limited number of chunks per slot. Each node in the network will only ever sign a single chunk for every (validator, slot) pair in the network.

This gives us a strong bound on the amount of work for the network by a single validator. If a validator attempts to create two conflicting chunks, they will only be able to create one chunk certificate (certificate requires 2f + 1 signatures)

Note: conflicting chunks are a provable fault, so we could implement slashing here, but we omit it since they do not cause the network any harm.

### Chunk Expiry

After validators sign a chunk, they commit to store the chunk contents to guarantee availability if it's included in a block. However, there's no guarantee a signed chunk will be eventually included, so we handle garbage collection by marking chunks as expired once the blockchain's time has moved past the expiration (configurable) of the chunk slot.

## Implementation

* requestChunkSignatureShare
* chunkSignature
* requestChunk
* chunk

### Chunk Building

### Block Building

### Block Verification

### Block Processing

### Extensible DSMR

- Implement chunk + local verification context + chunk builder
- Implement chunk storage / signer / aggregate signer
- Implement chunk block builder
- Implement chunk block executor
