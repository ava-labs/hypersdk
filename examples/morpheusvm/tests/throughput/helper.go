package throughput

import (
	"context"

	"github.com/ava-labs/hypersdk/api/ws"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	mauth "github.com/ava-labs/hypersdk/examples/morpheusvm/tests/auth"

	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/vm"
	"github.com/ava-labs/hypersdk/pubsub"
	"github.com/ava-labs/hypersdk/throughput"
	"github.com/ava-labs/hypersdk/utils"
)

type SpamHelper struct {
	KeyType string
	cli     *vm.JSONRPCClient
	ws      *ws.WebSocketClient
}

var _ throughput.SpamHelper = &SpamHelper{}


func (sh *SpamHelper) CreateAccount() (*auth.PrivateKey, error) {
	return mauth.GeneratePrivateKey(sh.KeyType)
}

func (sh *SpamHelper) CreateClient(uri string) error {
	sh.cli = vm.NewJSONRPCClient(uri)
	ws, err := ws.NewWebSocketClient(uri, ws.DefaultHandshakeTimeout, pubsub.MaxPendingMessages, pubsub.MaxReadMessageSize)
	if err != nil {
		return err
	}
	sh.ws = ws
	return nil
}

func (sh *SpamHelper) GetParser(ctx context.Context) (chain.Parser, error) {
	return sh.cli.Parser(ctx)
}

func (sh *SpamHelper) LookupBalance(choice int, address codec.Address) (uint64, error) {
	balance, err := sh.cli.Balance(context.TODO(), address)
	if err != nil {
		return 0, err
	}
	utils.Outf(
		"%d) {{cyan}}address:{{/}} %s {{cyan}}balance:{{/}} %s %s\n",
		choice,
		address,
		utils.FormatBalance(balance, consts.Decimals),
		consts.Symbol,
	)
	return balance, err
}

func (*SpamHelper) GetTransfer(address codec.Address, amount uint64, memo []byte) []chain.Action {
	return []chain.Action{&actions.Transfer{
		To:    address,
		Value: amount,
		Memo:  memo,
	}}
}
