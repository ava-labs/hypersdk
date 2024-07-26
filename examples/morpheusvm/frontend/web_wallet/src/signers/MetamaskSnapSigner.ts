import MetaMaskSDK, { SDKProvider } from "@metamask/sdk"
import { ED25519_AUTH_ID, SignerIface } from "./SignerIface";
import { DEVELOPMENT_MODE, SNAP_ID } from "../const";
import { base58 } from '@scure/base';

type InvokeSnapParams = {
    method: string;
    params?: Record<string, unknown>;
};

export class MetamaskSnapSigner implements SignerIface {
    private cachedPublicKey: Uint8Array | null = null;

    constructor(private snapId: string) {

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

    async signTx(binary: Uint8Array): Promise<Uint8Array> {
        const sig58 = await this._invokeSnap({ method: 'signTransaction', params: { binary } }) as string | undefined;
        if (!sig58) {
            throw new Error("Failed to sign transaction");
        }
        return base58.decode(sig58);
    }

    private providerPromise: Promise<SDKProvider> | null = null;
    async getProvider(): Promise<SDKProvider> {
        if (!this.providerPromise) {
            this.providerPromise = (async () => {
                const metamaskSDK = new MetaMaskSDK();
                await metamaskSDK.connect();
                const provider = metamaskSDK.getProvider();
                if (!provider) {
                    throw new Error("Failed to get provider");
                }
                return provider;
            })();
        }
        return this.providerPromise;
    }

    async connect() {
        const provider = await this.getProvider();

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

        const pubKey = await this._invokeSnap({ method: 'getPublicKey', params: { derivationPath: ["0'"], confirm: false } }) as string | undefined;

        if (!pubKey) {
            throw new Error("Failed to get public key");
        }

        this.cachedPublicKey = base58.decode(pubKey);
    }

    async reinstallSnap() {
        const provider = await this.getProvider();

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
        const provider = await this.getProvider();
        return await provider.request({
            method: 'wallet_invokeSnap',
            params: {
                snapId: this.snapId,
                request: params ? { method, params } : { method },
            },
        });
    }
}
