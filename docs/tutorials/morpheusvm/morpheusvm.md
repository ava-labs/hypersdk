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

- You are using Go 1.21.12

To confirm, you can run:

```sh
go version
```

And you should see the ouptut (slightly different depending on your machine):

```
go version go1.21.12 darwin/arm64
```

## Cloning HyperSDK

To get started, clone the HyperSDK:

```sh
git clone git@github.com:ava-labs/hypersdk.git
cd hypersdk
```

## Initializing MorpheusVM

To start, we'll create a new directory in examples and initialize the go mod:

```sh
mkdir examples/tutorial
cd examples/tutorial
go mod init github.com/ava-labs/hypersdk/examples/tutorial
```

We now define our VM constants. We'll create a new package to store these
values:

```sh
mkdir consts
```

Inside `consts/`, we'll add a `consts.go` file to declare the name, HRP for Bech32 addresses,
and the initial version of our VM:

```golang
package consts

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/version"
)

const (
	HRP  = "tutorial"
	Name = "tutorialvm"
)

var ID ids.ID

func init() {
	b := make([]byte, ids.IDLen)
	copy(b, []byte(Name))
	vmID, err := ids.ToID(b)
	if err != nil {
		panic(err)
	}
	ID = vmID
}

var Version = &version.Semantic{
	Major: 0,
	Minor: 0,
	Patch: 1,
}

```

This will create a warning because we have not imported AvalancheGo into this workspace yet.
To fix that, let's add AvalancheGo as a dependency:

```sh
go get github.com/ava-labs/avalanchego && go mod tidy
```

## Creating `Transfer` Pt. 1

We'll implement a single `Transfer` action to handle payments in a native token.
Actions are wrapped into the HyperSDK `chain.Transaction` type and executed on-chain.

A HyperSDK transaction includes a slice of actions. When the transaction is executed, it verifies
the transaction validity and then executes each action included in the transaction sequentially. If any
action execution fails, then all of the state changes from the transaction will be reverted.

To find more details on how actions are wrapped into HyperSDK transactions, see [here](../../../chain/transaction.go).

You can think of each field in the `Transfer` type as if it's a parameter to the function
you want to execute onchain.

To start, make sure you're in `tutorial/` and execute the following:

```sh
mkdir actions
```

In `actions/`, create `transfer.go`. This file will declare the struct and add a type assertion that it
implements the `chain.Action` interface:


```golang
package actions

import (
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
)

var _ chain.Action = (*Transfer)(nil)

type Transfer struct {
	// To is the recipient of the [Value].
	To codec.Address `serialize:"true" json:"to"`

	// Amount are transferred to [To].
	Value uint64 `serialize:"true" json:"value"`

	// Optional message to accompany transaction.
	Memo []byte `serialize:"true" json:"memo"`
}
```

This imports the HyperSDK, so we'll import it and redirect it to our local version of the
HyperSDK:

```sh
go get github.com/ava-labs/hypersdk
go mod edit -replace github.com/ava-labs/hypersdk=../../
go mod tidy
```

Note: we include JSON and `serialize` tags in the struct to provide instructions on how
the `Transfer` action should be marshalled to JSON or a binary representation. The JSON tags
provide the key in the resulting JSON and `serialize:"true"` indicates that the codec should
include each field when it marshals the struct to its binary representation.

Right now, the `chain.Action` type assertion should fail because we haven't implemented the
interface yet. To fix that for now, we'll include stubs for each of the required functions that
call `panic("unimplemented")`, so we can come back to fill these in later in the tutorial.

You can copy-paste the code below:

```golang
func (t *Transfer) GetTypeID() uint8 {
	panic("unimplemented")
}

func (t *Transfer) StateKeys(actor codec.Address, actionID ids.ID) state.Keys {
	panic("unimplemented")
}

func (t *Transfer) Execute(ctx context.Context, r chain.Rules, mu state.Mutable, timestamp int64, actor codec.Address, actionID ids.ID) (output codec.Typed, err error) {
	panic("unimplemented")
}

func (t *Transfer) ComputeUnits(chain.Rules) uint64 {
	panic("unimplemented")
}

func (t *Transfer) ValidRange(chain.Rules) (start int64, end int64) {
	panic("unimplemented")
}

```

