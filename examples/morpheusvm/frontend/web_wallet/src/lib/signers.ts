import { TransactionPayload, signTransactionBytes } from "../../../morpheus_snap/src/sign"
import { Marshaler } from "../../../morpheus_snap/src/Marshaler"

export interface SignerIface {
    signTx(txPayload: TransactionPayload, abiString: string): Promise<Uint8Array>
    getPublicKey(): Uint8Array
}

export class PrivateKeySigner implements SignerIface {
    constructor(private privateKey: Uint8Array) {
        if (this.privateKey.length !== 32) {
            throw new Error("Private key must be 32 bytes");
        }
    }

    async signTx(txPayload: TransactionPayload, abiString: string): Promise<Uint8Array> {
        const marshaler = new Marshaler(abiString);
        const digest = marshaler.encodeTransaction(txPayload);
        const signedTxBytes = signTransactionBytes(digest, this.privateKey);
        return signedTxBytes;
    }

    getPublicKey(): Uint8Array {
        return ed25519.getPublicKey(this.privateKey);
    }
}

export class EphemeralSigner extends PrivateKeySigner {
    constructor() {
        super(ed25519.utils.randomPrivateKey());
    }
}

import MetaMaskSDK, { SDKProvider } from "@metamask/sdk"
import { DEVELOPMENT_MODE, SNAP_ID } from "../const";
import { base58 } from '@scure/base';
import { ed25519 } from "@noble/curves/ed25519";
import { ED25519_AUTH_ID } from "../../../morpheus_snap/src/bech32";

type InvokeSnapParams = {
    method: string;
    params?: Record<string, unknown>;
};

let cachedProvider: SDKProvider | null = null;
async function getProvider(): Promise<SDKProvider> {
    if (!cachedProvider) {
        const metamaskSDK = new MetaMaskSDK();
        await metamaskSDK.connect();
        const provider = metamaskSDK.getProvider();
        if (!provider) {
            throw new Error("Failed to get provider");
        }
        cachedProvider = provider;
    }
    return cachedProvider;
}

export class MetamaskSnapSigner implements SignerIface {
    private cachedPublicKey: Uint8Array | null = null;

    constructor(private snapId: string, private lastDerivationSection: number = 0) {

    }

    getAuthId(): number {
        return ED25519_AUTH_ID
    }

    getPublicKey(): Uint8Array {
        if (!this.cachedPublicKey) {
            throw new Error("Public key not cached. Please call connect() first.");
        }
        return this.cachedPublicKey;
    }

    async signTx(txPayload: TransactionPayload, abiString: string): Promise<Uint8Array> {
        const sig58 = await this._invokeSnap({
            method: 'signBytes', params: {
                derivationPath: [`${this.lastDerivationSection}'`],
                abiString: abiString,
                tx: txPayload,
            }
        }) as string | undefined;
        if (!sig58) {
            throw new Error("Failed to sign transaction");
        }
        return base58.decode(sig58);
    }

    async connect() {
        const provider = await getProvider();

        const providerVersion = (await provider?.request({ method: "web3_clientVersion" })) as string || "";
        if (!providerVersion.includes("flask")) {
            throw new Error("Your client is not compatible with development snaps. Please install MetaMask Flask!");
        }

        const snaps = (await provider.request({
            method: 'wallet_getSnaps',
        })) as Record<string, unknown>;

        if (!Object.keys(snaps).includes(SNAP_ID) || DEVELOPMENT_MODE) {
            await this.reinstallSnap();
        }

        const pubKey = await this._invokeSnap({
            method: 'getPublicKey',
            params: {
                derivationPath: [`${this.lastDerivationSection}'`]
            }
        }) as string | undefined;

        if (!pubKey) {
            throw new Error("Failed to get public key");
        }

        this.cachedPublicKey = base58.decode(pubKey);
    }

    async reinstallSnap() {
        const provider = await getProvider();

        console.log('Installing snap...');
        await provider.request({
            method: 'wallet_requestSnaps',
            params: {
                [SNAP_ID]: {},
            },
        });
        console.log('Snap installed');

        const snaps = (await provider.request({
            method: 'wallet_getSnaps',
        })) as Record<string, unknown>;

        if (Object.keys(snaps).includes(SNAP_ID)) {
            console.log('Snap installed successfully');
        } else {
            console.error('Snap not installed');
            throw new Error('Failed to install snap');
        }
    }

    private async _invokeSnap({ method, params }: InvokeSnapParams): Promise<unknown> {
        const provider = await getProvider();
        return await provider.request({
            method: 'wallet_invokeSnap',
            params: {
                snapId: this.snapId,
                request: params ? { method, params } : { method },
            },
        });
    }
}
