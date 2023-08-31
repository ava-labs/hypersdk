### Program Simulator

## Introduction

This simulator allows you to run, test and play around with your own compiled WASM programs!

#### build

```sh
go build simulator.go
```

### generate new keys

```sh
./simulator key generate

created new private key with public address: sim_key_dc2
```

### create new program tx

This tx publishes and initializes the compiled binary. The compiled wasm must include an `init` function.

```sh
./simulator program create --caller sim_key_dc2 ../examples/testdata/token.wasm

create program action successful program id: 1
```

### invoke program tx

Reference the program id returned from the `create` tx, to invoke functions on your program!
Remember to reference the correct program id, as each published program has its own storage space on disk.
Pass your programs params(except `Context`) as a comma seperated list.

```sh
./simulator program invoke --id 1 --caller sim_key_dc2 --function mint_to --params sim_key_dc2,100

response: [1]

./simulator program invoke --id 1 --caller sim_key_dc2 --function get_balance --params sim_key_dc2

response: [100]
```

### lottery example

```sh
./simulator key generate <---- alice\'s key
./simulator key generate <---- bob\'s key
alice=sim_key_xxx
bob=sim_key_xxx
./simulator program create --caller $alice ../examples/testdata/token.wasm
./simulator program create --caller $alice ../examples/testdata/lottery.wasm
./simulator program invoke --id 1 --caller $alice --function mint_to --params $alice,10000
./simulator program invoke --id 2 --caller $alice --function set --params 1,$alice
./simulator program invoke --id 2 --caller $bob --function play --params $bob
./simulator program invoke --id 1 --caller $alice --function get_balance --params $alice
./simulator program invoke --id 1 --caller $bob --function get_balance --params $bob
```