Now that we've defined our action, we'll move on to implementing the state layout for
our VM, so that we can use state helpers to update the state instead of handling
constructing and writing key-value pairs directly from our action.

## Implementing Storage

When building a VM with the HyperSDK, the VM developer defines their own state storage
layout. The HyperSDK also stores metadata in the current state, and defers to the VM where to
store that data. Note: this will be changed in the future, so that developers get a default out
of the box and can choose to override it if needed instead of needing to provide it.

With that in mind, we'll break the storage down into two components:

- Read/Write Helper Functions for VM specific state
- StateManager interface to tell the HyperSDK where to store its metadata

Before that, in `tutorial/`, create a new folder called `storage`.

### Separating the State into Partitions

To start off, we'll add single byte prefixes to separate out the state into
metadata required by the HyperSDK and an address -> balance mapping.

Let's create the file `storage.go` in `storage/` with prefixes for balance, height, timestamp, and fees:

```golang
package storage

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

### Implementing HyperSDK Metadata Handlers

Now, we'll implement functions that return the key where the HyperSDK should
store the chain height, timestamp, and fee dimensions. This is currently defined by
the VM, so that it has full control of its own state layout, but could be abstracted
away and moved into the HyperSDK in the future.

Let's add the following functions to `storage.go`:

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

Next, we'll define the `BalanceKey` function. `BalanceKey` will return the
state key that stores the provided address' balance.

The HyperSDK requires using [size-encoded storage keys](../../explanation/features.md#size-encoded-storage-keys),
which include a "chunk suffix." The "chunk suffix" is encoded as a `uint16`.
The length encoded in the "chunk suffix" sets the maximum length of any value
that can ever be placed in the corresponding key-value pair. This also means
trying to change the suffix will change the key itself. In other words, if you
write to the same key with a different suffix, it will write to a different location
instead of overwriting. This means that the suffix must provide an upper bound
for the value size forever or the VM would need to explicitly handle migrating
from the key from size A to size B.

Chunks are denominated in 64 bytes, so a chunk size of 1 means the value at that
key will never exceed 64 bytes (2 -> max of 128 bytes, 3 -> max of 192, etc).

With that in mind, we'll add `BalanceChunks` as a constant to the top of the file:

```golang
const BalanceChunks uint16 = 1
```

This means we're committing that to always store a value (the balance) in a byte
array <= 64 bytes.

Now, we can implement `BalanceKey` using the constant:

```golang
// [balancePrefix] + [address] + [BalanceChunks suffix]
func BalanceKey(addr codec.Address) (k []byte) {
	k = make([]byte, 1+codec.AddressLen+consts.Uint16Len)
	k[0] = balancePrefix
	copy(k[1:], addr[:])
	binary.BigEndian.PutUint16(k[1+codec.AddressLen:], BalanceChunks)
	return
}
```

### Balance Helper Functions

Next, we'll implement balance getter and setter functions.

Let's start by adding `setBalance` to set the balance for a pre-computed key:

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

Note: we use `key` instead of a `codec.Address` type, so that the higher level
helpers only need to compute balance keys once if they call `setBalance` more
than once.

Now, let's add `getBalance`. To re-use later, we'll start by implementing
an `innerGetBalance` function that will interpret a raw value and error:

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
	val, err := database.ParseUInt64(v)
	if err != nil {
		return 0, false, err
	}
	return val, true, nil
}
```

Having defined the function to interpret the balance value, we'll implement an
exported `GetBalance` and a private version that returns additional details:

```golang
func getBalance(
	ctx context.Context,
	im state.Immutable,
	addr codec.Address,
) ([]byte, uint64, bool, error) {
	k := BalanceKey(addr)
	bal, exists, err := innerGetBalance(im.GetValue(ctx, k))
	return k, bal, exists, err
}

func GetBalance(
	ctx context.Context,
	im state.Immutable,
	addr codec.Address,
) (uint64, error) {
	_, bal, _, err := getBalance(ctx, im, addr)
	return bal, err
}
```

