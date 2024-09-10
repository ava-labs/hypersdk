# Introduction

In this tutorial, we will go through how to build MorpheusVM, a HyperVM which
supports token transfers. Although an implementation of MorpheusVM already
exists, we will set out to build our own version from scratch. The outline of
this tutorial is as follows:

- Prerequisites
- Initializing MorpheusVM
- Creating Transfer, Pt. 1
- Implementing Storage
  - Read/write functions
  - StateManager
- Creating Transfer, Pt. 2
- Bringing Everything Together
- Options
- Workload Tests

This introduction will go over everything up to and including the section
"Bringing Everything Together".

## Prerequisites

We assume the following for this tutorial:

- You are using Go 1.21.12
- You have the HyperSDK repository on your local machine

## Initializing MorpheusVM

We now create the files necessary for the rest of the tutorial. To start, create
 a new folder in the `/examples` directory of the HyperSDK. We’ll call it
 `tutorial/`. Run `go mod init github.com/ava-labs/hypersdk/examples/tutorial`
 to create the go.mod file for your project.

We want to use our local instance of HyperSDK as a dependency. 
Therefore, specify the following in `go.mod`:

```golang
require (
   github.com/ava-labs/hypersdk v0.0.1
)

replace github.com/ava-labs/hypersdk => ../../
```

Finally, create the following files so that your tutorial directory looks like
the following:

```bash
.
├── consts.go
├── go.mod
├── state_manager.go
├── storage.go
├── transfer.go
└── vm.go
```

> [!TIP]
> If at any point of the tutorial, if you are running into dependency issues,
> running `go mod tidy` should fix any issues

Finally, we’ll define the following consts.go file:

```golang
package tutorial

import "github.com/ava-labs/avalanchego/version"

const (
   HRP      = "tutorial"
)

var Version = &version.Semantic{
   Major: 0,
   Minor: 0,
   Patch: 1,
}
```

## Creating `Transfer` Pt. 1

Let’s start by declaring the `Transfer` action itself. Your `transfer.go` file
should look like the following:

```golang
package tutorial

import (
   "github.com/ava-labs/hypersdk/codec"
)

type Transfer struct {
   To codec.Address `json:"to"`
   Value uint64 `json:"value"`
   Memo []byte `json:"memo"`
}
```

`Transfer` includes all of the arguments needed to execute (you can think of its
 fields like the arguments to a function). Let’s start by stubbing out the 
 implementation of `chain.Action`. You can copy-paste the code below:

```golang
func (t *Transfer) ComputeUnits(chain.Rules) uint64 {
   panic("unimplemented")
}

func (t *Transfer) Execute(ctx context.Context, r chain.Rules, mu state.Mutable, timestamp int64, actor codec.Address, actionID ids.ID) (outputs [][]byte, err error) {
   panic("unimplemented")
}

func (t *Transfer) GetTypeID() uint8 {
   panic("unimplemented")
}

func (t *Transfer) StateKeys(actor codec.Address, actionID ids.ID) state.Keys {
   panic("unimplemented")
}

func (t *Transfer) StateKeysMaxChunks() []uint16 {
   panic("unimplemented")
}

func (t *Transfer) ValidRange(chain.Rules) (start int64, end int64) {
   panic("unimplemented")
}
```

Implementing each method individually:

- `ComputeUnits()`: since our main goal isn’t to focus on multi-dimensional
pricing, we can return 1 here
- `GetTypeID()`: normally, we’d want to keep assign action IDs in an organized
manner. However, Transfer is the only action we’re implementing and so we can
return 1 here as well
- `ValidRange()`: we want our function to work at any time. therefore, make this
return `-1, -1`

This leaves us with `StateKeys`, `StateKeysMaxChunks`, and `Execute`. However,
we are blocked until we implement our storage functions.

## Implementing Storage

Let’s first go over what each subsubsection should accomplish:

- Read/Write Helper Functions: these will be used by our actions rather than
actions writing directly to the key-value store
- StateManager: this will allow for HyperSDK to read/write to our state in a
safe manner (i.e. charging a user for gas)

### Read/Write Functions

To start, we want to define a partitioning of our state. We want to declare
three partitions for the StateManager and one partition for storing the balances
of users:

```golang
const (
   // Active state
   balancePrefix   = 0x0
   heightPrefix    = 0x1
   timestampPrefix = 0x2
   feePrefix       = 0x3
)

var (
   heightKey    = []byte{heightPrefix}
   timestampKey = []byte{timestampPrefix}
   feeKey       = []byte{feePrefix}
)
```

With these values defined, we write getter functions for the latter three
variables:

```golang
func HeightKey() (k []byte) {
   return heightKey
}

func TimestampKey() (k []byte) {
   return timestampKey
}

func FeeKey() (k []byte) {
   return feeKey
}
```

Let’s now define balance keys. Keys start with their prefix and have a suffix
representing how large the value they hold is (denominated in 64-byte chunks).
With the address of the user being the main differentiator, we implement the
following:

