# Contributing to hypersdk

Thank you for your interest in contributing to `hypersdk`! By contributing to hypersdk, you are helping to build the foundation for the next generation of blockchains and decentralized applications.

## Getting Started

### Prerequisites

To contribute to `hypersdk`, you'll need:

- [Go](https://golang.org/dl/) 1.19 or higher

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

3. Build the project:

```go
go build ./...
```
This will build the project and its dependencies.

## Building and running tests

To build and run tests for the hypersdk, simply run:

```go
go test -timeout 30s ./...
```

This will build and run all tests for the project.

## Building and running the project

> TODO: How to build and run the project

## Running linters

> TODO: How to lint and format 

## Contributing

We welcome contributions to hypersdk! To contribute, please follow these steps:

1. Fork the repository and create a new branch for your contribution.

2. Make your changes and ensure that all tests pass and linting is clean.

3. Write tests for any new features or bug fixes. (If necessary)

4. Submit a pull request with your changes.

## Pull Request Guidelines

When submitting a pull request, please ensure that:

1. Your code is formatted using `go fmt`.

2. Your code is properly tested.

3. Your code passes all linters.

4. Your pull request description explains the problem and solution clearly.

## Code of Conduct

To ensure a welcoming and inclusive community, please follow the `hypersdk` [Code of Conduct]() in all interactions both on and offline.
