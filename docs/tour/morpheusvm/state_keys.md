# State Keys

Recall that two key features of the HyperSDK are pessimistic concurrency control
and multidimensional fees. Pessimistic concurrency controls allows for two
actions which do not touch the same place in storage to execute concurrently
while multidimensional fees allows VMs to have a more granular approach to gas
prices. Both of these features, however, require us to specify the state keys that each
action will touch along with their sizing.

In this section, we'll go over how state keys are utilized in the HyperSDK and
later, challenge you to reimplement `StateKeys()` and `StateKeysMaxChunks()` in `morpheusvmtour/`.

## Database Key Structure

We first go over the concept of database keys. By definition, database keys are just a byte array. These byte arrays, however,
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

The prefix of a database key is always the first byte of the database key.

### Metadata

Following the prefix, the next set of bytes is the metadata component of the
database key. This segment should serve to give the associated value being stored a
unique place in storage. For example, consider the implementation of balance
database keys in MorpheusVM:

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

The last two bytes of a database key is used for specifying the size of the
value (denominated in chunks). In the HyperSDK, multidimensional fee pricing is
utilized and as a result, we need to be explicitly define the maximum size of our
values. Referring again to `BalanceKey()` from MorpheusVM, we see that each
account balance has a size of at most 1 chunk and therefore, we append `1` to
the end of our database key.

The chunk size at the end of a database key is a two-byte value.

## State Keys

State keys are comprised of the following two components:

- A database key
- The permissions relating to said database key

The only thing new here is the concept of permissions. In addition to specifying
the database keys that an action may touch, actions also need to specify how
they'll interact with those database keys. There are three types of permissions
in the HyperSDK:

- `Read`: the action can read to the database slot
- `Write`: the action can write to the database slot
- `Allocate`: the action can write a non-zero value to a previously zero slot

We also have the `All` permission which allows for reads, writes, and
allocations. 

When telling the HyperSDK what state keys an action will use, we use the
`state.Keys` type, which is a mapping from strings to the permission type. An
example can be found below:

```golang
state.Keys{
	string(databaseKey): state.All
}
```

## Implementing `StateKeys()` and `StateKeysMaxChunks`

Having gone over the concept of state keys, it's time to implement `StateKeys()`
and `StateKeysMaxChunks()`. Make sure to implement your code in `morpheusvmtour/actions/transfer.go`.

### `StateKeys()`

In this method, the following state keys should be defined:

- The state key of the `actor`'s balance along with read and write permissions
- The state key of `to`'s balance along with read, write, and allocate permissions.

### `StateKeysMaxChunks()`

Let's first look at the interface documentation for `StateKeysMaxChunks`:

```golang
// StateKeysMaxChunks is used to estimate the fee a transaction should pay. It includes the max
// chunks each state key could use without requiring the state keys to actually be provided
```

In layman's terms, for each state key that we specify in `StateKeys()`, it's
chunk size should be specified in `StateKeysMaxChunks` as an entry in the array.
Since the max size of an account balance has already been defined in `storage/`
we just need to populate the array. 

In this method, you will need to return an array which specifies the following:

- The chunk size of the `actor`'s balance
- The chunk size of `to`'s balance

## Conclusion

To check that your work is correct, you can again cross-reference with `morpheusvm/actions/transfer.go`.
