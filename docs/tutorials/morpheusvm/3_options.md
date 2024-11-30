# Options

The biggest difference between our version of MorpheusVM and the existing
version in `examples/` is that we didn't add any options to our VM.

Options allow developers to easily extend the functionality of their VM. In this
section, we'll implement a JSON-RPC option that will have the VM create a
JSON-RPC server. Our server will allow clients to query for account balances
along with the genesis of the VM.

## Prereq: Updating `storage.go` and `consts.go`

Before we build out our JSON-RPC function, we'll first need to add the following
function to `storage/storage.go`:

```golang
type ReadState func(context.Context, [][]byte) ([][]byte, []error)

func GetBalanceFromState(
	ctx context.Context,
	f ReadState,
	addr codec.Address,
) (uint64, error) {
	k := BalanceKey(addr)
	values, errs := f(ctx, [][]byte{k})
	bal, _, err := innerGetBalance(values[0], errs[0])
	return bal, err
}
```

This function is almost identical to `getBalance()` except that we are passing
along `f` of type `ReadState` instead of `im` of type `state.Immutable`. 


## Getting Started

We'll want to create the following files in `/tutorial/vm/`:

- `option.go`
- `client.go`
- `server.go`

## Implementing The Server

In `server.go`, start by copy-pasting the following code:

```golang
package vm

import (
	"net/http"

	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/tutorial/consts"
	"github.com/ava-labs/hypersdk/examples/tutorial/storage"
	"github.com/ava-labs/hypersdk/genesis"
)

type JSONRPCServer struct {
	vm api.VM
}

func NewJSONRPCServer(vm api.VM) *JSONRPCServer {
	return &JSONRPCServer{vm: vm}
}

```

In the above, we define a `JSONRPCServer` struct which contains a reference to our VM, from
which it can read from. We now add the methods that define the functionality of
`JSONRPCServer`:

```golang
type GenesisReply struct {
	Genesis *genesis.DefaultGenesis `json:"genesis"`
}

func (j *JSONRPCServer) Genesis(_ *http.Request, _ *struct{}, reply *GenesisReply) (err error) {
	reply.Genesis = j.vm.Genesis().(*genesis.DefaultGenesis)
	return nil
}

type BalanceArgs struct {
	Address codec.Address `json:"address"`
}

type BalanceReply struct {
	Amount uint64 `json:"amount"`
}

func (j *JSONRPCServer) Balance(req *http.Request, args *BalanceArgs, reply *BalanceReply) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "Server.Balance")
	defer span.End()

	balance, err := storage.GetBalanceFromState(ctx, j.vm.ReadState, args.Address)
	if err != nil {
		return err
	}
	reply.Amount = balance
	return err
}
```

Each JSON-RPC method generally consists of the following:

- A request struct
- A reply struct
- The method itself

For exmaple, consider the `Balance()` method. We define `BalanceArgs` which
takes in an argument of the address being queried (`Address`). Then, we have
`BalanceReply` which holds the balance of the account. Finally, `Balance()`
itself produces the expected results. In this case, it first converts `Address`
into a `codec.Address` type and then calls `GetBalanceFromState`. It then puts
the result of the function call into `reply` (of type `BalanceReply`).

While we've implemented our JSON-RPC server, we still need to provide a
way for our VM to instantiate the JSON-RPC server. Therefore, we create a
`jsonRPCServerFactory` struct with a method that instantiates
`JSONRPCServer`:

```golang
const JSONRPCEndpoint = "/tutorialapi"

var _ api.HandlerFactory[api.VM] = (*jsonRPCServerFactory)(nil)

type jsonRPCServerFactory struct{}

func (jsonRPCServerFactory) New(vm api.VM) (api.Handler, error) {
	handler, err := api.NewJSONRPCHandler(consts.Name, NewJSONRPCServer(vm))
	return api.Handler{
		Path:    JSONRPCEndpoint,
		Handler: handler,
	}, err
}
```

## Implementing The Client

The JSON-RPC server we've just implemented doesn't have much utility if we can't
easily call it. We therefore implement a `JSONRPCClient` struct which will
complement `JSONRPCServer`.

In `vm/client.go`, start by defining `JSONRPCClient`:

```golang
package vm

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/tutorial/consts"
	"github.com/ava-labs/hypersdk/examples/tutorial/storage"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/requester"
	"github.com/ava-labs/hypersdk/utils"
)

const balanceCheckInterval = 500 * time.Millisecond

type JSONRPCClient struct {
	requester *requester.EndpointRequester
	g         *genesis.DefaultGenesis
}

// NewJSONRPCClient creates a new client object.
func NewJSONRPCClient(uri string) *JSONRPCClient {
	uri = strings.TrimSuffix(uri, "/")
	uri += JSONRPCEndpoint
	req := requester.New(uri, consts.Name)
	return &JSONRPCClient{req, nil}
}

```

