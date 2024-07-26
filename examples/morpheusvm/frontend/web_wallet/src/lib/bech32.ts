import { bech32 } from '@scure/base';
import { ED25519_AUTH_ID } from '../signers/SignerIface';
import { sha256 } from '@noble/hashes/sha256';
import { HRP } from '../const';

// parseBech32 takes a bech32 address as input and returns the HRP and data
// section of a bech32 address.
export function parseBech32(addrStr: string): [string, Uint8Array] {
    const { prefix, words } = bech32.decode(addrStr);
    return [prefix, bech32.fromWords(words)];
}

export function pubKeyToED25519Addr(pubKeyBytes: Uint8Array): string {
    const eip712Addr = new Uint8Array(33);
    eip712Addr[0] = ED25519_AUTH_ID;
    const pubKeyHash = sha256(pubKeyBytes);
    eip712Addr.set(pubKeyHash, 1);
    return bech32.encode(HRP, bech32.toWords(eip712Addr));
}