import { sha256 } from '@noble/hashes/sha256';

export class ABI {
    constructor(private abi: string) { }

    getHash(): Uint8Array {
        return sha256(this.abi)
    }
}