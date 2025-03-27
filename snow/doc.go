// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Snow provides an adapter from the AvalancheGo [block.ChainVM]() interface to provide a more ergonomic interface for chain developers.
// The existing interface was largely defined for ease of use by the AvalancheGo consensus engine and as a single all access type for
// anything required by AvalancheGo (APIs, health check, shutdown, etc.)
//
// What does the original interface require that this package simplifies?
// 1. Maintaining a tree of processing blocks and proposed state diffs [ref](https://github.com/ava-labs/avalanchego/tree/a353b45944904ce56b5eddf961bf83430892dc32/vms#implementing-the-snowman-vm-block)
// 2. Vacuous block verification and acceptance during [Dynamic State Sync](https://github.com/ava-labs/avalanchego/blob/a353b45944904ce56b5eddf961bf83430892dc32/snow/engine/snowman/block/README.md#state-sync-engine-role-and-workflow)
// 3. [common.VM](https://github.com/ava-labs/avalanchego/blob/a353b45944904ce56b5eddf961bf83430892dc32/snow/engine/common/vm.go#L17) interface that we can replace with good, customizable defaults
//
// This package defines a simpler to use interface for use within the HyperSDK and to eventually iterate from
// an initial small, well-defined public API backwards to more layers of customizability including building directly on top of this package.
//
// This package simplifies the existing ChainVM interface by implementing the [snowman.Block]() interface and allowing VM developers to provide
// generic type parameters for each possible state of a block within consensus (bytes, input/un-verified, output/verified, accepted).
// Developers implement a single function to transition between each of these states with all of the information guaranteed to be available
// at that point in time.
//
// For example, when a block is verified, its parent is guarnateed to have been verified already, but may still be in processing. Therefore,
// chain developers implement `VerifyBlock` as follows:
//
// ```go
//
//	VerifyBlock(
//		ctx context.Context,
//		parent O,
//		block I,
//	) (O, error)
//
// ```
// 
// See [Chain] for complete documentation of the interface.
//
// Further, the package provides two functions `Initialize` and `SetConsensusIndex` as the effective "entrypoint" of the VM.
// This needs to be split into two parameters, because the consensus index depends on a fully initialized version of the chain.
// This provides all of the information available to the VM at the time of initialization. See [Chain.Initialize] and
// [Chain.]
// This package implements the [snowman.Block] type
// To do that, this package provides an alternative interface for blockchain developers to implement that we will continue
// to iterate on, simplify, and document as we build out the HyperSDK.
//
// The existing ChainVM interface has the following quircks/gotchas that this package aims to simplify:
// 1. ChainVM / Block must pin maintain a tree of verified blocks + state diffs to be committed/abandoned on Accept/Reject (link)
// 2. Dynamic State Sync requires vacuous block verification/acceptance until state sync completes and a transition to normal operation
//
// The Snow package provides a simpler Chain interface that requires developers to implement "pure" (not exaclty pure) functions
// to transition between the different possible states of a block within consensus.
// Specifically, blocks are represented as:
// 1. unparsed bytes
// 2. an input block that has not been verified
// 3. A block that has passed verification
// 4. A block that has been accepted
//
// Therefore, the Chain interface provides requires developers implement a single function to perform each
// of the valid transitions and provide concrete types for each of the valid states ie. input, output (verified), and accepted.
// This performs the same work, but provides a more ergonomic interface for developers and allows us to test the implementation
// of that definition in isolation of concrete block types and transition functions.
// Chain implementers can instead focus on testing the logic of each of those transitions in isolation and the intialization process.
//
// The AvalancheGo VM interface has also compiled a variety of one off functions that need to be implemented by each
// VM even when they don't need to add anything. For example, the VM interface requires a HealthCheck, CreateHandlers,
// ... etc. This package implements each of those functions and exposes them to the developer via optional to use setters.
// Rather than NEEDing to implement HealthCheck, a chain developer can add a namespaced health check via AddHealthCheck
// at any point in the chain's lifecycle.
// Rather than implementing Shutdown, a chain developer can add a namespaced shutdown function via AddCloser.
//
// What else is included?
// 1. State sync
// 2. Initialize
// 3. Snow Notifications
// 4. Setters for individual fields
package snow