```golang
const BalanceChunks uint16 = 1
// [balancePrefix] + [address]
func BalanceKey(addr codec.Address) (k []byte) {
   k = make([]byte, 1+codec.AddressLen+consts.Uint16Len)
   k[0] = balancePrefix
   copy(k[1:], addr[:])
   binary.BigEndian.PutUint16(k[1+codec.AddressLen:], BalanceChunks)
   return
}
```

Finally, let’s implement all the necessary read/write functions for our VM.
We’ll break this up by implementing write functions followed by read functions:

#### Write Functions

To start, let’s write the base function that’ll allow us to write a user’s
balance to state:

```golang
func setBalance(
   ctx context.Context,
   mu state.Mutable,
   key []byte,
   balance uint64,
) error {
   return mu.Insert(ctx, key, binary.BigEndian.AppendUint64(nil, balance))
}
```

With this base function, let’s now utilize it by writing functions that will
add/subtract a user’s balance:

```golang
func AddBalance(
   ctx context.Context,
   mu state.Mutable,
   addr codec.Address,
   amount uint64,
   create bool,
) error {
//   key, bal, exists, err := getBalance(ctx, mu, addr)
   if err != nil {
       return err
   }
   // Don't add balance if account doesn't exist. This
   // can be useful when processing fee refunds.
   if !exists && !create {
       return nil
   }
   nbal, err := smath.Add(bal, amount)
   if err != nil {
       return fmt.Errorf(
           "%w: could not add balance (bal=%d, addr=%v, amount=%d)",
           ErrInvalidBalance,
           bal,
           codec.MustAddressBech32(HRP, addr),
           amount,
       )
   }
   return setBalance(ctx, mu, key, nbal)
}

func SubBalance(
   ctx context.Context,
   mu state.Mutable,
   addr codec.Address,
   amount uint64,
) error {
//   key, bal, ok, err := getBalance(ctx, mu, addr)
   if !ok {
       return ErrInvalidAddress
   }
   if err != nil {
       return err
   }
   nbal, err := smath.Sub(bal, amount)
   if err != nil {
       return fmt.Errorf(
           "%w: could not subtract balance (bal=%d, addr=%v, amount=%d)",
           ErrInvalidBalance,
           bal,
           codec.MustAddressBech32(HRP, addr),
           amount,
       )
   }
   if nbal == 0 {
       // If there is no balance left, we should delete the record instead of
       // setting it to 0.
       return mu.Remove(ctx, key)
   }
   return setBalance(ctx, mu, key, nbal)
}
```

You might have noticed that both `AddBalance()` and `SubBalance()` have a line
of code commented out - let’s now focus on implementing the read functions so
that we can uncomment those lines out.

#### Read Function

Just like with the write functions, let’s start by defining a base function.
Instead of reading from state, however, this function will serve to interpret
whatever value is passed to it. That is, a parent function will get a user’s
balance from state and then pass on the responsibility of parsing any
values/errors to the inner function. Therefore, we have:

```golang
func innerGetBalance(
   v []byte,
   err error,
) (uint64, bool, error) {
   if errors.Is(err, database.ErrNotFound) {
       return 0, false, nil
   }
   if err != nil {
       return 0, false, err
   }
   return binary.BigEndian.Uint64(v), true, nil
}
```

Having defined our base function, we can now define the read functions necessary
for this tutorial:

```golang
func GetBalance(
   ctx context.Context,
   im state.Immutable,
   addr codec.Address,
) (uint64, error) {
   _, bal, _, err := getBalance(ctx, im, addr)
   return bal, err
}

func getBalance(
   ctx context.Context,
   im state.Immutable,
   addr codec.Address,
) ([]byte, uint64, bool, error) {
   k := BalanceKey(addr)
   bal, exists, err := innerGetBalance(im.GetValue(ctx, k))
   return k, bal, exists, err
}
```

With read/write functionality now implemented, we can move onto StateManager.

### State Manager

We start off by defining an empty struct for `StateManager`:

```golang
package storage

type StateManager struct {}
```

`StateManager` can be thought of as a proxy between HyperSDK and the state of 
MorpheusVM. Therefore, we need to implement the `chain.StateManager` interface:

```golang
package tutorial

import (
   "context"
   "github.com/ava-labs/hypersdk/chain"
   "github.com/ava-labs/hypersdk/codec"
   "github.com/ava-labs/hypersdk/state"
)

var _ chain.StateManager = (*StateManager)(nil)

type StateManager struct{}

func (s *StateManager) AddBalance(ctx context.Context, addr codec.Address, mu state.Mutable, amount uint64, createAccount bool) error {
   panic("unimplemented")
}

func (s *StateManager) CanDeduct(ctx context.Context, addr codec.Address, im state.Immutable, amount uint64) error {
   panic("unimplemented")
}

func (s *StateManager) Deduct(ctx context.Context, addr codec.Address, mu state.Mutable, amount uint64) error {
   panic("unimplemented")
}

func (s *StateManager) FeeKey() []byte {
   panic("unimplemented")
}
```

