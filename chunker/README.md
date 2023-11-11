Gossip Chunks to other validators. Desire to see a Chunk included
is based on validator's ability to distribute (no regossip by other
validators). Include transactions from our mempool even if we have already seen in other
chunks (peer may only send block to some people).

Chunks can be sent in any order by the signer but they cannot send
2 chunks with the same timestamp (malicious offense). We use a timestamp
here so we don't need to synchronize a counter when joining the network.
```
type Chunk struct {
    Signer: BLSPublicKey,
    Signature: BLSSignature,

    Timestamp: uint64, (modulo interval time of t milliseconds)
    Transactions: [
        <Transaction>,
        ...,
    ],
}
-> EndorseChunk
```

Only reply if all transactions are well-formatted, have valid async signatures. If not,
penalize sender (response to `Chunk`).
```
type EndorseChunk struct {
    Signer: BLSPublicKey,
    Signature: BLSSignature,

    Chunk: ids.ID,
}
```

Pushing signatures allows anyone to include a `Chunk` in their `Block`.
```
type Signature struct {
    Signers: BitSet,
    Signature: BLSSignature,

    Chunk: ids.ID,
}
```

If we are missing a chunk (must be a validator), we can request it:
```
type ChunkRequest struct {
    Chunk: ids.ID,
}
-> ChunkResponse
```

Respond with BLS multi-signature we've collected so far + Chunk:
```
type ChunkResponse struct {
    Signers: BitSet,
    Signature: BLSSignature,

    Chunk: Chunk,
}
```

A validator should only include a chunk in a block once it has Z% of stake signing it.
By filtering pre-consensus data, we get the best of both worlds. We can take advantage
of pre-consensus data distribution but not be beholden to the inefficiencies of it.

We include the chunk metadata here so that nodes hearing about the block for the first
time can post a slashable offense if they received a conflicting chunk at that time (without
having to fetch the OriginalChunkID -> signature must be over chunkID for this to work).
```
type Block struct {
    Timestamp: uint64,
    ParentBlock: ids.ID,
    ParentRoot: ids.ID,
    Height: uint64,
    Chunks: [
        <OriginalChunkID, ChunkSigner, ChunkSignature, ChunkTimestamp, SubnetSignatureBitSet, SubnetSignature, FilteredChunkID, WarpBitSet>,
        ...,
    ],
}
```

If we are want a filtered chunk (syncing validator or any non-validator), we can request it:
```
type FilteredChunkRequest struct {
    Chunk: ids.ID,
}
-> Chunk
```

Each validator can store Y processing chunks on the network for potential inclusion. If a chunk contains no valid transacitons,
it can be included in a block as a "delete" so that the validator doesn't need to wait the entire timeout for inclusion (and doesn't need
to waste the work of iterating over it during building).

Insight: Filter chunks we agreed on data availability of because they will undoubtedly contain unexecutable transactions (fee exceeds max, user
runs out of funds). Store OriginalChunkIDs until chain time surpasses latest tx or X blocks after it was accepted. Then we only store FilteredChunkID.
New nodes syncing only need to fetch FilteredChunkID. This differs slightly from Narwhal/Tusk/Bullshark, which try to use the chunks directly in consensus.

Other Benefit: at scale, chunk broadcaster and chunk receiver can be decoupled from the node (similar to Narwhal/Tusk).

## Max Throughput Estimates
### Parameters
Validators = 2000
Average Tx Size = 400B
Max Chunk Size = 2MB
Max Warp Messages Per Chunk = 64

### Max Calculations
Max Size Per Block Chunk = <32, 48, 96, 8, 2000/8, 96, 32, 64/8> = 570B
Max Chunks Per Block = 2MB/570B = 3508 Chunks
Max Txs Per Block = 3508 * 2MB/400B = 17.5M
Max Data Bandwidth Finalized Per Block = 4785 * 2MB = 7GB (56 Gb)

### 300k Tx/Block Calculations
Data Bandwidth Required = 120MB (0.96 Gb)
Block Size (tightly packed chunks) = 570B * 60 = 34KiB
Block Size (20% executable/full) = 570B * 300 = 171KiB (80% savings on long-term block data storage)

## Open Questions
* To minimize duplicate txs that can be issued by a single address, we require that addresses be sent (from non-validators) over P2P
to a specific issuer for a specific expiry time. May want to remove non-validator -> validator P2P gossip entirely?
-> Validators that don't want to distribute any of their own transactions end up having a very low outbound traffic requirement.
