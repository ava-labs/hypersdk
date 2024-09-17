# The Execute() Function

We can consider `Execute()` to be the most important method of any action as
it's responsible for the state-transitions of the action. For some context,
here's the interface for `Execute()`:

```golang
Execute(
    ctx context.Context,
    r Rules,
    mu state.Mutable,
    timestamp int64,
    actor codec.Address,
    actionID ids.ID,
) (outputs [][]byte, err error)
```

Going through the parameters specified in `Execute()`, we have:

- `r`: specifies the "rules" of the chain. Alternatively, `r` can be used to
  derive information about the chain (e.g. getting the network ID, getting the
  maximum number of transactions, etc.)
- `mu`: a reference to the state of our VM
- `timestamp`: the time at which the action was executed
- `actor`: the sender of the action
- `actionID`: the unique identifier associated with the action

The `Execute()` method also has a 2-D byte array output to return any values
along with returning an error.

Focusing back to the parameters provided to `Execute()`, the one we'll focus on is
`mu`. Any state transitions that a action executes will be by modifying `mu`. 

Although implementing `Execute()` can seem to be an ambiguous task, any instance
of `Execute()` will generally have the following structure:

- Check that any invariants are not violated
- Manipulate storage as necessary

To reinforce our understanding of `Execute()`, let's reimplement `Execute()`
from the `Transfer` action in MorpheusVM:

## Implementing `Execute()`

As we mentioned before, we can implement `Execute()` in two phases:

- Checking Invariants
- Manipulating Storage

### Checking Invariants

In `Execute()`, we first want to check the following:

- The value of the `Transfer` action is not zero
  - If so, return an `ErrOutputValueZero` error
- The length of the `Transfer` memo is not larger than `MaxMemoSize`
  - If so, return `ErrOutputMemoTooLarge`

### Manipulating Storage

Assuming that our invariants are now being enforced, we now focus on the state
transitions of `Execute()`. In particular, we want to do the following:

- Decrease the balance of `actor` by `value`
- Increase the balance of `to` by `value`

In both cases, we can refer to the existing functions `AddBalance()` and
`SubBalance()` in `storage.go` (and returning any errors that `AddBalance()` and
`SubBalance()` may produce).

At the end of `Execute()`, you can return `nil` values for both the output and
error.

## Conclusion

In this section, we've gone over in-depth the concept of the `Execute()` method
while writing the `Execute()` method for `Transfer` from scratch. To check if
you did things correct, simply cross-reference with `transfer.go` in
`examples/morpheusvm`. 

In the next section, we'll explore the concept of state-keys.


