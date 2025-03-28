// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Snow provides an adapter from the AvalancheGo [block.ChainVM] interface (the interface AvalancheGo VM developers must implement)
// to provide a more ergonomic interface for VM developers.
//
// The existing interface was largely defined for ease of use by the AvalancheGo consensus engine and as a single all access type for
// anything required by AvalancheGo (APIs, health check, shutdown, etc.)
//
// # What does the original interface require that this package simplifies?
//
//  1. Maintaining a [Processing Block Tree]
//  2. Vacuous block verification and acceptance during [Dynamic State Sync]
//  3. [common.VM] interface that we can replace with good, customizable defaults
//
// This package implements these tough to get right and tedious components and provides
// a slimmed down interface for VM developers to implement.
//
// # Using Snow
//
// Snow defines a concrete block type for each distinct state a block can be in
// within consensus:
//
//   - []byte: raw bytes of the block
//   - [Input]: un-verified block
//   - [Output]: verified block
//   - [Accepted]: accepted block
//
// This provides a customizable block representation and ensures that VM developers
// have access to the exact set of information guaranteed to be available in each
// state of block handling.
//
// For example, when a block is verified, its parent is guaranteed to have been verified successfully. However, it may not be accepted. Therefore,
// chain developers implement `VerifyBlock` and receive the verified version of the parent (but not accepted type) and the input block to be verified:
//
//	VerifyBlock(
//		ctx context.Context,
//		parent O,
//		block I,
//	) (O, error)
//
// See [Chain] for complete documentation of the interface.
//
// Further, the package provides two functions `chain.Initialize` and `chain.SetConsensusIndex` as the effective "entrypoint" of the VM.
// This needs to be split into two parameters, because the consensus index depends on a fully initialized version of the chain.
// This provides all of the information available to the VM at the time of initialization. See `Chain.Initialize` and
// `Chain.SetConsensusIndex` for details on the parameters provided to the VM at initialization.
//
// [block.ChainVM]: https://github.com/ava-labs/avalanchego/blob/a895cc6bb06e74b83f8b22ccd294443219aaddeb/snow/engine/snowman/block/vm.go#L28
// [Processing Block Tree]: https://github.com/ava-labs/avalanchego/tree/a353b45944904ce56b5eddf961bf83430892dc32/vms#implementing-the-snowman-vm-block
// [Dynamic State Sync]: https://github.com/ava-labs/avalanchego/blob/a353b45944904ce56b5eddf961bf83430892dc32/snow/engine/snowman/block/README.md#state-sync-engine-role-and-workflow
// [common.VM]: https://github.com/ava-labs/avalanchego/blob/a353b45944904ce56b5eddf961bf83430892dc32/snow/engine/common/vm.go#L17
//
// [snowman.Block]: https://github.com/ava-labs/avalanchego/blob/a895cc6bb06e74b83f8b22ccd294443219aaddeb/snow/consensus/snowman/block.go#L24
package snow
