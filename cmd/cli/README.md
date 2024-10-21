We need a CLI to do read only and write actions in transactions. 

Requirements:
- JSON/human-readable output. You can add `-o json` or `-o human` to any request
- action payload through interactive UI and through a one line tx, meaning any action argument, that's missing would be asked interactively
- Only flat actons are supported, no arrays, slices embeded structs, maps and struct fields.
- Keeps the private key and the endpoint in the user's home folder in `~/.hypersdk-cli/config.cfg`
- Argument `--endpoint` added to any request, overrides the request endpoint. If `--endpoint` is not set and `~/.hypersdk-cli/config.cfg` doesn't have endpoint field, errors
- Supports ed25519 keys only (enforce default, this is a default opinionated CLI)
- If --data supplied, or json selected as an output would not ask for the action arguments

API:
✅ `address` - prints current key address
✅ `key generate` - generates a new address
`balance` - balance
✅ `key set` set private key from file, base64 string or hex string presented in the first argument
✅ `endpoint` - prints URL of the current endpoint
✅ `ping` - checks connectivity with the current endpoing
✅ `endpoint set --endpoint=https://hello.world:1234` sets endpoint
✅ `actions` - print the list of actions available in the ABI, for JSON it prints ABI JSON
`tx [action name] [action params]` - sends a transaction with a single action
`read [action name] [action params]` - simulates a single action transaction


```bash
go run ./cmd/cli/ read Transfer --key=./examples/morpheusvm/demo.pk --data to=0x000000000000000000000000000000000000000000000000000000000000000000,value=12 
```
