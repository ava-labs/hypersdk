<p align="center">
  <img width="90%" alt="morpheusvm" src="assets/logo.jpeg">
</p>
<p align="center">
  The Choice is Yours
</p>

---

_[Who is Morpheus ("The Matrix")?](https://www.youtube.com/watch?v=zE7PKRjrid4)_

The [`morpheusvm`](./examples/morpheusvm) provides the first glimpse into the world of the `hypersdk`.
After learning how to implement native token transfers in a `hypervm` (one of the simplest Custom VMs
you could make), you will have the choice to go deeper (red pill) or to turn back to the VMs that you
already know (blue pill).

## Status
`morpheusvm` is considered **ALPHA** software for demonstration purposes only. It is not safe to use in production.

## Getting Started

### Building MorpheusVM from Source

#### Clone the HyperSDK

Clone the HyperSDK repository:

```sh
git clone git@github.com:ava-labs/hypersdk.git
cd hypersdk/examples/morpheusvm
```

This will clone and checkout the `main` branch.

#### Building MorpheusVM

Build MorpheusVM by running the build script:

```sh
./scripts/build.sh
```

The `morpheusvm` binary is now in the `build/./` directory.

### Run Integration Tests

To run the integration tests for MorpheusVM, run the following command:

```sh
./scripts/tests.integration.sh
```

You should see the following output:

```
[DeferCleanup (Suite)] 
/Users/user/go/src/github.com/ava-labs/hypersdk/tests/integration/integration.go:155
[DeferCleanup (Suite)] PASSED [0.000 seconds]
------------------------------

Ran 11 of 11 Specs in 1.039 seconds
SUCCESS! -- 11 Passed | 0 Failed | 0 Pending | 0 Skipped
PASS
coverage: 62.2% of statements in github.com/ava-labs/hypersdk/...
composite coverage: 60.5% of statements

Ginkgo ran 1 suite in 17.3622125s
Test Suite Passed
```


### Running a Local Network

```sh
./scripts/run.sh
```

To stop the network, run:

```sh
./scripts/stop.sh
```

The run script uses AvalancheGo's [tmpnet](https://github.com/ava-labs/avalanchego/tree/master/tests/fixture/tmpnet) to launch a 2 node network with one node's server running at the hardcoded URI: `http://127.0.0.1:9650/ext/bc/morpheusvm`.

Each default API comes with its own extension. For example, you can get the `networkID`, `subnetID`, and `chainID` by hitting the `/coreapi` extension:

```bash
curl -X POST --data '{
    "jsonrpc":"2.0",
    "id"     :1,
    "method" :"hypersdk.network",
    "params" : {}
}' -H 'content-type:application/json;' 127.0.0.1:9650/ext/bc/morpheusvm/coreapi
```

This should return the following JSON:

```json
{
  "jsonrpc": "2.0",
  "result": {
    "networkId": 88888,
    "subnetId": "21PJM6PJumoj6cueZufC5xMj3Zk6B8sALxnAhBaW85TmReU1Lq",
    "chainId": "2s9u6i9JqNrd88UqRQdYiJRJyDPjKaiduccEm2KSijjAg9VyCN"
  },
  "id": 1
}
```

_By default, this allocates all funds on the network to `morpheus1qrzvk4zlwj9zsacqgtufx7zvapd3quufqpxk5rsdd4633m4wz2fdjk97rwu`. The private
key for this address is `0x323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7`.
For convenience, this key has is also stored at `demo.pk`._


Remember that due to running a node at the default HTTP Port of AvalancheGo, you need to stop your network before restarting:

```sh
./scripts/stop.sh
```

Note: if you run into any issues starting your network, try running the following commands to troubleshoot and create a GitHub issue to report:

```bash
ps aux | grep avalanchego
```

If this prints out a number of AvalancheGo processes that are still running:

```
user    18157  18.0  1.4 413704400 466256   ??  Ss   Tue08AM 293:30.87 /Users/user/.hypersdk/avalanchego-d729e5c7ef9f008c3e89cd7131148ad3acda2e34/avalanchego --config-file /Users/user/.tmpnet/networks/20240903-083857.802339-morpheusvm-e2e-tests/NodeID-u9eMTbMcPWAj3yer1jhdheJTZL3yvC75/flags.json
user    18164  11.9  1.4 413686944 468480   ??  Ss   Tue08AM 262:35.56 /Users/user/.hypersdk/avalanchego-d729e5c7ef9f008c3e89cd7131148ad3acda2e34/avalanchego --config-file /Users/user/.tmpnet/networks/20240903-083857.802339-morpheusvm-e2e-tests/NodeID-KDrJ72L2Uvc2sgxsg22T4CanD2bTdNyqD/flags.json
user    18163   5.8  1.8 414329136 588672   ??  S    Tue08AM  59:58.94 /Users/user/.hypersdk/avalanchego-d729e5c7ef9f008c3e89cd7131148ad3acda2e34/plugins/qCNyZHrs3rZX458wPJXPJJypPf6w423A84jnfbdP2TPEmEE9u
user    18174   1.2  1.8 414329120 590544   ??  S    Tue08AM  59:20.53 /Users/user/.hypersdk/avalanchego-d729e5c7ef9f008c3e89cd7131148ad3acda2e34/plugins/qCNyZHrs3rZX458wPJXPJJypPf6w423A84jnfbdP2TPEmEE9u
user    33458   1.1  0.2 411703840  76336   ??  Ss   Fri05PM  55:42.04 /Users/user/.hypersdk/avalanchego-d729e5c7ef9f008c3e89cd7131148ad3acda2e34/avalanchego --config-file /Users/user/.tmpnet/networks/20240830-171145.374858-morpheusvm-e2e-tests/NodeID-PC24PqjXQFN81PA6a21msrhqtd6Axkvre/flags.json
user    22880   0.0  0.0 410059824     48 s001  R+    2:42PM   0:00.00 grep avalanchego
```

The following command will clean up to ensure that you can start the network:

```bash
killlall avalanchego
```

### Build `morpheus-cli`
To make it easy to interact with the `morpheusvm`, we implemented the `morpheus-cli`.

To build `morpheus-cli` run:
```sh
go build -o build/morpheus-cli cmd/morpheus-cli/*.go
```

_This command will put the compiled CLI in `./build/morpheus-cli`._

### Configure `morpheus-cli`
Next, you'll need to add the chains you created and the default key to the
`morpheus-cli`. You can use the following commands from this location to do so:
```bash
./build/morpheus-cli key import ed25519 demo.pk
```

If the key is added corretcly, you'll see the following log:
```
database: .morpheus-cli
imported address: morpheus1qrzvk4zlwj9zsacqgtufx7zvapd3quufqpxk5rsdd4633m4wz2fdjk97rwu
```

Next, you'll need to store the URL of the nodes running on your Subnet:
```sh
./build/morpheus-cli chain import
```
This will create the following prompt:

```
database: .morpheus-cli
2024/09/09 10:50:32 [JOB 1] WAL file .morpheus-cli/000038.log with log number 000038 stopped reading at offset: 0; replayed 0 keys in 0 batches
✔ uri: █
```

Input the default MorpheusVM URL for a local network:

```
http://127.0.0.1:9650/ext/bc/morpheusvm
```

You can confirm the chain was imported correctly by running:

```sh
./build/morpheus-cli chain info
```

This should output something like the following:

```
database: .morpheus-cli
2024/09/09 10:51:05 [JOB 1] WAL file .morpheus-cli/000041.log with log number 000041 stopped reading at offset: 0; replayed 0 keys in 0 batches
available chains: 1
0) chainID: JkJpw8ZPExTushPYYN4C8f7RHxjDRX8MAGGUGAdRRPEC2M3fx
select chainID: 0 [auto-selected]
networkID: 88888 subnetID: MtBf4KNMtpnZvmnWuJN5krGncZosKyqk9G339msc23CLbmBSo chainID: JkJpw8ZPExTushPYYN4C8f7RHxjDRX8MAGGUGAdRRPEC2M3fx
```

### Check Balance
To confirm you've done everything correctly up to this point, run the
following command to get the current balance of the key you added:
```bash
./build/morpheus-cli key balance
```

If successful, the balance response should look like this:
```
database: .morpheus-cli
2024/09/09 10:52:49 [JOB 1] WAL file .morpheus-cli/000044.log with log number 000044 stopped reading at offset: 0; replayed 0 keys in 0 batches
address: morpheus1qrzvk4zlwj9zsacqgtufx7zvapd3quufqpxk5rsdd4633m4wz2fdjk97rwu
chainID: JkJpw8ZPExTushPYYN4C8f7RHxjDRX8MAGGUGAdRRPEC2M3fx
uri: http://127.0.0.1:9650/ext/bc/morpheusvm
address: morpheus1qrzvk4zlwj9zsacqgtufx7zvapd3quufqpxk5rsdd4633m4wz2fdjk97rwu balance: 1000.000000000 RED
```

### Generate Another Address
Now that we have a balance to send, we need to generate another address to send to. Because
we use bech32 addresses, we can't just put a random string of characters as the recipient
(won't pass checksum test that protects users from sending to off-by-one addresses).
```bash
./build/morpheus-cli key generate secp256r1
```

If successful, the `morpheus-cli` will emit the new address:
```
database: .morpheus-cli
created address: morpheus1q8rc050907hx39vfejpawjydmwe6uujw0njx9s6skzdpp3cm2he5s036p07
```

By default, the `morpheus-cli` sets newly generated addresses to be the default. We run
the following command to set it back to `demo.pk`:
```bash
./build/morpheus-cli key set
```

You should see something like this:
```
database: .morpheus-cli
2024/09/09 10:53:51 [JOB 1] WAL file .morpheus-cli/000047.log with log number 000047 stopped reading at offset: 0; replayed 0 keys in 0 batches
chainID: JkJpw8ZPExTushPYYN4C8f7RHxjDRX8MAGGUGAdRRPEC2M3fx
stored keys: 2
0) address: morpheus1qrzvk4zlwj9zsacqgtufx7zvapd3quufqpxk5rsdd4633m4wz2fdjk97rwu balance: 10000000000.000000000 RED
1) address: morpheus1q8pyshaqzx4q9stqdt88hyg22axjwrvl0w9wgczct5fnfev9gcnrsqwjdn0 balance: 0.000000000 RED
set default key: 0
```

### Send Tokens
Lastly, we trigger the transfer:
```bash
./build/morpheus-cli action transfer
```

The `morpheus-cli` will emit the following logs when the transfer is successful:
```
database: .morpheus-cli
address: morpheus1qqds2l0ryq5hc2ddps04384zz6rfeuvn3kyvn77hp4n5sv3ahuh6wgkt57y
chainID: 2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk
balance: 1000.000000000 RED
recipient: morpheus1q8rc050907hx39vfejpawjydmwe6uujw0njx9s6skzdpp3cm2he5s036p07
✔ amount: 10
continue (y/n): y
✅ txID: sceRdaoqu2AAyLdHCdQkENZaXngGjRoc8nFdGyG8D9pCbTjbk
```

### Bonus: Watch Activity in Real-Time
To provide a better sense of what is actually happening on-chain, the
`morpheus-cli` comes bundled with a simple explorer that logs all blocks/txs that
occur on-chain. You can run this utility by running the following command from
this location:
```bash
./build/morpheus-cli chain watch
```

If you run it correctly, you'll see the following input (will run until the
network shuts down or you exit):
```
database: .morpheus-cli
available chains: 1 excluded: []
0) chainID: 2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk
select chainID: 0
uri: http://127.0.0.1:9650/ext/bc/morpheusvm
watching for new blocks on 2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk 👀
height:1 txs:1 units:440 root:WspVPrHNAwBcJRJPVwt7TW6WT4E74dN8DuD3WXueQTMt5FDdi
✅ sceRdaoqu2AAyLdHCdQkENZaXngGjRoc8nFdGyG8D9pCbTjbk actor: morpheus1qrzvk4zlwj9zsacqgtufx7zvapd3quufqpxk5rsdd4633m4wz2fdjk97rwu units: 440 summary (*actions.Transfer): [10.000000000 RED -> morpheus1q8rc050907hx39vfejpawjydmwe6uujw0njx9s6skzdpp3cm2he5s036p07]
```
