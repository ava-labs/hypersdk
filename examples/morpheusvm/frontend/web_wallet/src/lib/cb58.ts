import { base58 } from '@scure/base';
import { sha256 } from '@noble/hashes/sha256';

export const cb58 = {
    encode(data: Uint8Array): string {
        return base58.encode(new Uint8Array([...data, ...sha256(data).subarray(-4)]));
    },
    decode(string: string): Uint8Array {
        return base58.decode(string).subarray(0, -4);
    },
};