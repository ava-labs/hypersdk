import { bytesToHex } from '@noble/hashes/utils'
import { idStringToBigInt } from './cb58'

import { Marshaler, Transaction } from "./Marshaler";

test('Empty transaction', () => {
    const chainId = idStringToBigInt("2c7iUW3kCDwRA9ZFd5bjZZc8iDy68uAsFSBahjqSZGttiTDSNH")

    const tx: Transaction = {
        timestamp: "1717111222000",
        chainId: String(chainId),
        maxFee: String(10n * (10n ** 9n)),
        actions: [],
    }

    const marshaler = new Marshaler('{}')
    expect(bytesToHex(marshaler.getHash())).toBe(
        "44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a"
    )

    const txDigest = marshaler.encodeTransaction(tx)

    expect(
        bytesToHex(txDigest)
    ).toBe(
        "0000018fcbcdeef0" + "d36e467c73e2840140cc41b3d72f8a5a7446b2399c39b9c74d4cf077d2509024" + "00000002540be4" + "0000"
    );
})
