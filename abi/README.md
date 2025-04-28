# ABI Package

## Overview
The ABI package provides functionality for marshaling and unmarshaling actions.
It uses the [Canoto](https://github.com/StephenButtolph/canoto) serialization
protocol and is designed to work across different language implementations.

## ABI Format
The ABI is defined in JSON format, as shown in the `abi.json` file:
```json
{
    "actionsSpec": [
        {
            "name": "TestAction",
            "fields": [
                {
                    "fieldNumber": 1,
                    "name": "NumComputeUnits",
                    "typeUint": 4
                },
                
            ]
        }
    ],
    "outputsSpec": [
        {
            "name": "TestOutput",
            "fields": [
                {
                    "fieldNumber": 1,
                    "name": "Bytes",
                    "typeBytes": true
                },
            ]
        }
    ],
    "actionTypes": [
        {
            "name": "TestAction",
            "id": 0
        }
    ],
    "outputTypes": [
        {
            "name": "TestOutput",
            "id": 0
        }
    ]
}

```

The ABI consists of the following sections:
- `actionsSpec`: a list of action definitions, according to the Canoto specification 
- `outputsSpec`: a list of output definitions, according to the Canoto specification
- `actionTypes`: a dictionary of all available actions and their corresponding typeIDs
- `outputTypes`: a dictionary of all available outputs and their corresponding typeIDs

The type in an action/output field must be a [supported canoto type](https://github.com/StephenButtolph/canoto?tab=readme-ov-file#supported-types).

## ABI Verification
Frontends can use the ABI to display proper action and field names. For a wallet to verify it knows what it's signing, it must ensure that a canonical hash of the ABI is included in the message it signs.

A correct VM will verify the signature against the same ABI hash, such that verification fails if the wallet signed an action against a different than expected ABI.

This enables frontends to provide a verifiable display of what they are asking users to sign.
