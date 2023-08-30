### Program Simulator

## Introduction

This simulator allows you to run, test and play around with your own compiled WASM programs!

#### build

```sh
go build cmd/simulator/simulator.go
```

### generate new keys

```sh
./simulator key generate

created new private key with public address: sim_key_dc2
```

### create new program tx

```sh
./simulator program create --caller sim_key_dc2 ../examples/testdata/token.wasm

create program action successful program id: 1
```

### invoke program tx

```sh
./simulator program invoke --id 1 --caller sim_key_dc2 --function mint_to --params sim_key_dc2,100

response: [1]

./simulator program invoke --id 1 --caller sim_key_dc2 --function get_balance --params sim_key_dc2

response: [100]
```
