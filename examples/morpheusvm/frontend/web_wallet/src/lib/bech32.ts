import { bech32 } from '@scure/base';

// parseBech32 takes a bech32 address as input and returns the HRP and data
// section of a bech32 address.
export function parseBech32(addrStr: string): [string, Uint8Array] {
    const { prefix, words } = bech32.decode(addrStr);
    return [prefix, bech32.fromWords(words)];
}
