# ABI Package

## Overview
The ABI package provides functionality for marshaling and unmarshaling actions. It is designed to work across different language implementations.

## ABI Format
The ABI is defined in JSON format, as shown in the `abi.json` file:
```json
{
    "actions": [
        {
            "id": 1,
            "action": "MockObjectSingleNumber"
        },
    ],
    "types": [
        {
            "name": "MockObjectSingleNumber",
            "fields": [
                {
                    "name": "Field1",
                    "type": "uint16"
                }
            ]
        },
    ]
}
```

The ABI consists of two main sections:
- actions: A list of action definitions, each with an id and action name.
- types: A list of type definitions, each with a name and a list of fields.

## Implementation
To create an implementation of this package in any other langauge:
- Copy the testdata folder.
- Ensure all marshaling is identical to the Go implementation.
- JSON files should align with their corresponding .hex files.

## Hash verification
Wallets use ABI to display proper action names and field names. To verify ABI implementation in other languages, marshal the ABI into binary, hash it, and compare with the known hash.

## Constraints
- Actions must have an ID; other structs do not require one.
- Multiple structs with the same name from different packages are not supported.
- Maps are not supported; use slices (arrays) instead.
- Built-in types include `codec.Address` and `codec.Bytes`.
- In order to decode an action, find a type with the same name and all the other types mentioned in this type as field recursively. 

## Code Generation
Use cmd/abigen to automatically generate Go structs from JSON. For example: `go run ./cmd/abigen/ ./abi/testdata/abi.json ./example.go --package=testpackage`

## Type list
TODO:

## 