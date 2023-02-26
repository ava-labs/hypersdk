package vm

import (
	"context"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/version"
)

type request struct {
	handler   uint8
	requestID uint32
}

type NetworkManager struct {
	sender common.AppSender
	l      sync.Mutex

	handler  uint8
	handlers map[uint8]NetworkHandler

	requestID     uint32
	requestMapper map[uint32]*request
}

func NewNetworkManager(sender common.AppSender) *NetworkManager {
	return &NetworkManager{
		sender:        sender,
		handlers:      map[uint8]NetworkHandler{},
		requestMapper: map[uint32]*request{},
	}
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
	n.l.Lock()
	defer n.l.Unlock()

	newHandler := n.handler
	n.handlers[newHandler] = h
	n.handler++
	return &WrappedAppSender{n, newHandler}
}

func (n *NetworkManager) getSharedRequestID(handler uint8, requestID uint32) uint32 {
	n.l.Lock()
	defer n.l.Unlock()

	newID := n.requestID
	n.requestMapper[newID] = &request{handler, requestID}
	n.requestID++
	return newID
}

func (n *NetworkManager) handleSharedRequestID(requestID uint32) *request {
	n.l.Lock()
	defer n.l.Unlock()

	req := n.requestMapper[requestID]
	delete(n.requestMapper, requestID)
	return req
}

// WrappedAppSender is used to get a shared requestID and to prepend messages
// with the handler identifier.
type WrappedAppSender struct {
	n       *NetworkManager
	handler uint8
}

// Send an application-level request.
// A nil return value guarantees that for each nodeID in [nodeIDs],
// the VM corresponding to this AppSender eventually receives either:
// * An AppResponse from nodeID with ID [requestID]
// * An AppRequestFailed from nodeID with ID [requestID]
// Exactly one of the above messages will eventually be received per nodeID.
// A non-nil error should be considered fatal.
func (w *WrappedAppSender) SendAppRequest(
	ctx context.Context,
	nodeIDs set.Set[ids.NodeID],
	requestID uint32,
	appRequestBytes []byte,
) error {
	newRequestID := w.n.getSharedRequestID(w.handler, requestID)
	return w.n.sender.SendAppRequest(
		ctx,
		nodeIDs,
		newRequestID,
		append([]byte{w.handler}, appRequestBytes...),
	)
}

// Send an application-level response to a request.
// This response must be in response to an AppRequest that the VM corresponding
// to this AppSender received from [nodeID] with ID [requestID].
// A non-nil error should be considered fatal.
func (w *WrappedAppSender) SendAppResponse(
	ctx context.Context,
	nodeID ids.NodeID,
	requestID uint32,
	appResponseBytes []byte,
) error {
	// We don't need to wrap this response because the sender should know what
	// requestID is associated with which handler.
	return w.n.sender.SendAppResponse(
		ctx,
		nodeID,
		requestID,
		appResponseBytes,
	)
}

// Gossip an application-level message.
// A non-nil error should be considered fatal.
func (w *WrappedAppSender) SendAppGossip(ctx context.Context, appGossipBytes []byte) error {
	return w.n.sender.SendAppGossip(
		ctx,
		append([]byte{w.handler}, appGossipBytes...),
	)
}
func (w *WrappedAppSender) SendAppGossipSpecific(
	ctx context.Context,
	nodeIDs set.Set[ids.NodeID],
	appGossipBytes []byte,
) error {
	return w.n.sender.SendAppGossipSpecific(
		ctx,
		nodeIDs,
		append([]byte{w.handler}, appGossipBytes...),
	)
}

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
func (w *WrappedAppSender) SendCrossChainAppRequest(
	ctx context.Context,
	chainID ids.ID,
	requestID uint32,
	appRequestBytes []byte,
) error {
	return nil
}

// SendCrossChainAppResponse sends an application-level response to a
// specific chain
//
// This response must be in response to a CrossChainAppRequest that the VM
// corresponding to this CrossChainAppSender received from [chainID] with ID
// [requestID].
// A non-nil error should be considered fatal.
func (w *WrappedAppSender) SendCrossChainAppResponse(
	ctx context.Context,
	chainID ids.ID,
	requestID uint32,
	appResponseBytes []byte,
) error {
	return nil
}
