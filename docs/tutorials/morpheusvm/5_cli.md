# CLI

In the previous sections, we have gone over how to implement MorpheusVM from
scratch and how we can test our implementation. Now that we're confident that we
have a correct version of MorpheusVM, we can now utilize the HyperSDK-CLI tool
to actually interact with our VM!

In this tutorial, we'll go over the following:

- Setting up the CLI
- Interacting with MorpheusVM via the CLI

## CLI Setup

In this section, we'll want to both start our network along with passing in the
necessary information that the HyperSDK-CLI needs to interact with our network.
To start, run the following command from `./examples/tutorial`:

```bash
./scripts/run.sh
```

If all goes well, you will eventually see the following message:

```bash
Ran 1 of 8 Specs in 19.085 seconds
SUCCESS! -- 1 Passed | 0 Failed | 0 Pending | 7 Skipped
PASS
```

This means that our network is now running in the background. Focusing now on
the HyperSDK-CLI, we now want to store the private key of our (test!) account
and the RPC endpoint. We can do this by executing the following commands:

```bash
./hypersdk-cli endpoint set --endpoint=http://localhost:9650/ext/bc/morpheusvm/ 
./hypersdk-cli key set --key=0x323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7
```

Your command line should look like the following:

```bash
AVL-0W5L7Y:tutorial rodrigo.villar$ ./hypersdk-cli endpoint set --endpoint=http://localhost:9650/ext/bc/morpheusvm/ 
Endpoint set to: http://localhost:9650/ext/bc/morpheusvm/
AVL-0W5L7Y:tutorial rodrigo.villar$ ./hypersdk-cli key set --key=0x323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7
✅ Key added successfully!
Address: 0x00c4cb545f748a28770042f893784ce85b107389004d6a0e0d6d7518eeae1292d9
```

With this, we're now ready to interact with our implementation of MorpheusVM!

## Interacting with MorpheusVM

As a sanity test, let's first check that we can interact with our running VM -
to do this, run the following command:

```bash
./hypersdk-cli ping
```

If successful, you should see the following:

```bash
✅ Ping succeeded
```

Next, let's see the current balance of our account. We'll run the following
command:

```bash
./hypersdk-cli balance
```

This should give us the following result:

```bash
✅ Balance: 10000000000000
```

Since the account we are using is specified as a prefunded account in the
genesis of our VM (via `DefaultGenesis`), our account balance is as expected.
Having read into the state of our VM, let's now try writing to our VM by sending
a transaction via the CLI. In particular, we want to send a transaction with the
following action:

- Transfer
  - Recipient: the zero address
  - Value: 12
  - Memo: "Hello World!" (in hex)

With our action specified, let's call the following command:

```bash
./hypersdk-cli tx Transfer --data to=0x000000000000000000000000000000000000000000000000000000000000000000,value=12,memo=0x48656c6c6f20576f726c6421
```

If all goes well, you should see the following:

```bash
✅ Transaction successful (txID: Cg6N7x6Z2apwMc46heJ8mFMFk2H9CEhNxiUsicrNMnDbyC3ZU)
sender_balance: 9999999969888
receiver_balance: 12
```

Congrats! You've just sent a transaction to your implementation of MorpheusVM.
To double check that your transaction did indeed go through, we can again query
the balance of our account:

```bash
./hypersdk-cli balance
✅ Balance: 9999999969888
```

However, the CLI is not just limited to just the `Transfer` action. To see what
actions you can call, you can use the following:

```bash
./hypersdk-cli actions

---
Transfer

Inputs:
  to: Address
  value: uint64
  memo: []uint8

Outputs:
  sender_balance: uint64
  receiver_balance: uint64
```

Now that we have a good idea of what we can do via the HyperSDK-CLI, it's time
to shut down our VM. To do this, we can run the following:

```bash
./scripts/stop.sh
```

If all went well, you should see the following:

```bash
Removing symlink /Users/rodrigo.villar/.tmpnet/networks/latest_morpheusvm-e2e-tests
Stopping network
```

## Conclusion

In this section of the MorpheusVM tutorial, we were able to interact with our
implementation of MorpheusVM by using the HyperSDK-CLI.