### Higher Level Balance Functions

Now that we've implemented the base level storage helpers for balances,
we'll implement `AddBalance` and `SubBalance` utilizing the functions we
just wrote:

```golang
func AddBalance(
	ctx context.Context,
	mu state.Mutable,
	addr codec.Address,
	amount uint64,
	create bool,
) (uint64, error) {
	key, bal, exists, err := getBalance(ctx, mu, addr)
	if err != nil {
		return 0, err
	}
	// Don't add balance if account doesn't exist. This
	// can be useful when processing fee refunds.
	if !exists && !create {
		return 0, nil
	}
	nbal, err := smath.Add(bal, amount)
	if err != nil {
		return 0, fmt.Errorf(
			"%w: could not add balance (bal=%d, addr=%v, amount=%d)",
			ErrInvalidBalance,
			bal,
			addr,
			amount,
		)
	}
	return nbal, setBalance(ctx, mu, key, nbal)
}

func SubBalance(
	ctx context.Context,
	mu state.Mutable,
	addr codec.Address,
	amount uint64,
) (uint64, error) {
	key, bal, ok, err := getBalance(ctx, mu, addr)
	if !ok {
		return 0, ErrInvalidAddress
	}
	if err != nil {
		return 0, err
	}
	nbal, err := smath.Sub(bal, amount)
	if err != nil {
		return 0, fmt.Errorf(
			"%w: could not subtract balance (bal=%d, addr=%v, amount=%d)",
			ErrInvalidBalance,
			bal,
			addr,
			amount,
		)
	}
	if nbal == 0 {
		// If there is no balance left, we should delete the record instead of
		// setting it to 0.
		return 0, mu.Remove(ctx, key)
	}
	return nbal, setBalance(ctx, mu, key, nbal)
}
```

This will give us a few warnings for `smath`, `mconsts`, and two errors that we have
not defined yet. Let's go ahead and import the AvalancheGo safe math package
and define those two errors in another file.

First, let's add the math package from AvalancheGo to our imports:

```golang
	smath "github.com/ava-labs/avalanchego/utils/math"
```

Next, we'll add the `consts` package we defined earlier:

```golang
	mconsts "github.com/ava-labs/hypersdk/examples/tutorial/consts"
```

We now define the two errors from earlier. In `storage/`, create a file called
`errors.go` and paste the following:

```golang
package storage

import "errors"

var (
	ErrInvalidAddress = errors.New("invalid address")
	ErrInvalidBalance = errors.New("invalid balance")
)
```

Going back to `storage.go`, you should see that the errors from behind are now gone.

### State Manager

Now we'll implement the `chain.StateManager` interface to tell the HyperSDK
how to modify our VM's state when it needs to store metadata or charge fees.

Let's start off by creating a new `state_manager.go` file in `storage/` and adding a new
`StateManager` type with function stubs for each of the required functions:

```golang
package storage

import (
	"context"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
)

var _ chain.StateManager = (*StateManager)(nil)

type StateManager struct{}

func (s *StateManager) FeeKey() []byte {
	panic("unimplemented")
}

func (s *StateManager) HeightKey() []byte {
	panic("unimplemented")
}

func (s *StateManager) TimestampKey() []byte {
	panic("unimplemented")
}

func (s *StateManager) AddBalance(ctx context.Context, addr codec.Address, mu state.Mutable, amount uint64, createAccount bool) error {
	panic("unimplemented")
}

func (s *StateManager) CanDeduct(ctx context.Context, addr codec.Address, im state.Immutable, amount uint64) error {
	panic("unimplemented")
}

func (s *StateManager) Deduct(ctx context.Context, addr codec.Address, mu state.Mutable, amount uint64) error {
	panic("unimplemented")
}

func (s *StateManager) SponsorStateKeys(addr codec.Address) state.Keys {
	panic("unimplemented")
}
```

