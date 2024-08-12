import { expect } from '@jest/globals';
import { bytesToHex } from '@noble/hashes/utils'
import { Marshaler } from './Marshaler'

export const abiString = `[
  {
    "id": 1,
    "name": "MockActionSingleNumber",
    "types": {
      "MockActionSingleNumber": [
        {
          "name": "Field1",
          "type": "uint16"
        }
      ]
    }
  },
  {
    "id": 2,
    "name": "MockActionTransfer",
    "types": {
      "MockActionTransfer": [
        {
          "name": "to",
          "type": "Address"
        },
        {
          "name": "value",
          "type": "uint64"
        },
        {
          "name": "memo",
          "type": "[]uint8"
        }
      ]
    }
  },
  {
    "id": 3,
    "name": "MockActionAllNumbers",
    "types": {
      "MockActionAllNumbers": [
        {
          "name": "uint8",
          "type": "uint8"
        },
        {
          "name": "uint16",
          "type": "uint16"
        },
        {
          "name": "uint32",
          "type": "uint32"
        },
        {
          "name": "uint64",
          "type": "uint64"
        },
        {
          "name": "int8",
          "type": "int8"
        },
        {
          "name": "int16",
          "type": "int16"
        },
        {
          "name": "int32",
          "type": "int32"
        },
        {
          "name": "int64",
          "type": "int64"
        }
      ]
    }
  },
  {
    "id": 4,
    "name": "MockActionStringAndBytes",
    "types": {
      "MockActionStringAndBytes": [
        {
          "name": "field1",
          "type": "string"
        },
        {
          "name": "field2",
          "type": "[]uint8"
        }
      ]
    }
  },
  {
    "id": 5,
    "name": "MockActionArrays",
    "types": {
      "MockActionArrays": [
        {
          "name": "strings",
          "type": "[]string"
        },
        {
          "name": "bytes",
          "type": "[][]uint8"
        },
        {
          "name": "uint8s",
          "type": "[]uint8"
        },
        {
          "name": "uint16s",
          "type": "[]uint16"
        },
        {
          "name": "uint32s",
          "type": "[]uint32"
        },
        {
          "name": "uint64s",
          "type": "[]uint64"
        },
        {
          "name": "int8s",
          "type": "[]int8"
        },
        {
          "name": "int16s",
          "type": "[]int16"
        },
        {
          "name": "int32s",
          "type": "[]int32"
        },
        {
          "name": "int64s",
          "type": "[]int64"
        }
      ]
    }
  },
  {
    "id": 7,
    "name": "MockActionWithTransferArray",
    "types": {
      "MockActionTransfer": [
        {
          "name": "to",
          "type": "Address"
        },
        {
          "name": "value",
          "type": "uint64"
        },
        {
          "name": "memo",
          "type": "[]uint8"
        }
      ],
      "MockActionWithTransferArray": [
        {
          "name": "transfers",
          "type": "[]MockActionTransfer"
        }
      ]
    }
  },
  {
    "id": 6,
    "name": "MockActionWithTransfer",
    "types": {
      "MockActionTransfer": [
        {
          "name": "to",
          "type": "Address"
        },
        {
          "name": "value",
          "type": "uint64"
        },
        {
          "name": "memo",
          "type": "[]uint8"
        }
      ],
      "MockActionWithTransfer": [
        {
          "name": "transfer",
          "type": "MockActionTransfer"
        }
      ]
    }
  },
  {
    "id": 8,
    "name": "MockActionWithTransferMap",
    "types": {
      "MockActionWithTransferMap": [
        {
          "name": "transfersMap",
          "type": ""
        }
      ]
    }
  }
]`

test('TestABISpec', () => {
  const abi = new Marshaler(abiString)
  expect(bytesToHex(abi.getHash()))
    .toBe("1b11cc4be907c26b3aa8346c07866a86ec685f63198ccbe27937a4372c49a4bd")
})

test('TestMarshalEmptySpec', () => {
  const abi = new Marshaler(abiString)
  const jsonString = `
  {
    "Field1": 0
  }`
  const binary = abi.getActionBinary("MockActionSingleNumber", jsonString)
  expect(bytesToHex(binary))
    .toBe("0000")
})

test('TestMarshalSingleNumber', () => {
  const abi = new Marshaler(abiString)
  const jsonString = `
  {
    "Field1": 12333
  }`
  const binary = abi.getActionBinary("MockActionSingleNumber", jsonString)
  expect(bytesToHex(binary))
    .toBe("302d")
})

test('TestMarshalAllNumbersSpec', () => {
  const abi = new Marshaler(abiString)
  const jsonString = `
  {
		"uint8": 254,
		"uint16": 65534,
		"uint32": 4294967294,
		"uint64": 18446744073709551614,
		"int8": -127,
		"int16": -32767,
		"int32": -2147483647,
		"int64": -9223372036854775807
	}`
  const binary = abi.getActionBinary("MockActionAllNumbers", jsonString)
  expect(bytesToHex(binary))
    .toBe("fefffefffffffefffffffffffffffe818001800000018000000000000001")
})

