WRVM (White Rabbit VM)

---


The [`wrvm`](./x/programs/examples/wrvm) provides the first glimpse into the world of the `hypersdk` program support.

## Status
`wrvm` is considered **ALPHA** software and is not safe to use in
production. The framework is under active development and may change
significantly over the coming months as its modules are optimized and
audited.

## Demo
### Launch Subnet
The first step to running this demo is to launch your own `wrvm` Subnet. You
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

### Build `wr-cli`
To make it easy to interact with the `wrvm`, we implemented the `wr-cli`.
Next, you'll need to build this tool. You can use the following command:
```bash
./scripts/build.sh
```
<br>
<br>
<br>
<p align="center">
  <a href="https://github.com/ava-labs/hypersdk"><img width="40%" alt="powered-by-hypersdk" src="assets/hypersdk.png"></a>
</p>
