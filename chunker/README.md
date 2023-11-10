```
type Chunk struct {
    Height: uint64,
    Transactions: [
        <Transaction>,
        ...,
    ],
}
```

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

Filter chunks we agreed on data availability of because they will undoubtedly contain unexecutable transactions (fee exceeds max, user
runs out of funds).

To minimize duplicate txs that can be issued by a single address, we require that addresses be sent over P2P to a specific issuer
at a specific time.
