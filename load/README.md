# Load

This package provides generic utilities for blockchain load testing. We break load generation down into the following components:

- tx generator(s)
- tx issuer(s)
- orchestrator

The transaction generator(s) and issuer(s) may be VM specific and provide the necessary injected dependencies for the orchestrator. This enables us to construct different load testing strategies on top of the same re-usable code. For example, we can re-use these components for a short burst of transactions or to perform gradual load testing.

## Transaction Generator

## 