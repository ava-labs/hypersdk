package vm

import (
	"sync"

	"github.com/ava-labs/avalanchego/snow/engine/common"
)

type request struct {
	handler uint8
	requestID uint64
}

type NetworkManager struct {
	l sync.Mutex

	handler uint8
	handlers map[uint8]NetworkHandler

	requestID uint64
	requestMapper map[uint64]*request
}

type NetworkHandler interface {
	Connected(ctx context.Context, nodeID ids.NodeID, v *version.Application) error
Disconnected(ctx context.Context, nodeID ids.NodeID) error

AppGossip(ctx context.Context, nodeID ids.NodeID, msg []byte) error

AppRequest(
	ctx context.Context,
	nodeID ids.NodeID,
	requestID uint32,
	deadline time.Time,
	request []byte,
) error
AppRequestFailed(ctx context.Context, nodeID ids.NodeID, requestID uint32) error
AppResponse(
	ctx context.Context,
	nodeID ids.NodeID,
	requestID uint32,
	response []byte,
) error

CrossChainAppRequest(context.Context, ids.ID, uint32, time.Time, []byte) error
CrossChainAppRequestFailed(context.Context, ids.ID, uint32) error
CrossChainAppResponse(context.Context, ids.ID, uint32, []byte) error
}

func (n *NetworkManager) Register(h NetworkHandler) common.AppSender {
}

// TODO: need to protect against duplicate requestIDs -> will need to convert
type WrappedAppSender struct {
	// Send an application-level request.
	// A nil return value guarantees that for each nodeID in [nodeIDs],
	// the VM corresponding to this AppSender eventually receives either:
	// * An AppResponse from nodeID with ID [requestID]
	// * An AppRequestFailed from nodeID with ID [requestID]
	// Exactly one of the above messages will eventually be received per nodeID.
	// A non-nil error should be considered fatal.
	SendAppRequest(ctx context.Context, nodeIDs set.Set[ids.NodeID], requestID uint32, appRequestBytes []byte) error
	// Send an application-level response to a request.
	// This response must be in response to an AppRequest that the VM corresponding
	// to this AppSender received from [nodeID] with ID [requestID].
	// A non-nil error should be considered fatal.
	SendAppResponse(ctx context.Context, nodeID ids.NodeID, requestID uint32, appResponseBytes []byte) error
	// Gossip an application-level message.
	// A non-nil error should be considered fatal.
	SendAppGossip(ctx context.Context, appGossipBytes []byte) error
	SendAppGossipSpecific(ctx context.Context, nodeIDs set.Set[ids.NodeID], appGossipBytes []byte) error
	// SendCrossChainAppRequest sends an application-level request to a
	// specific chain.
	//
	// A nil return value guarantees that the VM corresponding to this
	// CrossChainAppSender eventually receives either:
	// * A CrossChainAppResponse from [chainID] with ID [requestID]
	// * A CrossChainAppRequestFailed from [chainID] with ID [requestID]
	// Exactly one of the above messages will eventually be received from
	// [chainID].
	// A non-nil error should be considered fatal.
	SendCrossChainAppRequest(ctx context.Context, chainID ids.ID, requestID uint32, appRequestBytes []byte) error
	// SendCrossChainAppResponse sends an application-level response to a
	// specific chain
	//
	// This response must be in response to a CrossChainAppRequest that the VM
	// corresponding to this CrossChainAppSender received from [chainID] with ID
	// [requestID].
	// A non-nil error should be considered fatal.
	SendCrossChainAppResponse(ctx context.Context, chainID ids.ID, requestID uint32, appResponseBytes []byte) error
}

// Convert requestIDs between each other and route between different handlers
type WrappedAppReceiver struct {
}
