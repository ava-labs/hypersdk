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

Testing algorithm in pseudocode:
```
abi = abi.json

for filename in testdata/*.hex:
    if filename.endswith(".hash.hex"):
        continue
    expectedHex = readFile(filename)
    json = readFile(filename.replace(".hex", ".json"))

    actualHex = Marshal(abi, json)
    if actualHex != expectedHex:
        raise "Hex values do not match"

```

## Hash verification
Wallets use ABI to display proper action names and field names. To verify ABI implementation in other languages, marshal the ABI into binary, hash it, and compare it with the known hash. We do this to avoid a situation where a bad actor could provide a different ABI, tricking users into signing the wrong action.

## Constraints
- Actions must have an ID; other structs do not require one.
- Multiple structs with the same name from different packages are not supported.
- Maps are not supported; use slices (arrays) instead.
- Built-in types include `codec.Address` and `codec.Bytes`.
- In order to decode an action, find a type with the same name and all the other types mentioned in this type as field recursively. 

## Code Generation
Use cmd/abigen to automatically generate Go structs from JSON. For example: `go run ./cmd/abigen/ ./abi/testdata/abi.json ./example.go --package=testpackage`

## Type list

| Type     | Range/Description                                        | JSON Serialization | Binary Serialization                  |
|----------|----------------------------------------------------------|--------------------|---------------------------------------|
| `bool`   | true or false                                            | boolean            | 1 byte                                |
| `uint8`  | numbers from 0 to 255                                    | number             | 1 byte                                |
| `uint16` | numbers from 0 to 65535                                  | number             | 2 bytes                               |
| `uint32` | numbers from 0 to 4294967295                             | number             | 4 bytes                               |
| `uint64` | numbers from 0 to 18446744073709551615                   | number             | 8 bytes                               |
| `int8`   | numbers from -128 to 127                                 | number             | 1 byte                                |
| `int16`  | numbers from -32768 to 32767                             | number             | 2 bytes                               |
| `int32`  | numbers from -2147483648 to 2147483647                   | number             | 4 bytes                               |
| `int64`  | numbers from -9223372036854775808 to 9223372036854775807 | number             | 8 bytes                               |
| `Address`| 33 byte array                                            | base64             | 33 bytes                              |
| `Bytes`  | byte array                                               | base64             | uint32 length + bytes                 |
| `string` | string                                                   | string             | uint16 length + bytes                 |
| `[]T`    | for any `T` in the above list, serialized as an array    | array              | uint32 length + elements              |

