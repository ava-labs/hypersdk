# Async

The Async package provides the interface for async block verification.

Traditional validated consensus implies that the block builder is not trusted and peers receiving a block for the first time must verify the proposal is valid. When block verification and block building take a similar amount of time, this results in "stacking" the block building and block verification times in the critical path of consensus.

Similarly, for the next block builder to start building the subsequent proposal, it typically must verify the previous block first, which limits how effectively we can pipeline operations in the critical path.

Luckily, we can do much better in the DSMR world and more generally with "async verification." Async verification implies that we'll verify the lighest possible condition on a container (block / chunk) that implies its validity. Once blocks / chunks are selected and ordered, the entire network has already agreed on a deterministic rule to compute their result.

This enables us to select the lightest possible verification rule that implies validity and once the network has ordered the data, we're guaranteed that a correct network (deterministic execution) will compute the identical result on every node.

- we cannot 