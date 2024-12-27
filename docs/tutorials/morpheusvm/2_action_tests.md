# Action Tests

In the previous section, we implemented our first action: `Transfer`. In this 
section, we'll implement a series of action tests that aim to maximize
testing coverage of `Transfer`.

## Structure of Action Tests

Action tests are defined as follows:

```go
type ActionTest struct {
	Name string

	Action chain.Action

	Rules     chain.Rules
	State     state.Mutable
	Timestamp int64
	Actor     codec.Address
	ActionID  ids.ID

	ExpectedOutputs codec.Typed
	ExpectedErr     error

	Assertion func(context.Context, *testing.T, state.Mutable)
}
```

Going over the fields of `ActionTest`:

- `Name`: the name of the test
- `Action`: the action being tested
- `Rules`: the parameters of the chain at the time of execution
- `State`: the underlying storage at the time of execution
- `Timestamp`: the time of execution
- `Actor`: the account executing the action
- `ActionID`: a unique identifier associated with the action (similar to TX ID)
- `ExpectedOutputs`: the output that executing `Action` should produce
- `ExpectedErr`: the error that executing `Action` should produce
- `Assertion`: a lambda function that the user can pass in to mainly check for
  expected state transitions

From a high level, we as VM developers are responsible for setting up the test
environment along with providing the expected outcomes that executing `Action`
should result in. The HyperSDK, in return, will execute the test and perform any
necessary checks.

## Running an Action Test

With action tests now defined, the next biggest question might be: how do we run
an action test? To do this, just call the `Run()` method!

```go
actionTest.Run(ctx, t)
```

where `ctx` is a `Context` instance and `t` is an instance of `*testing.T`. When
running an action test, the following happens in order:

- `Execute()` is called on the action. 
- `actionTest` checks that the error of `Execute()` matches `ExpectedErr`
- `actionTest` checks that the output of `Execute()` matches `ExpectedOutput`
- If an assertion function was defined, then `actionTest` runs the function

## Action Test Example

Let's write an action test that tests that an instance of `Transfer` with a
value of 0 fails. In this case, we want to focus on the following fields:

- `Actor`: since the actor sending the `Transfer` action here doesn't matter, we
  can just use `codec.EmptyAddress`.
- `Action`: we want to test the following action:
  ```go
    &Transfer{
        To:    codec.EmptyAddress,
        Value: 0,
    },
  ```
- `ExpectedErr`: we want execution of `Action` to produce a `ErrOutputValueZero` error

With the above in mind, we can produce the following action test:

```go
{
    Name:  "ZeroTransfer",
    Actor: codec.EmptyAddress,
    Action: &Transfer{
        To:    codec.EmptyAddress,
        Value: 0,
    },
    ExpectedErr: ErrOutputValueZero,
}
```

Simple, right? Now that we have a basic idea of action tests, we can write of
the rest of the action tests for `Transfer`!

## `Transfer` Action Tests

Let's start by reusing the action test we wrote in the last section.
Create a new file named `transfer_test.go` in the `action` folder. Next,
copy-paste the following code in:

```go
// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/codec/codectest"
	"github.com/ava-labs/hypersdk/examples/tutorial/storage"
	"github.com/ava-labs/hypersdk/state"
)

func TestTransferAction(t *testing.T) {}
```

As the name suggests, `TestTransferAction` is where we'll be implementing and
executing our action tests. To start, let's create an address that we can use
for testing purposes:

```go
func TestTransferAction(t *testing.T) {
	addr := codectest.NewRandomAddress()
}
```

Next, since we'll want to have multiple tests, let's implement a single table
test: 

```go
func TestTransferAction(t *testing.T) {
	addr := codectest.NewRandomAddress()

	tests := []chaintest.ActionTest{
		{
			Name:  "ZeroTransfer",
			Actor: codec.EmptyAddress,
			Action: &Transfer{
				To:    codec.EmptyAddress,
				Value: 0,
			},
			ExpectedErr: ErrOutputValueZero,
		},
    }
}
```

Finally, we can use a `for` loop to execute our action tests:

```go
func TestTransferAction(t *testing.T) {
	addr := codectest.NewRandomAddress()

	tests := []chaintest.ActionTest{
		{
			Name:  "ZeroTransfer",
			Actor: codec.EmptyAddress,
			Action: &Transfer{
				To:    codec.EmptyAddress,
				Value: 0,
			},
			ExpectedErr: ErrOutputValueZero,
		},
    }

    for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}
```

All that's left is to add the missing action tests. Let's go over the rest of
the action tests that we'll add:

- `Transfer` should error if `actor` doesn't exist
  ```go
    {
        Name:  "NonExistentAddress",
        Actor: codec.EmptyAddress,
        Action: &Transfer{
            To:    codec.EmptyAddress,
            Value: 1,
        },
        State:       chaintest.NewInMemoryStore(),
        ExpectedErr: storage.ErrInvalidBalance,
    },
  ```
- `Transfer` should error if `actor` doesn't have enough funds
  ```go
  {
        Name:  "NotEnoughBalance",
        Actor: codec.EmptyAddress,
        Action: &Transfer{
            To:    codec.EmptyAddress,
            Value: 1,
        },
        State: func() state.Mutable {
            s := chaintest.NewInMemoryStore()
            _, err := storage.AddBalance(
                context.Background(),
                s,
                codec.EmptyAddress,
                0,
            )
            require.NoError(t, err)
            return s
        }(),
        ExpectedErr: storage.ErrInvalidBalance,
    },
    ```
- `Actor` should be able to send an amount of `1` to itself
  ```go
  {
        Name:  "SelfTransfer",
        Actor: codec.EmptyAddress,
        Action: &Transfer{
            To:    codec.EmptyAddress,
            Value: 1,
        },
        State: func() state.Mutable {
            store := chaintest.NewInMemoryStore()
            require.NoError(t, storage.SetBalance(context.Background(), store, codec.EmptyAddress, 1))
            return store
        }(),
        Assertion: func(ctx context.Context, t *testing.T, store state.Mutable) {
            balance, err := storage.GetBalance(ctx, store, codec.EmptyAddress)
            require.NoError(t, err)
            require.Equal(t, balance, uint64(1))
        },
        ExpectedOutputs: &TransferResult{
            SenderBalance:   0,
            ReceiverBalance: 1,
        },
    },
  ```
- `Transfer` should error if there's an integer overflow
  ```go
    {
        Name:  "OverflowBalance",
        Actor: codec.EmptyAddress,
        Action: &Transfer{
            To:    codec.EmptyAddress,
            Value: math.MaxUint64,
        },
        State: func() state.Mutable {
            store := chaintest.NewInMemoryStore()
            require.NoError(t, storage.SetBalance(context.Background(), store, codec.EmptyAddress, 1))
            return store
        }(),
        ExpectedErr: storage.ErrInvalidBalance,
    },
  ```
- A basic instance of `Transfer`:
  ```go
    {
        Name:  "SimpleTransfer",
        Actor: codec.EmptyAddress,
        Action: &Transfer{
            To:    addr,
            Value: 1,
        },
        State: func() state.Mutable {
            store := chaintest.NewInMemoryStore()
            require.NoError(t, storage.SetBalance(context.Background(), store, codec.EmptyAddress, 1))
            return store
        }(),
        Assertion: func(ctx context.Context, t *testing.T, store state.Mutable) {
            receiverBalance, err := storage.GetBalance(ctx, store, addr)
            require.NoError(t, err)
            require.Equal(t, receiverBalance, uint64(1))
            senderBalance, err := storage.GetBalance(ctx, store, codec.EmptyAddress)
            require.NoError(t, err)
            require.Equal(t, senderBalance, uint64(0))
        },
        ExpectedOutputs: &TransferResult{
            SenderBalance:   0,
            ReceiverBalance: 1,
        },
    },
  ```


The tests above provide great examples on how `State` and `Assertion` are used.
In particular:

- `State`: in some cases, we don't need to pass in state if `Action` errors
  before touching the state provided. In other cases, we utilize
  `chaintest.NewInMemoryStore()` and sometimes even pass in a lambda function to set up
  our state.