describe('TestMarshalStringAndBytesSpec', () => {
  const abi = new Marshaler(abiString)
  const testCases = [
    {
      name: "Empty fields",
      jsonString: `{"field1": "","field2": ""}`,
      expectedDigest: "000000000000"
    },
    {
      name: "String 'A' and empty bytes",
      jsonString: `{"field1": "A","field2": ""}`,
      expectedDigest: "00014100000000"
    },
    {
      name: "Byte 0x00 and empty string",
      jsonString: `{"field1": "","field2": "AA=="}`,
      expectedDigest: "00000000000100"
    },
    {
      name: "Non-empty fields",
      jsonString: `{"field1": "Hello, World!","field2": "AQIDBA=="}`,
      expectedDigest: "000d48656c6c6f2c20576f726c64210000000401020304"
    },
  ]

  testCases.forEach(tc => {
    it(`Should marshal ${tc.name}`, () => {
      const binary = abi.getActionBinary("MockActionStringAndBytes", tc.jsonString)
      expect(bytesToHex(binary)).toBe(tc.expectedDigest)
    })
  })
})

test('TestMarshalArraysSpec', () => {
  const abi = new Marshaler(abiString)
  const jsonString = `
  {
    "strings": ["Hello", "World"],
    "bytes": ["AQI=", "AwQ="],
    "uint8s": "AQI=",
    "uint16s": [300, 400],
    "uint32s": [70000, 80000],
    "uint64s": [5000000000, 6000000000],
    "int8s": [-1, -2],
    "int16s": [-300, -400],
    "int32s": [-70000, -80000],
    "int64s": [-5000000000, -6000000000]
  }`
  const binary = abi.getActionBinary("MockActionArrays", jsonString)
  expect(bytesToHex(binary))
    .toBe("0002000548656c6c6f0005576f726c6400020000000201020000000203040000000201020002012c0190000200011170000138800002000000012a05f2000000000165a0bc000002fffe0002fed4fe700002fffeee90fffec7800002fffffffed5fa0e00fffffffe9a5f4400")
})



test('TestMarshalTransferSpec - bech32', () => {
  const abi = new Marshaler(abiString)
  const jsonString = `
  {
    "to": "morpheus1qypqxpq9qcrsszg2pvxq6rs0zqg3yyc5qqqqqqqqqqqqqqqqqqqqqmqvs7e",
    "value": 1000,
    "memo": "AQID"
  }`
  const binary = abi.getActionBinary("MockActionTransfer", jsonString)
  expect(bytesToHex(binary))
    .toBe("0102030405060708090a0b0c0d0e0f10111213140000000000000000000000000000000000000003e800000003010203")
})


test('TestMarshalComplexStructs', () => {
  const abi = new Marshaler(abiString)

  // Struct with a single transfer
  const jsonStringSingleTransfer = `
  {
    "transfer": {
      "to": "AQIDBAUGBwgJCgsMDQ4PEBESExQAAAAAAAAAAAAAAAAA",
      "value": 1000,
      "memo": "AQID"
    }
  }`
  const binarySingleTransfer = abi.getActionBinary("MockActionWithTransfer", jsonStringSingleTransfer)
  expect(bytesToHex(binarySingleTransfer))
    .toBe("0102030405060708090a0b0c0d0e0f10111213140000000000000000000000000000000000000003e800000003010203")

  // Struct with an array of transfers
  const jsonStringTransferArray = `
  {
    "transfers": [
      {
        "to": "AQIDBAUGBwgJCgsMDQ4PEBESExQAAAAAAAAAAAAAAAAA",
        "value": 1000,
        "memo": "AQID"
      },
      {
        "to": "AQIDBAUGBwgJCgsMDQ4PEBESExQAAAAAAAAAAAAAAAAA",
        "value": 1000,
        "memo": "AQID"
      }
    ]
  }`
  const binaryTransferArray = abi.getActionBinary("MockActionWithTransferArray", jsonStringTransferArray)
  expect(bytesToHex(binaryTransferArray))
    .toBe("00020102030405060708090a0b0c0d0e0f10111213140000000000000000000000000000000000000003e8000000030102030102030405060708090a0b0c0d0e0f10111213140000000000000000000000000000000000000003e800000003010203")

  //TODO: add support for maps
  // // Struct with a map of transfers
  // const jsonStringTransferMap = `
  // {
  //   "transfersMap": {
  //     "first": {
  //       "to": "AQIDBAUGBwgJCgsMDQ4PEBESExQAAAAAAAAAAAAAAAAA",
  //       "value": 1000,
  //       "memo": "AQID"
  //     },
  //     "second": {
  //       "to": "AQIDBAUGBwgJCgsMDQ4PEBESExQAAAAAAAAAAAAAAAAA",
  //       "value": 1000,
  //       "memo": "AQID"
  //     }
  //   }
  // }`
  // const binaryTransferMap = abi.getActionBinary("MockActionWithTransferMap", jsonStringTransferMap)
  // expect(bytesToHex(binaryTransferMap))
  //   .toBe("0002000566697273740102030405060708090a0b0c0d0e0f10111213140000000000000000000000000000000000000003e80000000301020300067365636f6e640102030405060708090a0b0c0d0e0f10111213140000000000000000000000000000000000000003e800000003010203")
})

//TODO: test uint256 packing