import { expect, test } from 'vitest'
import { bytesToHex } from '@noble/hashes/utils'

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
  }
]`

test('TestABISpec', () => {
  const abi = new ABI(abiString)
  expect(bytesToHex(abi.getHash()))
    .toBe("269a24f3157d1af5403d2f21313f2b3d48232a282c51c9cea1457322f2887358")
})

test('TestMarshalEmptySpec', () => {
  const abi = new ABI(abiString)
  const data = {
    "Field1": 0
  }
  const binary = abi.getActionBinary("MockActionSingleNumber", data)
  expect(bytesToHex(binary))
    .toBe("0000")
})

test('TestMarshalSingleNumber', () => {
  const abi = new ABI(abiString)
  const data = {
    "Field1": 12333
  }
  const binary = abi.getActionBinary("MockActionSingleNumber", data)
  expect(bytesToHex(binary))
    .toBe("302d")
})

test('TestMarshalAllNumbersSpec', () => {
  const abi = new ABI(abiString)
  const data = {
    "uint8": 254,
    "uint16": 65534,
    "uint32": 4294967294,
    "uint64": "18446744073709551614",
    "int8": -127,
    "int16": -32767,
    "int32": -2147483647,
    "int64": "-9223372036854775807"
  }
  const binary = abi.getActionBinary("MockActionAllNumbers", data)
  expect(bytesToHex(binary))
    .toBe("fefffefffffffefffffffffffffffe818001800000018000000000000001")
})

test('TestMarshalStringAndBytesSpec', () => {
  const abi = new ABI(abiString)
  const data = {
    "field1": "Hello, World!",
    "field2": new Uint8Array([0x01, 0x02, 0x03, 0x04])
  }
  const binary = abi.getActionBinary("MockActionStringAndBytes", data)
  expect(bytesToHex(binary))
    .toBe("000d48656c6c6f2c20576f726c64210000000401020304")
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

  getActionBinary(actionName: string, data: Record<string, unknown>): Uint8Array {
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
    case "[]uint8":
      return encodeByteArray(value as Uint8Array)
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

function encodeByteArray(value: Uint8Array): Uint8Array {
  const lengthBytes = encodeNumber("uint32", value.length)
  return new Uint8Array([...lengthBytes, ...value])
}