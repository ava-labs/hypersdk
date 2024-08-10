import { expect, test } from 'vitest'
import { bytesToHex } from '@noble/hashes/utils'
import { ABI } from './ABI'

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
  }
]`

//Test names represent test names in HyperSDK
test('TestABISpec', () => {
  const abi = new ABI(abiString)
  expect(bytesToHex(abi.getHash()))
    .toBe("a92c32c95198ea6539871193f2f45187d89378de0c6bd0095f5ff3b79557f34e")
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