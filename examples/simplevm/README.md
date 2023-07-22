<p align="center">
  <img width="90%" alt="simplevm" src="assets/logo.png">
</p>
<p align="center">
  Native Token Transfers. All Day. Every Day.
</p>
<p align="center">
  <a href="https://github.com/ava-labs/hypersdk/actions/workflows/simplevm-static-analysis.yml"><img src="https://github.com/ava-labs/hypersdk/actions/workflows/simplevm-static-analysis.yml/badge.svg" /></a>
  <a href="https://github.com/ava-labs/hypersdk/actions/workflows/simplevm-unit-tests.yml"><img src="https://github.com/ava-labs/hypersdk/actions/workflows/simplevm-unit-tests.yml/badge.svg" /></a>
  <a href="https://github.com/ava-labs/hypersdk/actions/workflows/simplevm-sync-tests.yml"><img src="https://github.com/ava-labs/hypersdk/actions/workflows/simplevm-sync-tests.yml/badge.svg" /></a>
  <a href="https://github.com/ava-labs/hypersdk/actions/workflows/simplevm-load-tests.yml"><img src="https://github.com/ava-labs/hypersdk/actions/workflows/simplevm-load-tests.yml/badge.svg" /></a>
</p>

---

The `simplevm`] is the "simplest `hypervm` you could make." The `simplevm`
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

_By default, this allocates all funds on the network to `simple1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97ns3deh6g`. The private
key for this address is `0x323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7`.
For convenience, this key has is also stored at `demo.pk`._

### Configure `simple-cli`
To make it easy to interact with the `simplevm`, we implemented the `simple-cli`.
Next, you'll need to build this. You can use the following command from this location
to do so:
```bash
./scripts/build.sh
```

_This command will put the compiled CLI in `./build/simple-cli`._

Lastly, you'll need to add the chains you created and the default key to the
`simple-cli`. You can use the following commands from this location to do so:
```bash
./build/simple-cli key import demo.pk
./build/simple-cli chain import-anr
```

_`chain import-anr` connects to the Avalanche Network Runner server running in
the background and pulls the URIs of all nodes tracking each chain you
created._

### Check Balance

### Generate Another Address

### Send Tokens

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
TODO
```

<br>
<br>
<br>
<p align="center">
  <a href="https://github.com/ava-labs/hypersdk"><img width="40%" alt="powered-by-hypersdk" src="assets/hypersdk.png"></a>
</p>