- `Assertion`: as previously mentioned, `Assertion` is used to check that our
  state was modified as expected. For example, consider the `Assertion` found in
  the `SelfTransfer` test:
  ```go
  Assertion: func(ctx context.Context, t *testing.T, store state.Mutable) {
          balance, err := storage.GetBalance(ctx, store, codec.EmptyAddress)
          require.NoError(t, err)
          require.Equal(t, balance, uint64(1))
      }
  ```
  In the above, we are checking that the balance of `Actor` is still 1.

Having defined the rest of our action tests, your implementation of
`TestTransferAction` should look as follows:

```go
func TestTransferAction(t *testing.T) {
	addr := codectest.NewRandomAddress()

	tests := []chaintest.ActionTest{
		{
			Name:  "ZeroTransfer",
			Actor: codec.EmptyAddress,
			Action: &Transfer{
				To:    codec.EmptyAddress,
				Value: 0,
			},
			ExpectedErr: ErrOutputValueZero,
		},
		{
			Name:  "NonExistentAddress",
			Actor: codec.EmptyAddress,
			Action: &Transfer{
				To:    codec.EmptyAddress,
				Value: 1,
			},
			State:       chaintest.NewInMemoryStore(),
			ExpectedErr: storage.ErrInvalidBalance,
		},
		{
			Name:  "NotEnoughBalance",
			Actor: codec.EmptyAddress,
			Action: &Transfer{
				To:    codec.EmptyAddress,
				Value: 1,
			},
			State: func() state.Mutable {
				s := chaintest.NewInMemoryStore()
				_, err := storage.AddBalance(
					context.Background(),
					s,
					codec.EmptyAddress,
					0,
				)
				require.NoError(t, err)
				return s
			}(),
			ExpectedErr: storage.ErrInvalidBalance,
		},
		{
			Name:  "SelfTransfer",
			Actor: codec.EmptyAddress,
			Action: &Transfer{
				To:    codec.EmptyAddress,
				Value: 1,
			},
			State: func() state.Mutable {
				store := chaintest.NewInMemoryStore()
				require.NoError(t, storage.SetBalance(context.Background(), store, codec.EmptyAddress, 1))
				return store
			}(),
			Assertion: func(ctx context.Context, t *testing.T, store state.Mutable) {
				balance, err := storage.GetBalance(ctx, store, codec.EmptyAddress)
				require.NoError(t, err)
				require.Equal(t, balance, uint64(1))
			},
			ExpectedOutputs: &TransferResult{
				SenderBalance:   0,
				ReceiverBalance: 1,
			},
		},
		{
			Name:  "OverflowBalance",
			Actor: codec.EmptyAddress,
			Action: &Transfer{
				To:    codec.EmptyAddress,
				Value: math.MaxUint64,
			},
			State: func() state.Mutable {
				store := chaintest.NewInMemoryStore()
				require.NoError(t, storage.SetBalance(context.Background(), store, codec.EmptyAddress, 1))
				return store
			}(),
			ExpectedErr: storage.ErrInvalidBalance,
		},
		{
			Name:  "SimpleTransfer",
			Actor: codec.EmptyAddress,
			Action: &Transfer{
				To:    addr,
				Value: 1,
			},
			State: func() state.Mutable {
				store := chaintest.NewInMemoryStore()
				require.NoError(t, storage.SetBalance(context.Background(), store, codec.EmptyAddress, 1))
				return store
			}(),
			Assertion: func(ctx context.Context, t *testing.T, store state.Mutable) {
				receiverBalance, err := storage.GetBalance(ctx, store, addr)
				require.NoError(t, err)
				require.Equal(t, receiverBalance, uint64(1))
				senderBalance, err := storage.GetBalance(ctx, store, codec.EmptyAddress)
				require.NoError(t, err)
				require.Equal(t, senderBalance, uint64(0))
			},
			ExpectedOutputs: &TransferResult{
				SenderBalance:   0,
				ReceiverBalance: 1,
			},
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}
```

To test `Transfer` against the action tests above, you can run the following:

```bash
go test ./actions/
```

If all goes well, you should see the following:

```bash
ok      github.com/ava-labs/hypersdk/examples/tutorial/actions        0.566s
```

Congratulations, you just tested an action using the action test framework!

## Conclusion

In this section, we verified that our implementation of `Transfer` was correct
by using action tests. In the next section, we'll look at extending our
implementation of MorpheusVM with options.
