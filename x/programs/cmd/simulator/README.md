# Program VM simulator (work in progress)

The `Simulator` is currently a work-in-progress.

It is a CLI-tool built in Go, but meant to be used from the `wasmlanche-sdk` as a way to run automated tests on your HyperSDK Wasm programs. For relatively up-to-date documentation, run `cargo doc --no-deps -p simulator --open`.

And please, if you see any inconsistencies in the `README.md` here, [open a PR](https://github.com/ava-labs/hypersdk/edit/main/x/programs/cmd/simulator/README.md)!

#### Note

Are we calling a Go-based CLI from Rust? Yes. The Go-CLI re-uses primitives from the HyperSDK, but we wanted to wrap that code in a Rust client to give a seamless experience testing.

## Build

```sh
go build
```
