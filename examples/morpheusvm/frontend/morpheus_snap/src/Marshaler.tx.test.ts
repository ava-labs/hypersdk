import { bytesToHex } from '@noble/hashes/utils'
import { idStringToBigInt } from './cb58'
import { hexToBytes } from '@noble/curves/abstract/utils'
import { Marshaler } from "./Marshaler";
import { parseBech32 } from './bech32';
import { base64 } from '@scure/base';
import { signTransactionBytes, TransactionPayload } from './sign';

test('Empty transaction', () => {
  const chainId = idStringToBigInt("2c7iUW3kCDwRA9ZFd5bjZZc8iDy68uAsFSBahjqSZGttiTDSNH")

  const tx: TransactionPayload = {
    timestamp: "1717111222000",
    chainId: String(chainId),
    maxFee: String(10n * (10n ** 9n)),
    actions: [],
  }

  const marshaler = new Marshaler('[]')
  expect(bytesToHex(marshaler.getHash())).toBe(
    "4f53cda18c2baa0c0354bb5f9a3ecbe5ed12ab4d8e11ba873c2f11161202b945"
  )

  const txDigest = marshaler.encodeTransaction(tx)

  expect(
    bytesToHex(txDigest)
  ).toBe(
    "0000018fcbcdeef0" + "d36e467c73e2840140cc41b3d72f8a5a7446b2399c39b9c74d4cf077d2509024" + "00000002540be4" + "0000"
  );
})

test('Single action tx sign and marshal', async () => {
  const chainId = idStringToBigInt("2c7iUW3kCDwRA9ZFd5bjZZc8iDy68uAsFSBahjqSZGttiTDSNH");
  const addrString = "morpheus1qqds2l0ryq5hc2ddps04384zz6rfeuvn3kyvn77hp4n5sv3ahuh6wgkt57y";
  const abi = `[{
        "id": 0,
        "name": "Transfer",
        "types": {
          "Transfer": [
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
}]`//TODO: replace with actual abi

  const actionData = {
    actionName: "Transfer",
    data: {
      to: addrString,
      value: "123",
      memo: base64.encode(new TextEncoder().encode("memo")),
    }
  }

  const tx: TransactionPayload = {
    timestamp: "1717111222000",
    chainId: String(chainId),
    maxFee: String(10n * (10n ** 9n)),
    actions: [actionData],
  }

  const marshaler = new Marshaler(abi)
  const digest = marshaler.encodeTransaction(tx)
  expect(Buffer.from(digest).toString('hex')).toBe("0000018fcbcdeef0d36e467c73e2840140cc41b3d72f8a5a7446b2399c39b9c74d4cf077d250902400000002540be4000100001b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7000000000000007b000000046d656d6f");

  const privateKeyHex = "323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7";
  const privateKey = hexToBytes(privateKeyHex).slice(0, 32)

  const signedTxBytes = signTransactionBytes(digest, privateKey);

  expect(Buffer.from(signedTxBytes).toString('hex')).toBe("0000018fcbcdeef0d36e467c73e2840140cc41b3d72f8a5a7446b2399c39b9c74d4cf077d250902400000002540be4000100001b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7000000000000007b000000046d656d6f001b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa739bc9d7a4e74beafcb45cd8fe12200beeb4dac5569407426315eb382d8d11024c5f73200da58f24d8fe9467d86ec0a0c8c2ccb3c15d78e14fd93c66f4a73d802");
});