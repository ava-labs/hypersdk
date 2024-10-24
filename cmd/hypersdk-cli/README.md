# HyperSDK CLI

A command-line interface for interacting with HyperSDK-based chains.

## Installation

```bash
go install github.com/ava-labs/hypersdk/cmd/hypersdk-cli@0d9e79c1c38c9e0611d29144040a8def8f2f60e3
```

TODO: Has to be @latest

## Configuration

The CLI stores configuration in `~/.hypersdk-cli/config.cfg`. This includes:
- Private key
- Endpoint URL

## Global Flags

- `--endpoint`: Override the default endpoint for a single command
- `-o, --output`: Set output format (`human` or `json`)

## Commands

### address

Print the current key address.

```bash
hypersdk-cli address
```

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
hypersdk-cli read Transfer --data to=0x000000000000000000000000000000000000000000000000000000000000000000,value=12
```

For interactive input remove --data from the comand line:

```bash
hypersdk-cli read Transfer
```

### tx

Send a transaction with a single action.

```bash
hypersdk-cli tx Transfer --data to=0x000000000000000000000000000000000000000000000000000000000000000000,value=12,memo=
```

For interactive input:

```bash
hypersdk-cli tx Transfer
```

## Notes

- The `balance` command is not currently implemented due to the lack of a standardized balance RPC method at the HyperSDK level.
- The `maxFee` for transactions is currently hardcoded to 1,000,000.
- Only flat actions are supported. Arrays, slices, embedded structs, maps, and struct fields are not supported.
- The CLI supports ED25519 keys only.
- If `--data` is supplied or JSON output is selected, the CLI will not ask for action arguments interactively.
