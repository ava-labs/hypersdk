<p align="center">
  <img width="90%" alt="simplevm" src="assets/logo.png">
</p>
<p align="center">
  TODO
</p>
<p align="center">
  <a href="https://github.com/ava-labs/hypersdk/actions/workflows/simplevm-static-analysis.yml"><img src="https://github.com/ava-labs/hypersdk/actions/workflows/simplevm-static-analysis.yml/badge.svg" /></a>
  <a href="https://github.com/ava-labs/hypersdk/actions/workflows/simplevm-unit-tests.yml"><img src="https://github.com/ava-labs/hypersdk/actions/workflows/simplevm-unit-tests.yml/badge.svg" /></a>
  <a href="https://github.com/ava-labs/hypersdk/actions/workflows/simplevm-sync-tests.yml"><img src="https://github.com/ava-labs/hypersdk/actions/workflows/simplevm-sync-tests.yml/badge.svg" /></a>
  <a href="https://github.com/ava-labs/hypersdk/actions/workflows/simplevm-load-tests.yml"><img src="https://github.com/ava-labs/hypersdk/actions/workflows/simplevm-load-tests.yml/badge.svg" /></a>
</p>

---

The `simplevm` is the "simplest `hypervm` you could make." The `simplevm`
lets you do one thing and one thing only: native token transfers.

When you are ready to build your own `hypervm`, we recommend
using the `simplevm` as a template!

## Status
`simplevm` is considered **ALPHA** software and is not safe to use in
production. The framework is under active development and may change
significantly over the coming months as its modules are optimized and
audited.

## Demo
### Launch Subnet
The first step to running this demo is to launch your own `simplevm` Subnet. You
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

_By default, this allocates all funds on the network to `simple1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97ns3deh6g`. The private
key for this address is `0x323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7`.
For convenience, this key has is also stored at `demo.pk`._

### Build `simple-cli`
To make it easy to interact with the `simplevm`, we implemented the `simple-cli`.
Next, you'll need to build this tool. You can use the following command:
```bash
./scripts/build.sh
```

_This command will put the compiled CLI in `./build/simple-cli`._

### Configure `simple-cli`
Next, you'll need to add the chains you created and the default key to the
`simple-cli`. You can use the following commands from this location to do so:
```bash
./build/simple-cli key import demo.pk
```

If the key is added corretcly, you'll see the following log:
```
database: .simple-cli
imported address: simple1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97ns3deh6g
```

Next, you'll need to store the URLs of the nodes running on your Subnet:
```bash
./build/simple-cli chain import-anr
```

If `simple-cli` is able to connect to ANR, it will emit the following logs:
```
database: .simple-cli
stored chainID: 2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk uri: http://127.0.0.1:45778/ext/bc/2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk
stored chainID: 2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk uri: http://127.0.0.1:58191/ext/bc/2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk
stored chainID: 2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk uri: http://127.0.0.1:16561/ext/bc/2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk
stored chainID: 2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk uri: http://127.0.0.1:14628/ext/bc/2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk
stored chainID: 2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk uri: http://127.0.0.1:44160/ext/bc/2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk
```

_`./build/simple-cli chain import-anr` connects to the Avalanche Network Runner server running in
the background and pulls the URIs of all nodes tracking each chain you
created._


### Check Balance
To confirm you've done everything correctly up to this point, run the
following command to get the current balance of the key you added:
```bash
./build/simple-cli key balance
```

If successful, the balance response should look like this:
```
database: .simple-cli
address: simple1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97ns3deh6g
chainID: 2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk
uri: http://127.0.0.1:45778/ext/bc/2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk
balance: 1000.000000000 SIMP
```

### Generate Another Address
Now that we have a balance to send, we need to generate another address to send to. Because
we use bech32 addresses, we can't just put a random string of characters as the reciepient
(won't pass checksum test that protects users from sending to off-by-one addresses).
```bash
./build/simple-cli key generate
```

If successful, the `simple-cli` will emit the new address:
```
database: .simple-cli
created address: simple1lst0js5nrhavcrquj2td97x02vxj0ktw3fre02p4qutw2ehd20nslnqv60
```

By default, the `simple-cli` sets newly generated addresses to be the default. We run
the following command to set it back to `demo.pk`:
```bash
./build/simple-cli key set
```

You should see something like this:
```
database: .simple-cli
chainID: 2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk
stored keys: 2
0) address: simple1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97ns3deh6g balance: 1000.000000000 SIMP
1) address: simple1lst0js5nrhavcrquj2td97x02vxj0ktw3fre02p4qutw2ehd20nslnqv60 balance: 0.000000000 SIMP
set default key: 0
```

### Send Tokens
Lastly, we trigger the transfer:
```bash
./build/simple-cli action transfer
```

The `simple-cli` will emit the following logs when the transfer is successful:
```
database: .simple-cli
address: simple1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97ns3deh6g
chainID: 2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk
balance: 1000.000000000 SIMP
recipient: simple1lst0js5nrhavcrquj2td97x02vxj0ktw3fre02p4qutw2ehd20nslnqv60
✔ amount: 10
continue (y/n): y
✅ txID: sceRdaoqu2AAyLdHCdQkENZaXngGjRoc8nFdGyG8D9pCbTjbk
```

### Bonus: Watch Activity in Real-Time
To provide a better sense of what is actually happening on-chain, the
`simple-cli` comes bundled with a simple explorer that logs all blocks/txs that
occur on-chain. You can run this utility by running the following command from
this location:
```bash
./build/simple-cli chain watch
```

If you run it correctly, you'll see the following input (will run until the
network shuts down or you exit):
```
database: .simple-cli
available chains: 1 excluded: []
0) chainID: 2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk
select chainID: 0
uri: http://127.0.0.1:45778/ext/bc/2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk
watching for new blocks on 2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk 👀
height:1 txs:1 units:440 root:WspVPrHNAwBcJRJPVwt7TW6WT4E74dN8DuD3WXueQTMt5FDdi
✅ sceRdaoqu2AAyLdHCdQkENZaXngGjRoc8nFdGyG8D9pCbTjbk actor: simple1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97ns3deh6g units: 440 summary (*actions.Transfer): [10.000000000 SIMP -> simple1lst0js5nrhavcrquj2td97x02vxj0ktw3fre02p4qutw2ehd20nslnqv60]
```

<br>
<br>
<br>
<p align="center">
  <a href="https://github.com/ava-labs/hypersdk"><img width="40%" alt="powered-by-hypersdk" src="assets/hypersdk.png"></a>
</p>
