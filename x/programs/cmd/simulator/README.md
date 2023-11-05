# Program VM simulator

## Introduction

The VM simulator provides a tool for testing and interacting with HyperSDK Wasm
`Programs`.

## Build

```sh
go build
```

## Getting Started

To try out out test token program its as easy as one command.

```sh
./simulator run ./cmd/testdata/basic_token.yaml
```
```json
{"id":0,"result":{"msg":"created key bob_key","timestamp":1697835142}}
{"id":1,"result":{"msg":"created key alice_key","timestamp":1697835142}}
{"id":2,"result":{"id":"2ut4fwdGE5FJG5w89CF3pVCjLrhiqCRZxB7ojtPnigh7QVU51i","timestamp":1697835142}}
{"id":3,"result":{"id":"5vQVVgXRygeYWcJ9bsRLLDs92wuS1B2qyndqEAmzXXf3ZQwAq","timestamp":1697835142}}
{"id":4,"result":{"id":"2pUQ85962MhjmQpzZsSGhFJCWLdeEWNU9W2ZZH4EnFYyAAF4Qr","timestamp":1697835142}}
{"id":5,"result":{"response":[10000],"timestamp":1697835142}}
```

## Testing a HyperSDK Programs

To test a HyperSDK Program you will need to create a `Plan` file which can be
either `JSON` and `YAML`. Look at the example `./cmd/testdata/basic_token.yaml`
for hints on usage.

### Deploy a HyperSDK Program

In this example we will create a new key `my_key` and deploy a new program

```yaml
# new_program.yaml
# example of creating a new key and deploying a new program using a `plan` file
steps:
  - description: create my key
    endpoint: key
    method: create
    params:
      - name: key name
        type: ed25519
        value: my_key
  - description: create my program
    endpoint: execute # execute endpoint is for creating a transaction
    method: program_create # program create is a method supported by the simulator
    max_units: 100000
    params:
      - name: program_path
        type: string
        value: ./my_program.wasm
```

Next we will run the simulation

```sh
$./simulator run ./new_program.yaml
```
```json
{"id":0,"result":{"msg":"created key my_key","timestamp":1697835142}}
{"id":2,"result":{"id":"2ut4fwdGE5FJG5w89CF3pVCjLrhiqCRZxB7ojtPnigh7QVU51i","timestamp":1697835142}}
```

Congratulations you have just deployed your first HyperSDK program! Lets make
sure to keep track of the transaction ID
`2ut4fwdGE5FJG5w89CF3pVCjLrhiqCRZxB7ojtPnigh7QVU51i`.

### Interact with a HyperSDK Program

Now that the program is on chain lets interact with it.

```yaml
# play_program.yaml
name: play
description: Playing with new program
caller_key: my_key
steps:
  - description: add two numbers
    endpoint: readonly # readonly will return the result of the function
    method: addition # name of the Wasm function to call
    max_units: 10000
    params:
      - name: program_id
        type: id
        value: 2ut4fwdGE5FJG5w89CF3pVCjLrhiqCRZxB7ojtPnigh7QVU51i
      - type: uint64
        value: 100
      - type: uint64
        value: 100
    require:
      result:
        operator: "=="
        value: 200
```

### Interact with Rust!

The Rust SDK now allows for writing `Plans` in pure Rust.

## Deploy and Interact with HyperSDK Program

The above examples show how to deploy and interact with a `HyperSDK` program in
separate `Plan`  files but we can also perform all of this in a single run. To reference a tx ID in you `Plan` just use the string `step_N` where `N` is the step number the tx was created


## Import Modules

Currently the simulator supports the `program` and `pstate` modules found in the
examples/imports directory.
