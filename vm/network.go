package vm

import (
	"context"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/version"
	"go.uber.org/zap"
)

type request struct {
	handler   uint8
	requestID uint32
}

type NetworkManager struct {
	sender common.AppSender
	l      sync.RWMutex

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

func (n *NetworkManager) routeIncomingMessage(msg []byte) (NetworkHandler, bool) {
	n.l.RLock()
	defer n.l.RUnlock()

	if len(msg) == 0 {
		return nil, false
	}
	handlerID := msg[0]
	handler, ok := n.handlers[handlerID]
	return handler, ok
}

func (n *NetworkManager) handleSharedRequestID(requestID uint32) (NetworkHandler, uint32, bool) {
	n.l.Lock()
	defer n.l.Unlock()

	req := n.requestMapper[requestID]
	if req == nil {
		return nil, 0, false
	}
	delete(n.requestMapper, requestID)
	return n.handlers[req.handler], req.requestID, true
}

// Handles incoming "AppGossip" messages, parses them to transactions,
// and submits them to the mempool. The "AppGossip" message is sent by
// the other VM  via "common.AppSender" to receive txs and
// forward them to the other node (validator).
//
// implements "snowmanblock.ChainVM.commom.VM.AppHandler"
// assume gossip via proposervm has been activated
// ref. "avalanchego/vms/platformvm/network.AppGossip"
func (vm *VM) AppGossip(ctx context.Context, nodeID ids.NodeID, msg []byte) error {
	ctx, span := vm.tracer.Start(ctx, "VM.AppGossip")
	defer span.End()

	if !vm.isReady() {
		vm.snowCtx.Log.Warn("handle app gossip failed", zap.Error(ErrNotReady))

		// Errors returned here are considered fatal so we just return nil
		return nil
	}

	return vm.gossiper.HandleAppGossip(ctx, nodeID, msg)
}

// implements "block.ChainVM.commom.VM.AppHandler"
func (vm *VM) AppRequest(
	ctx context.Context,
	nodeID ids.NodeID,
	requestID uint32,
	deadline time.Time,
	request []byte,
) error {
	// We don't know the requestID, so we prepend a byte to the request to
	// extract a scope.
	switch {
	case len(request) == 0:
		// Dropping empty request
		vm.snowCtx.Log.Debug(
			"received empty message",
			zap.Stringer("nodeID", nodeID),
		)
	case request[0] == stateSyncMessage:
		if delay := vm.config.GetStateSyncServerDelay(); delay > 0 {
			time.Sleep(delay)
		}
		return vm.stateSyncNetworkServer.AppRequest(ctx, nodeID, requestID, deadline, request[1:])
	case request[0] == warpMessage:
		// TODO
		panic("not implemented")
	default:
		// Dropping unrecognized message
		vm.snowCtx.Log.Debug(
			"received unrecognized message",
			zap.Uint8("type", request[0]),
			zap.Stringer("nodeID", nodeID),
		)
	}
	return nil
}

// implements "block.ChainVM.commom.VM.AppHandler"
func (vm *VM) AppRequestFailed(ctx context.Context, nodeID ids.NodeID, requestID uint32) error {
	vm.requestIDMapperL.Lock()
	dest, ok := vm.requestIDMapper[requestID]
	delete(vm.requestIDMapper, requestID)
	vm.requestIDMapperL.Unlock()
	if !ok {
		vm.snowCtx.Log.Debug(
			"received unexepected request failed",
			zap.Stringer("nodeID", nodeID),
			zap.Uint32("requestID", requestID),
		)
		return nil
	}
	switch dest {
	case stateSyncMessage:
		return vm.stateSyncNetworkClient.AppRequestFailed(ctx, nodeID, requestID)
	case warpMessage:
		panic("not implemented")
	default:
		// Dropping unrecognized message
		vm.snowCtx.Log.Debug(
			"received unrecognized message",
			zap.Uint8("type", dest),
			zap.Stringer("nodeID", nodeID),
		)
	}
	return nil
}

// implements "block.ChainVM.commom.VM.AppHandler"
func (vm *VM) AppResponse(
	ctx context.Context,
	nodeID ids.NodeID,
	requestID uint32,
	response []byte,
) error {
	vm.requestIDMapperL.Lock()
	dest, ok := vm.requestIDMapper[requestID]
	delete(vm.requestIDMapper, requestID)
	vm.requestIDMapperL.Unlock()
	if !ok {
		vm.snowCtx.Log.Debug(
			"received unexepected response",
			zap.Stringer("nodeID", nodeID),
			zap.Uint32("requestID", requestID),
		)
		return nil
	}
	switch dest {
	case stateSyncMessage:
		return vm.stateSyncNetworkClient.AppResponse(ctx, nodeID, requestID, response)
	case warpMessage:
		panic("not implemented")
	default:
		// Dropping unrecognized message
		vm.snowCtx.Log.Debug(
			"received unrecognized message",
			zap.Uint8("type", dest),
			zap.Stringer("nodeID", nodeID),
		)
	}
	return nil
}

// implements "block.ChainVM.commom.VM.validators.Connector"
func (vm *VM) Connected(ctx context.Context, nodeID ids.NodeID, v *version.Application) error {
	return vm.stateSyncNetworkClient.Connected(ctx, nodeID, v)
}

// implements "block.ChainVM.commom.VM.validators.Connector"
func (vm *VM) Disconnected(ctx context.Context, nodeID ids.NodeID) error {
	return vm.stateSyncNetworkClient.Disconnected(ctx, nodeID)
}

func (n *NetworkManager) CrossChainAppRequest(
	ctx context.Context,
	id ids.ID,
	requestID uint32,
	deadline time.Time,
	msg []byte,
) error {
	handler, ok := n.routeIncomingMessage(msg)
	if !ok {
		return nil
	}
	return handler.CrossChainAppRequest(ctx, id, requestID, deadline, msg[1:])
}

func (n *NetworkManager) CrossChainAppRequestFailed(
	ctx context.Context,
	nodeID ids.ID,
	requestID uint32,
) error {
	handler, cRequestID, ok := n.handleSharedRequestID(requestID)
	if !ok {
		return nil
	}
	return handler.CrossChainAppRequestFailed(ctx, nodeID, cRequestID)
}

// CrossChainAppResponse is a no-op.
func (*VM) CrossChainAppResponse(context.Context, ids.ID, uint32, []byte) error {
	return nil
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
	return w.n.sender.SendCrossChainAppRequest(
		ctx,
		chainID,
		requestID,
		append([]byte{w.handler}, appRequestBytes...),
	)
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
	// We don't need to wrap this response because the sender should know what
	// requestID is associated with which handler.
	return w.n.sender.SendCrossChainAppResponse(ctx, chainID, requestID, appResponseBytes)
}
