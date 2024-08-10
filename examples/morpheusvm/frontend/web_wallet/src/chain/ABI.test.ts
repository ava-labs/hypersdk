import { expect, test, describe, it } from 'vitest'
import { bytesToHex } from '@noble/hashes/utils'
import { parse } from 'lossless-json'

const abiString = `[
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
  }
]`

test.only('TestABISpec', () => {
  const abi = new ABI(abiString)
  expect(bytesToHex(abi.getHash()))
    .toBe("26d5faddb952328f5c11d96f814163f002ff45cdce436eb9c7426caf0633c24e")
})

test.only('TestMarshalEmptySpec', () => {
  const abi = new ABI(abiString)
  const jsonString = `
  {
    "Field1": 0
  }`
  const binary = abi.getActionBinary("MockActionSingleNumber", jsonString)
  expect(bytesToHex(binary))
    .toBe("0000")
})

test.only('TestMarshalSingleNumber', () => {
  const abi = new ABI(abiString)
  const jsonString = `
  {
    "Field1": 12333
  }`
  const binary = abi.getActionBinary("MockActionSingleNumber", jsonString)
  expect(bytesToHex(binary))
    .toBe("302d")
})

test.only('TestMarshalAllNumbersSpec', () => {
  const abi = new ABI(abiString)
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

describe.only('TestMarshalStringAndBytesSpec', () => {
  const abi = new ABI(abiString)
  const testCases = [
    {
      name: "Empty fields",
      jsonString: `{"field1": "","field2": ""}`,
      expectedDigest: "000000000000"
    },
    // {
    //   name: "String 'A' and empty bytes",
    //   jsonString: `{"field1": "A","field2": ""}`,
    //   expectedDigest: "00014100000000"
    // },
    // {
    //   name: "Byte 0x00 and empty string",
    //   jsonString: `{"field1": "","field2": "AA=="}`,
    //   expectedDigest: "00000000000100"
    // },
    // {
    //   name: "Non-empty fields",
    //   jsonString: `{"field1": "Hello, World!","field2": "AQIDBA=="}`,
    //   expectedDigest: "000d48656c6c6f2c20576f726c64210000000401020304"
    // },
  ]

  testCases.forEach(tc => {
    it(`Should marshal ${tc.name}`, () => {
      const binary = abi.getActionBinary("MockActionStringAndBytes", tc.jsonString)
      expect(bytesToHex(binary)).toBe(tc.expectedDigest)
    })
  })
})

test('TestMarshalArraysSpec', () => {
  const abi = new ABI(abiString)
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

import { sha256 } from '@noble/hashes/sha256';

type ABIField = {
  name: string
  type: string
}

type SingleActionABI = {
  id: number
  name: string
  types: Record<string, ABIField[]>
}

export class ABI {
  private abi: SingleActionABI[]

  constructor(private abiString: string) {
    this.abi = JSON.parse(abiString)
  }

  getHash(): Uint8Array {
    return sha256(this.abiString)
  }

  getActionBinary(actionName: string, dataJSON: string): Uint8Array {
    const data = parse(dataJSON) as Record<string, unknown>

    const actionABI = this.abi.find(abi => abi.name === actionName)
    if (!actionABI) throw new Error(`No action ABI found: ${actionName}`)

    const structABI = actionABI.types[actionName]
    if (!structABI) throw new Error(`No struct ${actionName} found in action ${actionName} ABI`)

    let resultingBinary = new Uint8Array()
    for (const field of structABI) {
      const value = data[field.name]
      const fieldBinary = encodeField(field.type, value)
      resultingBinary = new Uint8Array([...resultingBinary, ...fieldBinary])
    }

    return resultingBinary
  }
}

function encodeField(type: string, value: unknown): Uint8Array {
  console.warn("DEBUG: encodeField", type, value)

  if (type === '[]uint8' && typeof value === 'string') {
    value = Array.from(atob(value), char => char.charCodeAt(0));
  }

  if (type.startsWith('[]')) {
    return encodeArray(type.slice(2), value as unknown[]);
  }

  switch (type) {
    case "uint8":
    case "uint16":
    case "uint32":
    case "uint64":
    case "int8":
    case "int16":
    case "int32":
    case "int64":
      return encodeNumber(type, value as number | string)
    case "string":
      return encodeString(value as string)
    default:
      throw new Error(`Type ${type} marshaling is not implemented yet`)
  }
}

function encodeNumber(type: string, value: number | string): Uint8Array {
  const bigValue = BigInt(value)
  let buffer: ArrayBuffer
  let dataView: DataView

  switch (type) {
    case "uint8":
      buffer = new ArrayBuffer(1)
      dataView = new DataView(buffer)
      dataView.setUint8(0, Number(bigValue))
      break
    case "uint16":
      buffer = new ArrayBuffer(2)
      dataView = new DataView(buffer)
      dataView.setUint16(0, Number(bigValue), false)
      break
    case "uint32":
      buffer = new ArrayBuffer(4)
      dataView = new DataView(buffer)
      dataView.setUint32(0, Number(bigValue), false)
      break
    case "uint64":
      buffer = new ArrayBuffer(8)
      dataView = new DataView(buffer)
      dataView.setBigUint64(0, bigValue, false)
      break
    case "int8":
      buffer = new ArrayBuffer(1)
      dataView = new DataView(buffer)
      dataView.setInt8(0, Number(bigValue))
      break
    case "int16":
      buffer = new ArrayBuffer(2)
      dataView = new DataView(buffer)
      dataView.setInt16(0, Number(bigValue), false)
      break
    case "int32":
      buffer = new ArrayBuffer(4)
      dataView = new DataView(buffer)
      dataView.setInt32(0, Number(bigValue), false)
      break
    case "int64":
      buffer = new ArrayBuffer(8)
      dataView = new DataView(buffer)
      dataView.setBigInt64(0, bigValue, false)
      break
    default:
      throw new Error(`Unsupported number type: ${type}`)
  }

  return new Uint8Array(buffer)
}

function encodeString(value: string): Uint8Array {
  const encoder = new TextEncoder()
  const stringBytes = encoder.encode(value)
  const lengthBytes = encodeNumber("uint16", stringBytes.length)
  return new Uint8Array([...lengthBytes, ...stringBytes])
}

function encodeArray(type: string, value: unknown[]): Uint8Array {
  if (!Array.isArray(value)) {
    throw new Error(`Error in encodeArray: Expected an array for type ${type}, but received ${typeof value} of declared type ${type}`)
  }

  const lengthBytes = encodeNumber("uint16", value.length);
  const encodedItems = value.map(item => encodeField(type, item));
  const flattenedItems = encodedItems.reduce((acc, item) => {
    if (item instanceof Uint8Array) {
      return [...acc, ...item];
    } else if (typeof item === 'number') {
      return [...acc, item];
    } else {
      throw new Error(`Unexpected item type in encoded array: ${typeof item}`);
    }
  }, [] as number[]);
  return new Uint8Array([...lengthBytes, ...flattenedItems]);
}