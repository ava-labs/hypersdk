# Options

Looking closely at the `New()` function we’ve just created, the biggest
difference between our version and the canonical MorpheusVM version is that we
haven’t included any options in our code.

We’ll talk about options later on, but for right now, the main gist of options
is that they allow developers to extend the functionality of their VM. Here,
we’ll implement a JSON-RPC option that will allow users to query the state of
their VM.

To get started, create the following files in your `tutorial/` directory:

- option.go
- client.go
- server.go

We now implement each section of the JSON-RPC tutorial:

## Server

The server will act to handle JSON-RPC requests from clients. To start, let's
define the logic of the JSON-RPC server:

```golang
type JSONRPCServer struct {
 vm api.VM
}

func NewJSONRPCServer(vm api.VM) *JSONRPCServer {
 return &JSONRPCServer{vm: vm}
}

type GenesisReply struct {
 Genesis *genesis.DefaultGenesis `json:"genesis"`
}

func (j *JSONRPCServer) Genesis(_ *http.Request, _ *struct{}, reply *GenesisReply) (err error) {
 reply.Genesis = j.vm.Genesis().(*genesis.DefaultGenesis)
 return nil
}

type BalanceArgs struct {
 Address string `json:"address"`
}

type BalanceReply struct {
 Amount uint64 `json:"amount"`
}

func (j *JSONRPCServer) Balance(req *http.Request, args *BalanceArgs, reply *BalanceReply) error {
 ctx, span := j.vm.Tracer().Start(req.Context(), "Server.Balance")
 defer span.End()

 addr, err := codec.ParseAddressBech32(consts.HRP, args.Address)
 if err != nil {
  return err
 }
 balance, err := storage.GetBalanceFromState(ctx, j.vm.ReadState, addr)
 if err != nil {
  return err
 }
 reply.Amount = balance
 return err
}
```

While we've implemented a JSON-RPC server, we have one step left. The HyperSDK
requires us to wrap this JSON-RPC server into a factory so that the VM can
create the server. This requires us to implement the following:

```golang
const (
   Name          = "tutorial"
   JSONRPCEndpoint = "/tutorial"
)

var _ api.HandlerFactory[api.VM] = (*jsonRPCServerFactory)(nil)

type jsonRPCServerFactory struct{}

func (jsonRPCServerFactory) New(vm api.VM) (api.Handler, error) {
   handler, err := api.NewJSONRPCHandler(Name, NewJSONRPCServer(vm))
   return api.Handler{
       Path:    JSONRPCEndpoint,
       Handler: handler,
   }, err
}
```

With the JSON-RPC server defined, we can move towards implementing the client.

## Client

Within the client, we do not need to conform to any HyperSDK-specific interface.
Instead, we need to implement the following:

- A CLI for calling the MorpheusVM JSON-RPC Server
- A parser which can be used to marshal/unmarshal data for our VM

Focusing on the CLI, we have the following:

```golang
const balanceCheckInterval = 500 * time.Millisecond

type JSONRPCClient struct {
   requester *requester.EndpointRequester
   g         *genesis.DefaultGenesis
}

// NewJSONRPCClient creates a new client object.
func NewJSONRPCClient(uri string) *JSONRPCClient {
   uri = strings.TrimSuffix(uri, "/")
   uri += JSONRPCEndpoint
   req := requester.New(uri, Name)
   return &JSONRPCClient{req, nil}
}

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

func (cli *JSONRPCClient) Balance(ctx context.Context, addr string) (uint64, error) {
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
   addr string,
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
               utils.FormatBalance(min, 9),
               addr,
           )
       }
       return shouldExit, nil
   })
}
```

Having implemented the CLI, we now implement the parser:

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

func (*Parser) Registry() (chain.ActionRegistry, chain.AuthRegistry) {
   return ActionParser, AuthParser
}

func (*Parser) StateManager() chain.StateManager {
   return &StateManager{}
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

## Option

We can now bring everything we've built together by implementing the option
which will allow MorpheusVM to have a JSON-RPC server. 

```golang
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

## Updating `New()`

To enable the JSON-RPC optio, we can add the following at the beginning of the
`New()` function:

```golang
options = append(options, With())
```

We've created both the fundamental components of MorpheusVM along with adding a
JSON-RPC server. We now can move onto the final component of this tutorial -
adding workload tests. By adding workload tests, we can see whether if what
we've built is correct (i.e. our implementation of MorpheusVM works just like
the canonical implementation of MorpheusVM).
