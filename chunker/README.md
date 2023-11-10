Gossip Chunks to other validators. Desire to see transactions included
in a block is based on validator's ability to distribute. Don't include
transactions we have already seen in other chunks.
```
type Chunk struct {
    Height: uint64,
    Transactions: [
        <Transaction>,
        ...,
    ],
}
```

If we are missing a chunk from someone, we can request it (if height, it
is from a particular builder):
```
type ChunkRequest struct {
    Height: uint64,
    Chunk: ids.ID,
}
```

Only reply if all transactions are well-formatted, have valid async signatures. If not,
penalize sender.
```
type ChunkResponse struct {
    Chunk: ids.ID,
    PublicKey: BLSPublicKey,
    Signature: BLSSignature,
}
```

Pushing signatures allows anyone to include a `Chunk` in their `Block`.
```
type Signature struct {
    Chunk: ids.ID,
    Signers: BitSet,
    Signature: BLSSignature,
}
```

```
type Block struct {
    Timestamp: uint64,
    ParentBlock: ids.ID,
    ParentRoot: ids.ID,
    Height: uint64,
    Chunks: [
        <OriginalChunkID, SignatureBitSet, Signature, TxBitSet, FilteredChunkID, WarpBitSet>,
        ...,
    ],
}
```

Each validator can store Y processing chunks on the network for potential inclusion. If a chunk contains no valid transacitons,
it can be included in a block as a "delete" so that the validator doesn't need to wait the entire timeout for inclusion (and doesn't need
to waste the work of iterating over it during building).

Insight: Filter chunks we agreed on data availability of because they will undoubtedly contain unexecutable transactions (fee exceeds max, user
runs out of funds). Store OriginalChunkIDs until chain time surpasses latest tx or X blocks after it was accepted. Then we only store FilteredChunkID.
New nodes syncing only need to fetch FilteredChunkID.

## Max Throughput Estimates
### Parameters
Validators = 2000
Average Tx Size = 400B
Max Chunk Size = 2MB
Max Warp Messages Per Chunk = 64

### Calculations
Max Size Per Block Chunk = <32, 2000/8, 96, 2MB/400B/8, 32, 64/8> = 1043B
Max Chunks Per Block = 2MB/1043B = 1917 Chunks
Max Txs Per Block = 1917 * 2MB/400B = 9.5M
Max Data Bandwidth Per Block = 1917 * 2MB = 3.83GB (30.64 Gb)

## Open Questions
* To minimize duplicate txs that can be issued by a single address, we require that addresses be sent (from non-validators) over P2P
to a specific issuer for a specific expiry time.
