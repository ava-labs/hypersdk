# State Keys

Recall that two key features of the HyperSDK are pessimistic concurrency control
and multidimensional fees. Pessimistic concurrency controls allows for two
actions which do not touch the same place in storage to execute concurrently
while multidimensional fees allows VMs to have a more granular approach to gas
prices. Both of these features, however, require us to specify the state keys that each
action will touch along with their sizing.

In this section, we'll go over how state keys are utilized in the HyperSDK and
how to go about defining state keys for your action.

## State Key Structure

By definition, state keys are just a byte array. These byte arrays, however,
have a specific structure that developers must adhere to. This structure is
comprised of three components:

- Prefix
- Metadata
- Chunk Size

Focusing on each component:

### Prefix

Prefixes are used to partition the state of our VM. Since we have complete
control over how to organize our VM, prefixes give us a way to organize our VM
by grouping together similar pieces of data. 

A great example of how prefixes are utilized is with `StateManager`. Recall that
we had to define the following:

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

These functions tell the HyperSDK which partitions of the VM state it can write
data related to the block height, block timestamp, and block fees, respectively. 

The prefix of a state key is always the first byte of the state key.

### Metadata

Following the prefix, the next set of bytes is the metadata component of the
state key. This segment should serve to give the associated value being stored a
unique place in storage. For example, consider the implementation of balance
state keys in MorpheusVM:

```golang
func BalanceKey(addr codec.Address) (k []byte) {
	k = make([]byte, 1+codec.AddressLen+consts.Uint16Len)
	k[0] = balancePrefix
	copy(k[1:], addr[:])
	binary.BigEndian.PutUint16(k[1+codec.AddressLen:], BalanceChunks)
	return
}
```

In addition to storing balances in a unique partition of storage, `BalanceKey()`
uses the address of the account for the metadata field. By doing this, we have
that two different accounts will never utilize the same space in storage. 

### Chunk Size

The last two bytes of a state key is used for specifying the size of the
value (denominated in chunks). In the HyperSDK, multidimensional fee pricing is
utilized and as a result, we need to be explicitly define the size of our
values. Referring again to `BalanceKey()` from MorpheusVM, we see that each
account balance has a size of at most 1 chunk and therefore, we append `1` to
the end of our state key.

The chunk size at the end of a state keys is a two-byte value.

## Implementing `StateKeys()`

Focusing now on the `chain.Action` interface, we now look at the `StateKeys()`
method. At a high level, `StateKeys()` tells the HyperSDK the state keys that
the action will touch during the lifetime of `Execute()`. However, reading
through the implementation, there's more to that:

```golang
func (t *Transfer) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.BalanceKey(actor)): state.Read | state.Write,
		string(storage.BalanceKey(t.To)):  state.All,
	}
}
```

Each state key is comprised of the following:

- The state key itself
- Permissions defining how the value stored at the state key can be modified

Focusing on the second point, there are three types of permissions in the
HyperSDK:

- `Read`: the action can read to the storage slot
- `Write`: the action can write to the storage slot
- `Allocate`: the action can write a non-zero value to a previously zero slot

We also have the `All` permission which allows for reads, writes, and
allocations. 
