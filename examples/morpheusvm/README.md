<p align="center">
  <img width="90%" alt="morpheusvm" src="assets/hyperevm2.png">
</p>
<p align="center" style="font-weight: bold;">
  EVM and beyond
</p>

---

## Status
`hyperevm` is considered **ALPHA** software for demonstration purposes only. It is not safe to use in production.

## Getting Started

### Building HyperEVM from Source

#### Clone the HyperSDK

Clone the HyperSDK repository:

```sh
git clone git@github.com:ava-labs/hypersdk.git
cd hypersdk/examples/hyperevm
```

This will clone and checkout the `main` branch.

#### Building HyperEVM

Build HyperEVM by running the build script:

```sh
./scripts/build.sh
```

The `hyperevm` binary is now in the `build/./` directory.

### Run Integration Tests

To run the integration tests for HyperEVM, run the following command:

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

_By default, this allocates all funds on the network to `0x00c4cb545f748a28770042f893784ce85b107389004d6a0e0d6d7518eeae1292d9`. The private
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
killall avalanchego
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
imported address: 0x00c4cb545f748a28770042f893784ce85b107389004d6a0e0d6d7518eeae1292d9
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
address: 0x00c4cb545f748a28770042f893784ce85b107389004d6a0e0d6d7518eeae1292d9
chainID: JkJpw8ZPExTushPYYN4C8f7RHxjDRX8MAGGUGAdRRPEC2M3fx
uri: http://127.0.0.1:9650/ext/bc/morpheusvm
address: 0x00c4cb545f748a28770042f893784ce85b107389004d6a0e0d6d7518eeae1292d9 balance: 1000.000000000 RED
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
0) address: 0x00c4cb545f748a28770042f893784ce85b107389004d6a0e0d6d7518eeae1292d9 balance: 10000000000.000000000 RED
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
✅ sceRdaoqu2AAyLdHCdQkENZaXngGjRoc8nFdGyG8D9pCbTjbk actor: 0x00c4cb545f748a28770042f893784ce85b107389004d6a0e0d6d7518eeae1292d9 units: 440 summary (*actions.Transfer): [10.000000000 RED -> morpheus1q8rc050907hx39vfejpawjydmwe6uujw0njx9s6skzdpp3cm2he5s036p07]
```

If you are running this on a local network, you may see that all blocks are empty.
To view the same results with non-empty transactions, you can run the spam command from another window:
```bash
./build/morpheus-cli spam run ed25519
```

After inputting the parameters, you'll see output from the spam command showing inflight transactions:

```
minimum txs to issue per second: 10
txs to increase per second: 10
number of clients per node: 10
unique participants expected every 60s: 10
distributing funds to each account: 999.978355900 RED
distributed funds to 10 accounts
initial target tps: 10
txs seen: 10 success rate: 100.00% inflight: 10 issued/s: 21 unit prices: [bandwidth=100 compute=100 storage(read)=100 storage(allocate)=100 storage(write)=100]
```

The transactions will start to show up in the CLI explorer tool as well:

```
watching for new blocks on bsED31vJbynkhzKrjtNFtUXcg6Y78MjQLg1KBgesMw1xom8F9 👀
height:634 txs:0 root:B6upYeCegoR5bvfUGLo9GqbymyBCgXNobpTk53HBFNBZGG4aM size:0.08KB units consumed: [bandwidth=0 compute=0 storage(read)=0 storage(allocate)=0 storage(write)=0] unit prices: [bandwidth=100 compute=100 storage(read)=100 storage(allocate)=100 storage(write)=100]
height:635 txs:10 root:2W6eEZQYqD9xCsZoDPjASqtZx25fyUUobpesuKJJAxnFMU4RJi size:2.04KB units consumed: [bandwidth=2000 compute=70 storage(read)=140 storage(allocate)=500 storage(write)=260] unit prices: [bandwidth=100 compute=100 storage(read)=100 storage(allocate)=100 storage(write)=100] [TPS:85.69 latency:81ms gap:116ms]
✅ 2QEttqwxZyMF8Lzyj44wpsEhpbVyASq8c7Z4ogh3DDrWDKgKyh actor: 0090dc1ecabfc7680d68bc226158095861544b9309b251eed2f3d2425bc991285f summary (*actions.Transfer): [0.000000001 RED -> 0090dc1ecabfc7680d68bc226158095861544b9309b251eed2f3d2425bc991285f
] fee (max 86.84%): 0.000029700 RED consumed: [bandwidth=200 compute=7 storage(read)=14 storage(allocate)=50 storage(write)=26]
✅ 2HVNY8gbeBCrNiekTReGpKZ9hqkZea8Wf4VGjbdroorwEY7s7u actor: 00bd82f4be137f29222695f693e72a9e85e83510e575a3e485eb306a8ad5999010 summary (*actions.Transfer): [0.000000001 RED -> 00bd82f4be137f29222695f693e72a9e85e83510e575a3e485eb306a8ad5999010
] fee (max 86.84%): 0.000029700 RED consumed: [bandwidth=200 compute=7 storage(read)=14 storage(allocate)=50 storage(write)=26]
✅ 2WXLjEXf25WeinidC9qmghZWbCeDa26F8pwwkFb53MSsEQm1NL actor: 0090dc1ecabfc7680d68bc226158095861544b9309b251eed2f3d2425bc991285f summary (*actions.Transfer): [0.000000001 RED -> 0090dc1ecabfc7680d68bc226158095861544b9309b251eed2f3d2425bc991285f
] fee (max 86.84%): 0.000029700 RED consumed: [bandwidth=200 compute=7 storage(read)=14 storage(allocate)=50 storage(write)=26]
```
