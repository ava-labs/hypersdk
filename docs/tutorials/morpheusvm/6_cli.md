# CLI

In the previous section, we implemented our network scripts along with the
HyperSDK-CLI. Now, it's time to interact with `TutorialVM`!

In this tutorial, we'll go over the following:

- Creating our binary
- Setting up our network
- Setting up the CLI
- Interacting with MorpheusVM via the CLI

## Binary Creation

In the `/tutorial` directory, start by running the following:

```bash
go build -o ./build/tutorial ./cmd/tutorialvm
```

This will create the VM binary for the next step.

## Network Setup

We first create our blockchain:

```bash
avalanche blockchain create tutorial
```

These are the options you should select:

- Create a Custom-VM
- When asked to enter a genesis, input `./genesis.json`
- Select that you already have a VM binary
- When asked to enter a binary, input `./build/tutorial`

If all goes well, you should see the following:

```bash
✓ Successfully created blockchain configuration
```

With our blockchain created, we'll now deploy it! To do this, run the following:

```bash
avalanche blockchain deploy tutorial --local
```

If all goes well, you'll see a very long message with the following:

```bash
Blockchain ready to use

+---------------------------------------------------------------------------------------------------------------+
|                                                    TUTORIAL                                                   |
+---------------+-----------------------------------------------------------------------------------------------+
| Name          | tutorial                                                                                      |
+---------------+-----------------------------------------------------------------------------------------------+
| VM ID         | tHnUBreJu9Vriu2SNqh7zbCVWnQTcopkVcKcFdvezWjG65NoK                                             |
+---------------+-----------------------------------------------------------------------------------------------+
| VM Version    |                                                                                               |
+---------------+--------------------------+--------------------------------------------------------------------+
| Local Network | SubnetID                 | 2DkbYSegR1iRSFpxV63Ef1QFQXahuux7aDPh6BbWNpMNHgHJiy                 |
|               +--------------------------+--------------------------------------------------------------------+
|               | Owners (Threhold=1)      | P-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p                    |
|               +--------------------------+--------------------------------------------------------------------+
|               | BlockchainID (CB58)      | BY3yRXon1VLZwhmWkS5V2BRBScWRaBqaVvhNqv5QMihyZmWDq                  |
|               +--------------------------+--------------------------------------------------------------------+
|               | BlockchainID (HEX)       | 0x17ebfd99e53b621ae5255d291de71a76885f64039fec13d8680aace76a222420 |
+---------------+--------------------------+--------------------------------------------------------------------+

+---------------------------+
|           TOKEN           |
+--------------+------------+
| Token Name   | Test Token |
+--------------+------------+
| Token Symbol | TEST       |
+--------------+------------+

+------------------------------------------------------------------------------------------------+
|                                        TUTORIAL RPC URLS                                       |
+-----------+------------------------------------------------------------------------------------+
| Localhost | http://127.0.0.1:9650/ext/bc/tutorial/rpc                                          |
|           +------------------------------------------------------------------------------------+
|           | http://127.0.0.1:9650/ext/bc/BY3yRXon1VLZwhmWkS5V2BRBScWRaBqaVvhNqv5QMihyZmWDq/rpc |
+-----------+------------------------------------------------------------------------------------+

+--------------------------------------------------------------------------+
|                                   NODES                                  |
+-------+------------------------------------------+-----------------------+
| NAME  | NODE ID                                  | LOCALHOST ENDPOINT    |
+-------+------------------------------------------+-----------------------+
| node1 | NodeID-7Xhw2mDxuDS44j42TCB6U5579esbSt3Lg | http://127.0.0.1:9650 |
+-------+------------------------------------------+-----------------------+
| node2 | NodeID-MFrZFVCXPv5iCn6M9K6XduxGTYp891xXZ | http://127.0.0.1:9652 |
+-------+------------------------------------------+-----------------------+
| node3 | NodeID-NFBbbJ4qCmNaCzeW7sxErhvWqvEQMnYcN | http://127.0.0.1:9654 |
+-------+------------------------------------------+-----------------------+
| node4 | NodeID-GWPcbFJZFfZreETSoWjPimr846mXEKCtu | http://127.0.0.1:9656 |
+-------+------------------------------------------+-----------------------+
| node5 | NodeID-P7oB2McjBGgW2NXXWVYjV8JEDFoW9xDE5 | http://127.0.0.1:9658 |
+-------+------------------------------------------+-----------------------+

+-------------------------------------------------------------+
|                      WALLET CONNECTION                      |
+-----------------+-------------------------------------------+
| Network RPC URL | http://127.0.0.1:9650/ext/bc/tutorial/rpc |
+-----------------+-------------------------------------------+
| Network Name    | tutorial                                  |
+-----------------+-------------------------------------------+
| Chain ID        |                                           |
+-----------------+-------------------------------------------+
| Token Symbol    | TEST                                      |
+-----------------+-------------------------------------------+
| Token Name      | Test Token                                |
+-----------------+-------------------------------------------+
```

We're now ready to use the HyperSDK-CLI!

## CLI Setup

We want to store the private key of our (test!) account
and the RPC endpoint. We can do this by executing the following commands:

```bash
./hypersdk-cli endpoint set --endpoint=http://localhost:9650/ext/bc/tutorial/ 
./hypersdk-cli key set --key=0x323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7
```

Your command line should look as follows:

```bash
AVL-0W5L7Y:tutorial rodrigo.villar$ ./hypersdk-cli endpoint set --endpoint=http://localhost:9650/ext/bc/tutorial/ 
Endpoint set to: http://localhost:9650/ext/bc/tutorial/
AVL-0W5L7Y:tutorial rodrigo.villar$ ./hypersdk-cli key set --key=0x323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7
✅ Key added successfully!
Address: 0x00c4cb545f748a28770042f893784ce85b107389004d6a0e0d6d7518eeae1292d9
```

We're now ready to interact with our implementation of MorpheusVM!

## Interacting with Tutorial

As a sanity test, let's first check that we can interact with our running VM by
running the following:

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
Let's now write to our VM by sending a TX via the CLI with the following action:

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
avalanche network stop
```

If all went well, you should see the following:

```bash
Network stopped successfully.
Server shutdown gracefully
```

## Conclusion

In this section of the MorpheusVM tutorial, we were able to interact with our
implementation of MorpheusVM by using the HyperSDK-CLI.
