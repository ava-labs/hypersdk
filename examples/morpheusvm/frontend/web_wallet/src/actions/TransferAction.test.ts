import { expect, test } from 'vitest'
import { TRANSFER_ACTION_ID, TransferAction } from './TransferAction'
import { bytesToHex } from '@noble/hashes/utils'
import { idStringToBigInt } from '../../../morpheus_snap/src/cb58'
import { Transaction } from '../chain/Transaction'
import { hexToBytes } from '@noble/curves/abstract/utils'
import { PrivateKeySigner } from '../signers/PrivateKeySigner'

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


test('Single action tx sign and marshal', async () => {
    const chainId = idStringToBigInt("2c7iUW3kCDwRA9ZFd5bjZZc8iDy68uAsFSBahjqSZGttiTDSNH");
    const addr1 = "morpheus1qqds2l0ryq5hc2ddps04384zz6rfeuvn3kyvn77hp4n5sv3ahuh6wgkt57y";



    const action = new TransferAction(addr1, 123n, "memo");

    const tx = new Transaction(
        1717111222000n,
        chainId,
        10n * (10n ** 9n),
        [action],
    );

    const digest = await tx.digest();
    expect(Buffer.from(digest).toString('hex')).toBe("0000018fcbcdeef0d36e467c73e2840140cc41b3d72f8a5a7446b2399c39b9c74d4cf077d250902400000002540be4000100001b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7000000000000007b000000046d656d6f");

    const privateKeyHex = "323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7";
    const privateKey = hexToBytes(privateKeyHex);
    const signer = new PrivateKeySigner(privateKey.slice(0, 32));

    const signedTxBytes = await tx.sign(signer);

    expect(Buffer.from(signedTxBytes).toString('hex')).toBe("0000018fcbcdeef0d36e467c73e2840140cc41b3d72f8a5a7446b2399c39b9c74d4cf077d250902400000002540be4000100001b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7000000000000007b000000046d656d6f001b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa739bc9d7a4e74beafcb45cd8fe12200beeb4dac5569407426315eb382d8d11024c5f73200da58f24d8fe9467d86ec0a0c8c2ccb3c15d78e14fd93c66f4a73d802");
});