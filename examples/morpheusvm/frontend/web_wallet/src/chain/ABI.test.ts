import { expect, test } from 'vitest'
import { Transaction } from './Transaction'
import { bytesToHex } from '@noble/hashes/utils'
import { idStringToBigInt } from '../lib/cb58'
import { ABI } from './ABI'

test('ABI Specs', () => {
    const abiString = `[
  {
    "id": 1,
    "name": "MockAction1",
    "types": {
      "MockAction1": [
        {
          "name": "Field1",
          "type": "string"
        },
        {
          "name": "Field2",
          "type": "int32"
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

    const abi = new ABI(abiString)
    expect(bytesToHex(abi.getHash())).toBe(
        "404e365ee910729071642fa843076186ad6001a6314cde7c0ae5f1355f90ab8e"
    )
})
