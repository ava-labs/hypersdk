export interface WalletIface { }

import { ed25519 } from '@noble/curves/ed25519';

export class EphemeralWallet implements WalletIface {
    private privateKey: Uint8Array;
    constructor() {
        this.privateKey = ed25519.utils.randomPrivateKey();
    }
}


export class MetamaskSnapWallet implements WalletIface {
    constructor(private snapId: string) {

    }
}