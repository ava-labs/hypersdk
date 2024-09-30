# Wasmlanche-sdk

This is the SDK to write web-assembly based smart-contracts for a HyperVM with smart-contracts enabled.

First and foremost, this SDK is under active development and keeping the documentation up to date with rapidly changing code can be difficult. If you see any discrepencies, please feel free to open up a GitHub issue, or even submit a PR.

### Concepts

This SDK _should_ be fairly easy to use, you just have to learn a couple of simple concepts.

Any Rust that compiles to the `wasm32-unknown-unknown` target should be valid. The SDK provides all the necessary `extern` calls into the runtime and provides safe-wrappers around these calls. It also provides a couple of macros to simplify _state_ and _function_ definitions. You can think of the `state_schema` just like you would a schema definition for a relational-database. And _public_ functions can be thought of as entrypoints with access to the particular state-space belonging to the smart-contract.

For further details, please see the generated rust-docs. You can generate them locally by running `cargo doc --open`.
