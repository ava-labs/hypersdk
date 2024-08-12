import { bech32 } from '@scure/base';
import { sha256 } from '@noble/hashes/sha256';

// parseBech32 takes a bech32 address as input and returns the HRP and data
// section of a bech32 address.
export function parseBech32(addrStr: string): [string, Uint8Array] {
    const { prefix, words } = bech32.decode(addrStr);
    return [prefix, bech32.fromWords(words)];
}

export const ED25519_AUTH_ID = 0x00

export function pubKeyToED25519Addr(pubKeyBytes: Uint8Array, hrp: string): string {
    const eip712Addr = new Uint8Array(33);
    eip712Addr[0] = ED25519_AUTH_ID;
    const pubKeyHash = sha256(pubKeyBytes);
    eip712Addr.set(pubKeyHash, 1);
    return bech32.encode(hrp, bech32.toWords(eip712Addr));
}