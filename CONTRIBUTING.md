# Contributing to hypersdk

Thank you for your interest in contributing to `hypersdk`! By contributing to hypersdk, you are helping to build the foundation for the next generation of blockchains and decentralized applications.

## Getting Started

### Prerequisites

To contribute to `hypersdk`, you'll need:

- [Go](https://golang.org/dl/) 1.23.7 or higher

On MacOS, a modern version of bash is required (e.g. via [homebrew](https://brew.sh/) with `brew install bash`). The version installed by default is not compatible with HyperSDK's [shell scripts](scripts).

An [optional dev shell](#optional-dev-shell) may be used to gain access to additional dependencies.

### Setting up your development environment

1. Clone the repository:

```bash
git clone https://github.com/ava-labs/hypersdk.git
cd hypersdk
```

2. Install the dependencies:

```go
go mod download
```

This will download and install all required dependencies for the project.

## Building and running tests

To build and run tests for the hypersdk, simply run:

```go
./scripts/tests.unit.sh
```

This will build and run all tests for the project.

## Running linters

To run the linters, simply run:

```go
./scripts/lint.sh
```

This will run the linters on all code in the project.

The `hypersdk` project also has a fixer that tries to help. To run the fixer, simply run:

```go
./scripts/fix.lint.sh
```

## Optional Dev Shell

Some activities, such as collecting metrics and logs from the nodes targeted by an e2e
test run, require binary dependencies. One way of making these dependencies available is
to use a nix shell which will give access to the dependencies expected by the test
tooling:

 - Install [nix](https://nixos.org/). The [determinate systems
   installer](https://github.com/DeterminateSystems/nix-installer?tab=readme-ov-file#install-nix)
   is recommended.
 - Use ./scripts/dev_shell.sh to start a nix shell
 - Execute the dependency-requiring command (e.g. `ginkgo -v ./tests/e2e -- --start-collectors`)

This repo also defines a `.envrc` file to configure [devenv](https://direnv.net/). With `devenv`
and `nix` installed, a shell at the root of the hypersdk repo will automatically start a nix dev
shell.

## Contributing

We welcome contributions to hypersdk! To contribute, please follow these steps:

1. Fork the repository and create a new branch for your contribution.

2. Make your changes, sign all of your commits, and ensure that all tests pass and linting is clean.

3. Write tests for any new features or bug fixes. (If necessary)

4. Submit a pull request with your changes.

## Pull Request Guidelines

When submitting a pull request, please ensure that:

1. Your code is formatted using `go fmt`.

2. Your code is properly tested.

3. Your code passes all linters.

4. Your pull request description explains the problem and solution clearly.
