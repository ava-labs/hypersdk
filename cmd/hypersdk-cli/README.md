# HyperSDK CLI

A command-line interface for interacting with HyperSDK-based chains.

## Installation

```bash
go install github.com/ava-labs/hypersdk/cmd/hypersdk-cli@main
```

<!--FIXME: Point to the latest tag once tagged.-->

## Configuration

The CLI stores configuration in `~/.hypersdk-cli/config.yaml`. This includes:
- Private key
- Endpoint URL

Example setup for a local HyperSDK VM:
```bash
hypersdk-cli endpoint set --endpoint=http://localhost:9650/ext/bc/morpheusvm/ 
hypersdk-cli key set --key=0x323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7
```

## Global Flags

- `--endpoint`: Override the default endpoint for a single command
- `-o, --output`: Set output format (`text` or `json`)

## Commands


### key

Manage keys.

#### generate

Generate a new ED25519 key pair.

```bash
hypersdk-cli key generate
```

#### set

Set the private ED25519 key. 

```bash
hypersdk-cli key set --key=0x323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7
```

`--key` could also be file path like `examples/morpheusvm/demo.pk`

### endpoint

Print the current endpoint URL.

```bash
hypersdk-cli endpoint
```

#### set

Set the endpoint URL.

```bash
hypersdk-cli endpoint set --endpoint=http://localhost:9650/ext/bc/morpheusvm/
```

### ping

Check connectivity with the current endpoint.

```bash
hypersdk-cli ping
```

### address

Print the current key address.

```bash
hypersdk-cli address
```

### actions

Print the list of actions available in the ABI.

```bash
hypersdk-cli actions
```

For JSON output:

```bash
hypersdk-cli actions -o json
```

### read

Simulate a single action transaction.

```bash
hypersdk-cli read Transfer --data to=0x000000000000000000000000000000000000000000000000000000000000000000a7396ce9,value=12,memo=0xdeadc0de
```

For interactive input remove --data from the command line:

```bash
hypersdk-cli read Transfer
```

### tx

Send a transaction with a single action.

```bash
hypersdk-cli tx Transfer --data to=0x000000000000000000000000000000000000000000000000000000000000000000a7396ce9,value=12,memo=0x001234
```

For interactive input:

```bash
hypersdk-cli tx Transfer
```

### balance

Query the balance of an address

```bash
hypersdk-cli balance --sender 0x000000000000000000000000000000000000000000000000000000000000000000a7396ce9
```

If `--sender` isn't provided, the address associated with the private key in
`~/.hypersdk-cli/config.yaml` is queried.


## Notes

- Only flat actions are supported. Arrays, slices, embedded structs, maps, and struct fields are not supported.
- The CLI supports ED25519 keys only.
- If `--data` is supplied or JSON output is selected, the CLI will not ask for action arguments interactively.

## Known Issues

- The `maxFee` for transactions is currently hardcoded to 1,000,000.
- The `key set` and `endpoint set` commands use a nested command structure which adds unnecessary complexity for a small CLI tool. A flatter command structure would be more appropriate.
- Currency values are represented as uint64 without decimal point support in the ABI. The CLI cannot automatically parse decimal inputs (e.g. "12.0") since there is no currency type annotation. Users must enter the raw uint64 value including all decimal places (e.g. "12000000000" for 12 coins with 9 decimal places).