Our `JSONRPCClient` holds a requester used to send JSON requests, along with the genesis
of the VM. We now implement the methods which we'll use to call `JSONRPCServer`:

```golang
func (cli *JSONRPCClient) Genesis(ctx context.Context) (*genesis.DefaultGenesis, error) {
	if cli.g != nil {
		return cli.g, nil
	}

	resp := new(GenesisReply)
	err := cli.requester.SendRequest(
		ctx,
		"genesis",
		nil,
		resp,
	)
	if err != nil {
		return nil, err
	}
	cli.g = resp.Genesis
	return resp.Genesis, nil
}

func (cli *JSONRPCClient) Balance(ctx context.Context, addr codec.Address) (uint64, error) {
	resp := new(BalanceReply)
	err := cli.requester.SendRequest(
		ctx,
		"balance",
		&BalanceArgs{
			Address: addr,
		},
		resp,
	)
	return resp.Amount, err
}

func (cli *JSONRPCClient) WaitForBalance(
	ctx context.Context,
	addr codec.Address,
	min uint64,
) error {
	return jsonrpc.Wait(ctx, balanceCheckInterval, func(ctx context.Context) (bool, error) {
		balance, err := cli.Balance(ctx, addr)
		if err != nil {
			return false, err
		}
		shouldExit := balance >= min
		if !shouldExit {
			utils.Outf(
				"{{yellow}}waiting for %s balance: %s{{/}}\n",
				utils.FormatBalance(min),
				addr,
			)
		}
		return shouldExit, nil
	})
}
```

Finally, while our JSON-RPC client can now call our server, our client is still
missing a parser. While not necessary, having a parser is highly recommended as
it allows for features such as marshalling/unmarshalling stateless blocks and is
necessary for integration testing.

```golang
func (cli *JSONRPCClient) Parser(ctx context.Context) (chain.Parser, error) {
	g, err := cli.Genesis(ctx)
	if err != nil {
		return nil, err
	}
	return NewParser(g), nil
}

var _ chain.Parser = (*Parser)(nil)

type Parser struct {
	genesis *genesis.DefaultGenesis
}

func (p *Parser) Rules(_ int64) chain.Rules {
	return p.genesis.Rules
}

func (*Parser) ActionCodec() *codec.TypeParser[chain.Action] {
	return ActionParser
}

func (*Parser) OutputCodec() *codec.TypeParser[codec.Typed] {
	return OutputParser
}

func (*Parser) AuthCodec() *codec.TypeParser[chain.Auth] {
	return AuthParser
}

func NewParser(genesis *genesis.DefaultGenesis) chain.Parser {
	return &Parser{genesis: genesis}
}

// Used as a lambda function for creating ExternalSubscriberServer parser
func CreateParser(genesisBytes []byte) (chain.Parser, error) {
	var genesis genesis.DefaultGenesis
	if err := json.Unmarshal(genesisBytes, &genesis); err != nil {
		return nil, err
	}
	return NewParser(&genesis), nil
}
```

## Implementing The Option

With the JSON-RPC server implemented, we can now implement the option which will
allow our VM to have JSON-RPC functionality. In `vm/option.go`, we first want to
define the following:

```golang
package vm

import "github.com/ava-labs/hypersdk/vm"

const Namespace = "controller"

type Config struct {
	Enabled bool `json:"enabled"`
}

func NewDefaultConfig() Config {
	return Config{
		Enabled: true,
	}
}
```

First, notice that each option has a unique namespace. By using namespaces,
options can define their own config within the main VM config. This leads us to
the option config of our JSON-RPC server. Our config contains only one field,
`enabled`, which specifies whether if we want to use the JSON-RPC server in our
VM or not. Finally, we implement a `NewDefaultConfig()` function which returns a
default config. We now implement the option itself:

```golang
func With() vm.Option {
	return vm.NewOption(Namespace, NewDefaultConfig(), func(v *vm.VM, config Config) error {
		if !config.Enabled {
			return nil
		}
		vm.WithVMAPIs(jsonRPCServerFactory{})(v)
		return nil
	})
}
```

Options consist of the following:

- A namespace
- A default config the option can unmarshal into
- An option function that takes in the VM along with the recieved config value

The option function is especially important, as it's what allows our VM to
eventually instantiate our JSON-RPC server. At this point, your `vm` directory
should look as follows:

```
vm
├── client.go
├── option.go
├── server.go
└── vm.go
```

## Adding Our Option To The VM

Finally, we now add the option we've just created to our VM. In `vm/vm.go`,
paste the following line within `New()` (right before the return statement):

```golang
	options = append(options, With())
```

This line of code by default appends the JSON-RPC option to the list of options
that the function caller passed in. 

## Conclusion

In this section, we've built upon our existing VM implementation by adding a
JSON-RPC server option. With options implemented, we are now ready to deploy our
VM!
