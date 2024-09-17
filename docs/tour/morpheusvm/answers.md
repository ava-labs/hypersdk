# Answers

## `Execute()`

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
	if string(t.Memo) == "donate" {
		if err := storage.SubBalance(ctx, mu, actor, t.Value); err != nil {
			return nil, err
		}
		if err := storage.AddBalance(ctx, mu, codec.EmptyAddress, t.Value, true); err != nil {
			return nil, err
		}
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

## State Keys

```golang
func (t *Transfer) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.BalanceKey(actor)): state.Read | state.Write,
		string(storage.BalanceKey(t.To)):  state.All,
		string(storage.BalanceKey(codec.EmptyAddress)): state.All,
	}
}

func (*Transfer) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.BalanceChunks, storage.BalanceChunks, storage.BalanceChunks}
}
```
