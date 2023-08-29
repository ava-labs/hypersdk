// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package network

import (
	"context"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/version"
	"go.uber.org/zap"
)

type nodeIDRequester struct {
	requestID     uint32
	requestMapper map[uint32]*request
}

type request struct {
	handler   uint8
	requestID uint32
}

type Manager struct {
	log    logging.Logger
	nodeID ids.NodeID
	sender common.AppSender
	l      sync.RWMutex

	handler         uint8
	pendingHandlers map[uint8]struct{}
	handlers        map[uint8]Handler

	requesters map[ids.NodeID]*nodeIDRequester
}

func NewManager(log logging.Logger, nodeID ids.NodeID, sender common.AppSender) *Manager {
	return &Manager{
		log:             log,
		nodeID:          nodeID,
		sender:          sender,
		handlers:        map[uint8]Handler{},
		pendingHandlers: map[uint8]struct{}{},
		requesters:      map[ids.NodeID]*nodeIDRequester{},
	}
}

type Handler interface {
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

func (n *Manager) Register() (uint8, common.AppSender) {
	n.l.Lock()
	defer n.l.Unlock()

	newHandler := n.handler
	n.pendingHandlers[newHandler] = struct{}{}
	n.handler++
	return newHandler, &WrappedAppSender{n, newHandler}
}

// Some callers take a sender before the handler is initialized, so we need to
// set the handler after initialization to avoid a potential panic.
//
// TODO: in the future allow for queueing messages during the time between
// Register and SetHandler (should both happen in init so should not be an
// issue for standard usage)
func (n *Manager) SetHandler(handler uint8, h Handler) {
	n.l.Lock()
	defer n.l.Unlock()

	_, ok := n.pendingHandlers[handler]
	if !ok {
		n.log.Error("pending handler does not exist", zap.Uint8("id", handler))
		return
	}
	delete(n.pendingHandlers, handler)
	n.handlers[handler] = h
}

func (n *Manager) getSharedRequestID(
	handler uint8,
	nodeID ids.NodeID,
	requestID uint32,
) uint32 {
	n.l.Lock()
	defer n.l.Unlock()

	obj, ok := n.requesters[nodeID]
	if !ok {
		obj = &nodeIDRequester{
			requestMapper: map[uint32]*request{},
		}
		n.requesters[nodeID] = obj
	}
	newID := obj.requestID
	obj.requestMapper[newID] = &request{handler, requestID}
	obj.requestID++
	return newID
}

func (n *Manager) routeIncomingMessage(msg []byte) ([]byte, Handler, bool) {
	n.l.RLock()
	defer n.l.RUnlock()

	l := len(msg)
	if l == 0 {
		return nil, nil, false
	}
	handlerID := msg[0]
	handler, ok := n.handlers[handlerID]
	return msg[1:], handler, ok
}

func (n *Manager) handleSharedRequestID(
	nodeID ids.NodeID,
	requestID uint32,
) (Handler, uint32, bool) {
	n.l.Lock()
	defer n.l.Unlock()

	obj, ok := n.requesters[nodeID]
	if !ok {
		return nil, 0, false
	}
	req := obj.requestMapper[requestID]
	if req == nil {
		return nil, 0, false
	}
	delete(obj.requestMapper, requestID)
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
func (n *Manager) AppGossip(ctx context.Context, nodeID ids.NodeID, msg []byte) error {
	parsedMsg, handler, ok := n.routeIncomingMessage(msg)
	if !ok {
		n.log.Debug(
			"could not route incoming AppGossip",
			zap.Stringer("nodeID", nodeID),
		)
		return nil
	}
	return handler.AppGossip(ctx, nodeID, parsedMsg)
}

// implements "block.ChainVM.commom.VM.AppHandler"
func (n *Manager) AppRequest(
	ctx context.Context,
	nodeID ids.NodeID,
	requestID uint32,
	deadline time.Time,
	request []byte,
) error {
	parsedMsg, handler, ok := n.routeIncomingMessage(request)
	if !ok {
		n.log.Debug(
			"could not route incoming AppRequest",
			zap.Stringer("nodeID", nodeID),
			zap.Uint32("requestID", requestID),
		)
		return nil
	}
	return handler.AppRequest(ctx, nodeID, requestID, deadline, parsedMsg)
}

// implements "block.ChainVM.commom.VM.AppHandler"
func (n *Manager) AppRequestFailed(
	ctx context.Context,
	nodeID ids.NodeID,
	requestID uint32,
) error {
	handler, cRequestID, ok := n.handleSharedRequestID(nodeID, requestID)
	if !ok {
		n.log.Debug(
			"could not handle incoming AppRequestFailed",
			zap.Stringer("nodeID", nodeID),
			zap.Uint32("requestID", requestID),
		)
		return nil
	}
	return handler.AppRequestFailed(ctx, nodeID, cRequestID)
}

// implements "block.ChainVM.commom.VM.AppHandler"
func (n *Manager) AppResponse(
	ctx context.Context,
	nodeID ids.NodeID,
	requestID uint32,
	response []byte,
) error {
	handler, cRequestID, ok := n.handleSharedRequestID(nodeID, requestID)
	if !ok {
		n.log.Debug(
			"could not handle incoming AppResponse",
			zap.Stringer("nodeID", nodeID),
			zap.Uint32("requestID", requestID),
		)
		return nil
	}
	return handler.AppResponse(ctx, nodeID, cRequestID, response)
}

// implements "block.ChainVM.commom.VM.validators.Connector"
func (n *Manager) Connected(
	ctx context.Context,
	nodeID ids.NodeID,
	v *version.Application,
) error {
	n.l.RLock()
	defer n.l.RUnlock()
	for k, handler := range n.handlers {
		if err := handler.Connected(ctx, nodeID, v); err != nil {
			n.log.Debug(
				"handler could not hanlde connected message",
				zap.Stringer("nodeID", nodeID),
				zap.Uint8("handler", k),
				zap.Error(err),
			)
		}
	}
	return nil
}

// implements "block.ChainVM.commom.VM.validators.Connector"
func (n *Manager) Disconnected(ctx context.Context, nodeID ids.NodeID) error {
	n.l.RLock()
	defer n.l.RUnlock()
	for k, handler := range n.handlers {
		if err := handler.Disconnected(ctx, nodeID); err != nil {
			n.log.Debug(
				"handler could not hanlde disconnected message",
				zap.Stringer("nodeID", nodeID),
				zap.Uint8("handler", k),
				zap.Error(err),
			)
		}
	}
	return nil
}

func (n *Manager) CrossChainAppRequest(
	ctx context.Context,
	chainID ids.ID,
	requestID uint32,
	deadline time.Time,
	msg []byte,
) error {
	parsedMsg, handler, ok := n.routeIncomingMessage(msg)
	if !ok {
		n.log.Debug(
			"could not route incoming CrossChainAppRequest",
			zap.Stringer("chainID", chainID),
			zap.Uint32("requestID", requestID),
		)
		return nil
	}
	return handler.CrossChainAppRequest(ctx, chainID, requestID, deadline, parsedMsg)
}

func (n *Manager) CrossChainAppRequestFailed(
	ctx context.Context,
	chainID ids.ID,
	requestID uint32,
) error {
	handler, cRequestID, ok := n.handleSharedRequestID(n.nodeID, requestID)
	if !ok {
		n.log.Debug(
			"could not handle incoming CrossChainAppRequestFailed",
			zap.Stringer("chainID", chainID),
			zap.Uint32("requestID", requestID),
		)
		return nil
	}
	return handler.CrossChainAppRequestFailed(ctx, chainID, cRequestID)
}

func (n *Manager) CrossChainAppResponse(
	ctx context.Context,
	chainID ids.ID,
	requestID uint32,
	response []byte,
) error {
	handler, cRequestID, ok := n.handleSharedRequestID(n.nodeID, requestID)
	if !ok {
		n.log.Debug(
			"could not handle incoming CrossChainAppResponse",
			zap.Stringer("chainID", chainID),
			zap.Uint32("requestID", requestID),
		)
		return nil
	}
	return handler.CrossChainAppResponse(ctx, chainID, cRequestID, response)
}

// WrappedAppSender is used to get a shared requestID and to prepend messages
// with the handler identifier.
type WrappedAppSender struct {
	n       *Manager
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
	appRequestBytes = w.createMessageBytes(appRequestBytes)
	for nodeID := range nodeIDs {
		newRequestID := w.n.getSharedRequestID(w.handler, nodeID, requestID)
		if err := w.n.sender.SendAppRequest(
			ctx,
			set.Of(nodeID),
			newRequestID,
			appRequestBytes,
		); err != nil {
			return err
		}
	}
	return nil
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
		w.createMessageBytes(appGossipBytes),
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
		w.createMessageBytes(appGossipBytes),
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
	newRequestID := w.n.getSharedRequestID(w.handler, w.n.nodeID, requestID)
	return w.n.sender.SendCrossChainAppRequest(
		ctx,
		chainID,
		newRequestID,
		w.createMessageBytes(appRequestBytes),
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

func (w *WrappedAppSender) createMessageBytes(src []byte) []byte {
	messageBytes := make([]byte, 1+len(src))
	messageBytes[0] = w.handler
	copy(messageBytes[1:], src)
	return messageBytes
}
