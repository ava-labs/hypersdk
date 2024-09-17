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

Out of the many parameters provided to `Execute()`, the one we'll focus on is
`mu`. This is a reference to the state of our VM and highlights the main purpose
of `Execute()` - to modify our VM state so that our action takes effect.

Looking at `Execute()` from the `Transfer` action in MorpheusVM, we have the
following:

```golang
func (t *Transfer) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	actor codec.Address,
	_ ids.ID,
) ([][]byte, error) {
	if t.Value == 0 {
		return nil, ErrOutputValueZero
	}
	if len(t.Memo) > MaxMemoSize {
		return nil, ErrOutputMemoTooLarge
	}
	if err := storage.SubBalance(ctx, mu, actor, t.Value); err != nil {
		return nil, err
	}
	if err := storage.AddBalance(ctx, mu, t.To, t.Value, true); err != nil {
		return nil, err
	}
	return nil, nil
}
```

The example above is great as it makes clear the general structure that any
`Execute()` method will follow:

- Check that any invariants are not violated
  - In `Transfer`, we check that a non-zero value is being transferred and that
    the memo is not too large
- Manipulate storage as necessary
  - in `Transfer`, we decrease the `actor`'s (i.e. sender) balance and increase
    the balance of `to` by `value`
