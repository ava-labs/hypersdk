# Custom import modules

## Introduction

Imports are custom host functions exposed from the host to the `Program`.
See the examples in this folder for details on how to implement your
own custom import module.

## Components

Each import must implement the `runtime.Import` interface.

```golang
// register all supported imports
supported := runtime.NewSupportedImports()
supported.Register("state", func() runtime.Import {
	return pstate.New(log, db)
})
supported.Register("something", func() runtime.Import {
	return something.New(log, db)
})

// pass supported imports to runtime.
rt := runtime.New(log, cfg, supported.Imports())

```

## Rust SDK

See the implementation of
[state](https://github.com/ava-labs/hypersdk/blob/8ecd682bba203a55e964e4b0841b45cd52773da5/x/programs/rust/wasmlanche_sdk/src/host/state.rs)
for an example of how you can extend your program to support custom imports in
a Rust `Program`.
