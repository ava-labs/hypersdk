import { expect, test } from 'vitest'
import { TRANSFER_ACTION_ID, TransferAction } from './TransferAction'
import { bytesToHex } from '@noble/hashes/utils'

const TRANSFER_ACTION_ID_HEX = bytesToHex(Uint8Array.from([TRANSFER_ACTION_ID]))


test('Transfer action bytes - Zero value', () => {
    const action = new TransferAction(
        "morpheus1qqds2l0ryq5hc2ddps04384zz6rfeuvn3kyvn77hp4n5sv3ahuh6wgkt57y",
        0n,
        "test memo"
    )

    expect(
        //for simplicity, we concatenate the action id and the action bytes
        bytesToHex(action.toBytes())
    ).toBe(
        TRANSFER_ACTION_ID_HEX + "001b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa700000000000000000000000974657374206d656d6f"
    );
})

test('Transfer action bytes - Max uint64 value', () => {
    const action = new TransferAction(
        "morpheus1qqds2l0ryq5hc2ddps04384zz6rfeuvn3kyvn77hp4n5sv3ahuh6wgkt57y",
        18446744073709551615n, // Max uint64 value
        "another memo"
    )

    expect(
        bytesToHex(action.toBytes())
    ).toBe(
        TRANSFER_ACTION_ID_HEX + "001b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7ffffffffffffffff0000000c616e6f74686572206d656d6f"
    );
})

test('Transfer action bytes - Empty address', () => {
    const action = new TransferAction(
        "morpheus1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqxez33a",
        123n,
        "memo"
    )

    expect(
        bytesToHex(action.toBytes())
    ).toBe(
        TRANSFER_ACTION_ID_HEX + "000000000000000000000000000000000000000000000000000000000000000000000000000000007b000000046d656d6f"
    );
})

test('Transfer action bytes - Empty memo', () => {
    const action = new TransferAction(
        "morpheus1q8rc050907hx39vfejpawjydmwe6uujw0njx9s6skzdpp3cm2he5s036p07",
        456n,
        ""
    )

    expect(
        bytesToHex(action.toBytes())
    ).toBe(
        TRANSFER_ACTION_ID_HEX + "01c787d1e57fae689589cc83d7488ddbb3ae724e7ce462c350b09a10c71b55f34800000000000001c800000000"
    );
})