import { ed25519 } from "@noble/curves/ed25519";
import { ED25519_AUTH_ID, SignerIface } from "./SignerIface";

export class PrivateKeySigner implements SignerIface {

    constructor(private privateKey: Uint8Array) {
    }
    getAuthId(): number {
        return ED25519_AUTH_ID
    }

    async signTx(binary: Uint8Array): Promise<Uint8Array> {
        return ed25519.sign(binary, this.privateKey);
    }

    getPublicKey(): Uint8Array {
        return ed25519.getPublicKey(this.privateKey);
    }
}
