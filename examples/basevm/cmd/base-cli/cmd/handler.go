package cmd

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/cli"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/examples/basevm/auth"
	"github.com/ava-labs/hypersdk/examples/basevm/consts"
	brpc "github.com/ava-labs/hypersdk/examples/basevm/rpc"
	"github.com/ava-labs/hypersdk/examples/basevm/utils"
	"github.com/ava-labs/hypersdk/rpc"
)

var _ cli.Controller = (*Controller)(nil)

type Handler struct {
	h *cli.Handler
}

func NewHandler(h *cli.Handler) *Handler {
	return &Handler{h}
}

func (h *Handler) Root() *cli.Handler {
	return h.h
}

func (h *Handler) DefaultActor() (
	ids.ID, crypto.PrivateKey, *auth.ED25519Factory,
	*rpc.JSONRPCClient, *brpc.JSONRPCClient, error,
) {
	priv, err := h.h.GetDefaultKey()
	if err != nil {
		return ids.Empty, crypto.EmptyPrivateKey, nil, nil, nil, err
	}
	chainID, uris, err := h.h.GetDefaultChain()
	if err != nil {
		return ids.Empty, crypto.EmptyPrivateKey, nil, nil, nil, err
	}
	cli := rpc.NewJSONRPCClient(uris[0])
	networkID, _, _, err := cli.Network(context.TODO())
	if err != nil {
		return ids.Empty, crypto.EmptyPrivateKey, nil, nil, nil, err
	}
	// For [defaultActor], we always send requests to the first returned URI.
	return chainID, priv, auth.NewED25519Factory(
			priv,
		), cli,
		brpc.NewJSONRPCClient(
			uris[0],
			networkID,
			chainID,
		), nil
}

type Controller struct {
	databasePath string
}

func NewController(databasePath string) *Controller {
	return &Controller{databasePath}
}

func (c *Controller) DatabasePath() string {
	return c.databasePath
}

func (*Controller) Symbol() string {
	return consts.Symbol
}

func (*Controller) Address(pk crypto.PublicKey) string {
	return utils.Address(pk)
}

func (*Controller) ParseAddress(address string) (crypto.PublicKey, error) {
	return utils.ParseAddress(address)
}
