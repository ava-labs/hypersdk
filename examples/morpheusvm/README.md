<p align="center">
  <img width="90%" alt="morpheusvm" src="assets/logo.jpeg">
</p>
<p align="center">
  The Choice is Yours
</p>
<p align="center">
  <a href="https://github.com/ava-labs/hypersdk/actions/workflows/morpheusvm-static-analysis.yml"><img src="https://github.com/ava-labs/hypersdk/actions/workflows/morpheusvm-static-analysis.yml/badge.svg" /></a>
  <a href="https://github.com/ava-labs/hypersdk/actions/workflows/morpheusvm-unit-tests.yml"><img src="https://github.com/ava-labs/hypersdk/actions/workflows/morpheusvm-unit-tests.yml/badge.svg" /></a>
  <a href="https://github.com/ava-labs/hypersdk/actions/workflows/morpheusvm-sync-tests.yml"><img src="https://github.com/ava-labs/hypersdk/actions/workflows/morpheusvm-sync-tests.yml/badge.svg" /></a>
  <a href="https://github.com/ava-labs/hypersdk/actions/workflows/morpheusvm-load-tests.yml"><img src="https://github.com/ava-labs/hypersdk/actions/workflows/morpheusvm-load-tests.yml/badge.svg" /></a>
</p>

---

_[Who is Morpheus ("The Matrix")?](https://www.youtube.com/watch?v=zE7PKRjrid4)_

The [`morpheusvm`](./examples/morpheusvm) provides the first glimpse into the world of the `hypersdk`.
After learning how to implement native token transfers in a `hypervm` (one of the simplest Custom VMs
you could make), you will have the choice to go deeper (red pill) or to turn back to the VMs that you
already know (blue pill).

When you are ready to build your own `hypervm`, we recommend using the `morpheusvm` as a template!

## Status
`morpheusvm` is considered **ALPHA** software and is not safe to use in
production. The framework is under active development and may change
significantly over the coming months as its modules are optimized and
audited.

## Demo
### Launch Subnet
The first step to running this demo is to launch your own `morpheusvm` Subnet. You
can do so by running the following command from this location (may take a few
minutes):
```bash
./scripts/run.sh;
```

When the Subnet is running, you'll see the following logs emitted:
```
cluster is ready!
avalanche-network-runner is running in the background...

use the following command to terminate:

./scripts/stop.sh;
```

_By default, this allocates all funds on the network to `morpheus1qrzvk4zlwj9zsacqgtufx7zvapd3quufqpxk5rsdd4633m4wz2fdjk97rwu`. The private
key for this address is `0x323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7`.
For convenience, this key has is also stored at `demo.pk`._

### Build `morpheus-cli`
To make it easy to interact with the `morpheusvm`, we implemented the `morpheus-cli`.
Next, you'll need to build this tool. You can use the following command:
```bash
./scripts/build.sh
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

Next, you'll need to store the URLs of the nodes running on your Subnet:
```bash
./build/morpheus-cli chain import-anr
```

If `morpheus-cli` is able to connect to ANR, it will emit the following logs:
```
database: .morpheus-cli
stored chainID: 2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk uri: http://127.0.0.1:45778/ext/bc/2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk
stored chainID: 2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk uri: http://127.0.0.1:58191/ext/bc/2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk
stored chainID: 2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk uri: http://127.0.0.1:16561/ext/bc/2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk
stored chainID: 2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk uri: http://127.0.0.1:14628/ext/bc/2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk
stored chainID: 2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk uri: http://127.0.0.1:44160/ext/bc/2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk
```

_`./build/morpheus-cli chain import-anr` connects to the Avalanche Network Runner server running in
the background and pulls the URIs of all nodes tracking each chain you
created._


### Check Balance
To confirm you've done everything correctly up to this point, run the
following command to get the current balance of the key you added:
```bash
./build/morpheus-cli key balance
```

If successful, the balance response should look like this:
```
database: .morpheus-cli
address:morpheus1qrzvk4zlwj9zsacqgtufx7zvapd3quufqpxk5rsdd4633m4wz2fdjk97rwu
chainID: 2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk
uri: http://127.0.0.1:45778/ext/bc/2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk
balance: 1000.000000000 RED
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
chainID: 2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk
stored keys: 2
0) address (ed25519): morpheus1qrzvk4zlwj9zsacqgtufx7zvapd3quufqpxk5rsdd4633m4wz2fdjk97rwu balance: 10000000000.000000000 RED
1) address (secp256r1): morpheus1q8rc050907hx39vfejpawjydmwe6uujw0njx9s6skzdpp3cm2he5s036p07 balance: 0.000000000 RED
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
uri: http://127.0.0.1:45778/ext/bc/2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk
watching for new blocks on 2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk 👀
height:1 txs:1 units:440 root:WspVPrHNAwBcJRJPVwt7TW6WT4E74dN8DuD3WXueQTMt5FDdi
✅ sceRdaoqu2AAyLdHCdQkENZaXngGjRoc8nFdGyG8D9pCbTjbk actor: morpheus1qrzvk4zlwj9zsacqgtufx7zvapd3quufqpxk5rsdd4633m4wz2fdjk97rwu units: 440 summary (*actions.Transfer): [10.000000000 RED -> morpheus1q8rc050907hx39vfejpawjydmwe6uujw0njx9s6skzdpp3cm2he5s036p07]
```

<br>
<br>
<br>
<p align="center">
  <a href="https://github.com/ava-labs/hypersdk"><img width="40%" alt="powered-by-hypersdk" src="assets/hypersdk.png"></a>
</p>
