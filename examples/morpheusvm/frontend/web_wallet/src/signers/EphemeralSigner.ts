import { ed25519 } from "@noble/curves/ed25519";
import { PrivateKeySigner } from "./PrivateKeySigner";

export class EphemeralSigner extends PrivateKeySigner {
    constructor() {
        super(ed25519.utils.randomPrivateKey());
    }
}