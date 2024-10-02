# MorpheusVM

In this guide, we’ll go over MorpheusVM, the simplest VM one can build using the
HyperSDK. MorpheusVM implements single token transfer functionality only.

## Structure

MorpheusVM can be broken down into four simple packages:

- Actions - what the VM can do
- Storage - how actions read/write to state
- VM - how the VM is instantiated
- CLI - how the VM binary is created

### Actions

MorpheusVM has only one action: `Transfer`. `Transfer` sends funds from the
sender (`actor`) to the receiver (the `To` address). `Transfer` is defined below:

```golang
type Transfer struct {
   // To is the recipient of the [Value].
   To codec.Address `json:"to"`

   // Amount are transferred to [To].
   Value uint64 `json:"value"`

   // Optional message to accompany transaction.
   Memo []byte `json:"memo"`
}
```

As seen above, actions must encode any arguments they need to execute. For
example, `Transfer` requires the arguments `To`, `Value`, and `Memo`, so these
fields are added to the `Transfer` struct.

Actions implement the following interface:

```golang
ComputeUnits(Rules) uint64
StateKeys(actor codec.Address, actionID ids.ID) state.Keys
GetTypeID() uint8
ValidRange(Rules) (start int64, end int64)
Execute(
       ctx context.Context,
       r Rules,
       mu state.Mutable,
       timestamp int64,
       actor codec.Address,
       actionID ids.ID,
   ) (outputs [][]byte, err error)
```

We break this interface down by organizing functions into different categories:

#### Fees and Pessimistic Concurrency Control

`StateKeys()` provides a description of exactly how
key-value pairs in the state will be manipulated during the action’s
execution. This method returns the set of keys and the set of permissions that are
required to operate on them (ie. Read/Write/Allocate).

For example, `Transfer` implements this method as:

```golang
func (t *Transfer) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
 return state.Keys{
  string(storage.BalanceKey(actor)): state.Read | state.Write,
  string(storage.BalanceKey(t.To)):  state.All,
 }
}
```

This indicates that the actor’s balance key can be read or written, but will not
be allocated during execution. The `To` address may be read, written, or
allocated, since transferring may allocate a new account in storage.

`ComputeUnits()` returns a measure of the units of compute required to execute
the action.

These functions, along with the size of the action itself, determine the total
amount of each resource (bandwidth, compute, read, write, and allocate) consumed
by the action. The HyperSDK uses this information to calculate the fees required
by the action.

#### Action Validity

`GetTypeID()` returns a unique identifier of the action.

`ValidRange()` returns the start and stop timestamp where the action is
considered valid.

#### Action Execution

Finally, each action implements an `Execute()` function, which performs the
state transition described by the action.

This is typically structured as follows:

1. Check the required invariants
2. If the invariants hold, update the state

For MorpheusVM, `Transfer` implements this as:

1. Verify the memo field is less than the maximum length
2. Check the sender has the required balance
3. Transfer the specified value from the sender to receiver

### Storage

The storage package provides higher level helper functions to define a state
schema and interact directly with the key-value store. This package is not
strictly necessary, but helps to better organize the code, so that actions can
call higher level functions instead of directly interacting with the key-value
store and state schema.

### VM

The VM package wraps everything together and provides a VM constructor, which
attaches everything we’ve built into the MorpheusVM. This includes the Actions
we’ve built, the Auth packages to enable for the VM, and finally adds a Custom
API as a VM option.

We use the `defaultvm` package to construct our VM with the default set of
services (registered as options) provided by the HyperSDK.

#### Options

For those taking a close look at the `New()` function, one might have seen that
we are also passing in an option by default with our VM. Options are extensions
of a HyperVM which allow for additional functionality; in this case, the option
creates a JSON-RPC API which supports a single query - `Balance` - which gets the
token balance of the given account.

### CLI

The CLI package provides a run function for your VM binary by constructing your
VM and serving it over AvalancheGo’s `rpcchainvm`:

```golang
func runFunc(*cobra.Command, []string) error {
   if err := ulimit.Set(ulimit.DefaultFDLimit, logging.NoLog{}); err != nil {
       return fmt.Errorf("%w: failed to set fd limit correctly", err)
   }

   controller, err := vm.New()
   if err != nil {
       return err
   }
   return rpcchainvm.Serve(context.TODO(), controller)
}
```

This connects the VM interface over gRPC to AvalancheGo.

This package additionally provides a simple user-facing CLI for interacting with
a running instance of a MorpheusVM network.

## Conclusion

In this guide, we’ve gone over the key components of MorpheusVM - actions,
storage, VM, and the CLI. Now that you have a high level understanding of what
goes into building a VM with the HyperSDK, be on the lookout for the MorpheusVM
tutorial, which is coming soon.