Focusing on each method:

- `FeeKey()`: defer to the function in storage
- `HeightKey()`: defer to the function in storage
- `TimestampKey()`: defer to the function in storage
- `SponsorStateKeys()`: this should return the balance key of addr along with
read/write permissions. We should have the following:

```golang
return state.Keys{
       string(BalanceKey(addr)): state.Read | state.Write,
   }
```

- `Deduct()`: defer to `SubBalance()` function in storage
- `AddBalance()`: defer to `AddBalance()` in storage
- `CanDeduct()`: this function should do the following
  - Get the user’s balance from state
  - Check if the user has enough funds
  - Return nil if the above holds, err otherwise

The implementation of `CanDeduct()` should be as follows:

```golang
func (s *StateManager) CanDeduct(ctx context.Context, addr codec.Address, im state.Immutable, amount uint64) error {
   bal, err := GetBalance(ctx, im, addr)
   if err != nil {
       return err
   }
   if bal < amount {
       return ErrInvalidBalance
   }
   return nil
}
```

## Creating `Transfer`, Pt. 2

With our state implementation complete, we can now finish implementing the
`Transfer` action. Focusing on each method individually:

### `StateKeys()`

For `Transfer`, we need to access the balances of `actor` and `To`. Furthermore, we 
need both read/write access. Therefore, we have the following:

```golang
return state.Keys{
       string(BalanceKey(actor)): state.All,
       string(BalanceKey(t.To)):  state.All,
   }
```

### `StateKeysMaxChunks()`

For each state key, we have to specify how many chunks it’ll take up in storage.
Since we’ve previously defined this value, we can implement the following:

```golang
return []uint16{BalanceChunks, BalanceChunks}
```

## `Execute()`

We start off by first defining the values necessary for this function:

```golang
const MaxMemoSize = 256

var (
   ErrOutputValueZero                 = errors.New("value is zero")
   ErrOutputMemoTooLarge              = errors.New("memo is too large")
   _                     chain.Action = (*Transfer)(nil)
)
```

We can now define `Execute()`. Recall that we should first check any invariants
and then execute the necessary state changes. Therefore, we have the following:

```golang
if t.Value == 0 {
    return nil, ErrOutputValueZero
}
if len(t.Memo) > MaxMemoSize {
    return nil, ErrOutputMemoTooLarge
}
if err := SubBalance(ctx, mu, actor, t.Value); err != nil {
    return nil, err
}
if err := AddBalance(ctx, mu, t.To, t.Value, true); err != nil {
    return nil, err
}
return nil, nil
```

## Bringing Everything Together

So far, we’ve implemented the following:

- Actions
- Storage

We can now finish this MorpheusVM tutorial by implementing the vm interface.
In `vm.go`, we first define the “registry” of MorpheusVM:

```golang
package vm

import (
   "github.com/ava-labs/avalanchego/utils/wrappers"

   "github.com/ava-labs/hypersdk/auth"
   "github.com/ava-labs/hypersdk/chain"
   "github.com/ava-labs/hypersdk/codec"
   "github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
   "github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
   "github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
   "github.com/ava-labs/hypersdk/genesis"
   "github.com/ava-labs/hypersdk/vm"
   "github.com/ava-labs/hypersdk/vm/defaultvm"
)

var (
   ActionParser *codec.TypeParser[chain.Action]
   AuthParser   *codec.TypeParser[chain.Auth]
)

// Setup types
func init() {
   ActionParser = codec.NewTypeParser[chain.Action]()
   AuthParser = codec.NewTypeParser[chain.Auth]()

   errs := &wrappers.Errs{}
   errs.Add(
       ActionParser.Register(&actions.Transfer{}, nil),

       AuthParser.Register(&auth.ED25519{}, auth.UnmarshalED25519),
       AuthParser.Register(&auth.SECP256R1{}, auth.UnmarshalSECP256R1),
       AuthParser.Register(&auth.BLS{}, auth.UnmarshalBLS),
   )
   if errs.Errored() {
       panic(errs.Err)
   }
}
```

By “registry”, we mean the `ActionParser` and `AuthParser` which tell our VM
which actions and cryptographic functions that it’ll support.

Finally, we create a `New()` function that allows for the VM we’ve worked on to
be instantiated.

```golang
// NewWithOptions returns a VM with the specified options
func New(options ...vm.Option) (*vm.VM, error) {
   return defaultvm.New(
       Version,
       genesis.DefaultGenesisFactory{},
       &StateManager{},
       ActionParser,
       AuthParser,
       auth.Engines(),
       options...,
   )
}
```

At this point, we've implemented the necessary components of MorpheusVM. In the
next section, we'll look at extending MorpheusVM with options. By adding
options, we can test the correctness of our VM by adding workload tests.