The type assertion should pass, so now we can go through and implement
each function correctly.

For each of the metadata functions, we'll simply return the state keys
we already defined when partitioning our state:

```golang
func (*StateManager) HeightKey() []byte {
	return HeightKey()
}

func (*StateManager) TimestampKey() []byte {
	return TimestampKey()
}

func (*StateManager) FeeKey() []byte {
	return FeeKey()
}
```

Now, we'll implement the balance handler functions re-using the helpers
we've already implemented:

```golang
func (*StateManager) CanDeduct(
	ctx context.Context,
	addr codec.Address,
	im state.Immutable,
	amount uint64,
) error {
	bal, err := GetBalance(ctx, im, addr)
	if err != nil {
		return err
	}
	if bal < amount {
		return ErrInvalidBalance
	}
	return nil
}

func (*StateManager) Deduct(
	ctx context.Context,
	addr codec.Address,
	mu state.Mutable,
	amount uint64,
) error {
	return SubBalance(ctx, mu, addr, amount)
}

func (*StateManager) AddBalance(
	ctx context.Context,
	addr codec.Address,
	mu state.Mutable,
	amount uint64,
	createAccount bool,
) error {
	_, err := AddBalance(ctx, mu, addr, amount, createAccount)
	return err
}
```

Finally, we need to implement `SponsorStateKeys`.

The HyperSDK uses `state.Keys` to determine what state keys may be touched during
the execution of a transaction or block. `state.Keys` includes explicit
Read/Write/Allocate permissions, so that it can determine how to safely execute
transactions in parallel.

