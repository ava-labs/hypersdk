import { expect, test } from 'vitest'
import { TransferAction } from './TransferAction'
import { Transaction } from '../chain/Transaction'
import { bytesToHex } from '@noble/hashes/utils'
import { cb58 } from '../lib/cb58'



export function idStringToBigInt(id: string): bigint {
    const bytes = cb58.decode(id);
    return BigInt(`0x${bytes.reduce((str, byte) => str + byte.toString(16).padStart(2, '0'), '')}`);
}

test('Transfer action bytes', () => {
    const action = new TransferAction(
        "morpheus1qrzvk4zlwj9zsacqgtufx7zvapd3quufqpxk5rsdd4633m4wz2fdjk97rwu",
        123n * (10n ** 9n)
    )

    expect(
        bytesToHex(action.toBytes())
    ).toBe(
        "0000c4cb545f748a28770042f893784ce85b107389004d6a0e0d6d7518eeae1292d90000001ca35f0e00"
    );
})



test('Tx with a transfer action digest', () => {
    const action = new TransferAction(
        "morpheus1qrzvk4zlwj9zsacqgtufx7zvapd3quufqpxk5rsdd4633m4wz2fdjk97rwu",
        123n * (10n ** 9n)
    )

    const chainId = idStringToBigInt("2c7iUW3kCDwRA9ZFd5bjZZc8iDy68uAsFSBahjqSZGttiTDSNH")

    const tx = new Transaction(
        1717111222000n,
        chainId,
        10n * (10n ** 9n),
        [action],
    )

    expect(
        bytesToHex(tx.digest())
    ).toBe(
        "0000018fcbcdeef0d36e467c73e2840140cc41b3d72f8a5a7446b2399c39b9c74d4cf077d250902400000002540be400010000c4cb545f748a28770042f893784ce85b107389004d6a0e0d6d7518eeae1292d90000001ca35f0e00"
    );
})