
export interface SignerIface {
    signTransfer(params: transferParams): Promise<Uint8Array>
    getPublicKey(): Uint8Array
}

import { ed25519 } from '@noble/curves/ed25519';
import { transferDigest, transferParams } from './Transfer';

export class EphemeralSigner implements SignerIface {
    private privateKey: Uint8Array;
    constructor() {
        this.privateKey = ed25519.utils.randomPrivateKey();
    }

    async signTransfer(params: transferParams): Promise<Uint8Array> {
        const digest: Uint8Array = transferDigest(params);
        const signature = ed25519.sign(digest, this.privateKey);
        return signature;
    }

    getPublicKey(): Uint8Array {
        return ed25519.getPublicKey(this.privateKey);
    }
}