This enables [parallel transaction execution](../../explanation/features.md#parallel-transaction-execution)
both for prefetching state and executing transactions.

`SponsorStateKeys` provides the state keys required by `CanDeduct` and `Deduct`
when the HyperSDK charges fees to the transaction sponsor (see [account abstraction](../../explanation/features.md#account-abstraction)
for more details on the `Auth` module).

We'll re-use our `BalanceKey` function and specify that both read and write
permissions, since the HyperSDK may need to both read/write this balance when
it handles fees:

```golang
func (*StateManager) SponsorStateKeys(addr codec.Address) state.Keys {
	return state.Keys{
		string(BalanceKey(addr)): state.Read | state.Write,
	}
}
```

## Creating `Transfer`, Pt. 2

Now it's time to put them into action. Going back to `transfer.go` we'll update
the stubbed out functions implementing the `chain.Action` interface with a real
implementation.

### GetTypeID

We need to include a `typeID` for our codec, so we'll implement `GetTypeID` to return
a constant `typeID`.

In `consts/`, create a new file named `types.go` and paste the following:

```golang
package consts

const (
	// Action TypeIDs
	TransferID uint8 = 0
)

```

Next, go to `actions/transfer.go` where we'll update our `GetTypeID` function stub to return the constant:

```golang
func (*Transfer) GetTypeID() uint8 {
	return mconsts.TransferID
}
```

We import the following into `transfer.go`:

```golang
    mconsts "github.com/ava-labs/hypersdk/examples/tutorial/consts"  
```

### `StateKeys()`

For `Transfer`, we need read/write access to both the `actor` address and the
`to` address (sender and receiver). Additionally, if we're transferring to
a new address, we may allocate a new account, which means we need to include
permission to allocate a new state. So we'll use `state.Read | state.Write`
for the actor and `state.All` for the `to` address:

Beforehand, we need to import the `storage` package from earlier:

```golang
	"github.com/ava-labs/hypersdk/examples/tutorial/storage"
```


```golang
func (t *Transfer) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.BalanceKey(actor)): state.Read | state.Write,
		string(storage.BalanceKey(t.To)):  state.All,
	}
}
```

### `Execute()`

We start off by first defining the values necessary for this function at the
top of the file:

```golang
const MaxMemoSize = 256

var (
	ErrOutputValueZero    = errors.New("value is zero")
	ErrOutputMemoTooLarge = errors.New("memo is too large")
)
```

We can now define `Execute()`. Recall that we should first check any invariants
and then execute the necessary state changes. Therefore, we have the following:

```golang
func (t *Transfer) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	actor codec.Address,
	_ ids.ID,
) (codec.Typed, error) {
	if t.Value == 0 {
		return nil, ErrOutputValueZero
	}
	if len(t.Memo) > MaxMemoSize {
		return nil, ErrOutputMemoTooLarge
	}
	senderBalance, err := storage.SubBalance(ctx, mu, actor, t.Value)
	if err != nil {
		return nil, err
	}
	receiverBalance, err := storage.AddBalance(ctx, mu, t.To, t.Value, true)
	if err != nil {
		return nil, err
	}

	return &TransferResult{
		SenderBalance:   senderBalance,
		ReceiverBalance: receiverBalance,
	}, nil
}
```

### ComputeUnits

The `ComputeUnits` function specifies how many units of computation are consumed
by this action. The HyperSDK uses dynamic, multi-dimensional fees and specifies its
own resource limits ie. units of compute, storage, and bandwidth. This means it's 
more important that `ComputeUnits` of different actions is proportional to the maximum,
target, and the cost of other actions than it is to be exact. So, we'll simply return 1
for `Transfer`.

Let's define a constant, `TransferComputeUnits`, at the top of the file.

```golang
const TransferComputeUnits = 1
```

Now we can implement our function:

```golang
func (*Transfer) ComputeUnits(chain.Rules) uint64 {
	return TransferComputeUnits
}
```

### ValidRange

Finally, we need to specify the range of timestamps where this action will be considered
valid. We'll use `-1` to denote that there's no bound ie. `-1, -1` indicates that the action
is valid forever.

Now, we'll update our function stub:

```golang
func (*Transfer) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}
```

## Bringing Everything Together

So far, we’ve implemented the following:

- Actions
- Storage

We can now finish this MorpheusVM tutorial by implementing the vm interface.
In `tutorial/`, create a subdirectory called `vm`. In that folder, let's create the file `vm.go`. Here, we first define the “registry” of MorpheusVM:

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

func newRegistry() (chain.Registry, error) {
	actionParser := codec.NewTypeParser[chain.Action]()
	authParser := codec.NewTypeParser[chain.Auth]()
	outputParser := codec.NewTypeParser[codec.Typed]()

	errs := &wrappers.Errs{}
	errs.Add(
		// When registering new actions, ALWAYS make sure to append at the end.
		// Pass nil as second argument if manual marshalling isn't needed (if in doubt, you probably don't)
		actionParser.Register(&actions.Transfer{}, actions.UnmarshalTransfer),

		// When registering new auth, ALWAYS make sure to append at the end.
		authParser.Register(&auth.ED25519{}, auth.UnmarshalED25519),
		authParser.Register(&auth.SECP256R1{}, auth.UnmarshalSECP256R1),
		authParser.Register(&auth.BLS{}, auth.UnmarshalBLS),

		outputParser.Register(&actions.TransferResult{}, nil),
	)
	if errs.Errored() {
		return nil, errs.Err
	}
	return vm.NewRegistry(actionParser, authParser, outputParser), nil
}
```

By “registry”, we mean the `actionParser`, `authParser` and `outputParser` which tell our VM
which actions and cryptographic functions that it’ll support.

Finally, we create a `New()` function that allows for the VM we’ve worked on to
be instantiated.

```golang
// NewWithOptions returns a VM with the specified options
func New(options ...vm.Option) (*vm.VM, error) {
	options = append(options, With()) // Add MorpheusVM API
	registry, err := newRegistry()
	if err != nil {
		return nil, err
	}
	return defaultvm.New(
		consts.Version,
		genesis.DefaultGenesisFactory{},
		&storage.StateManager{},
		registry,
		auth.Engines(),
		options...,
	)
}
```

Note: if you see an error when importing
`"github.com/ava-labs/hypersdk/vm/defaultvm"`, running `go mod tidy` should fix it

At this point, we've implemented the necessary components of MorpheusVM. It's
time to move onto the next section, where we will implement options and extend
the functionality of MorpheusVM.
