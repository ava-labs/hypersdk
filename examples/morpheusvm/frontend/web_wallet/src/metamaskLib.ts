import MetaMaskSDK, { SDKProvider } from "@metamask/sdk"

export namespace metamaskLib {
    let cachedProviderPromise: Promise<SDKProvider> | null = null;
    export async function getProvider(): Promise<SDKProvider> {
        if (!cachedProviderPromise) {
            cachedProviderPromise = (async () => {
                const metamaskSDK = new MetaMaskSDK();
                await metamaskSDK.connect();
                const provider = metamaskSDK.getProvider();
                if (!provider) {
                    throw new Error("Failed to get provider");
                }
                return provider;
            })();
        }
        return cachedProviderPromise;
    }


    type InvokeSnapParams = {
        method: string;
        params?: Record<string, unknown>;
    };


    type Snap = {
        permissionName: string;
        id: string;
        version: string;
        initialPermissions: Record<string, unknown>;
    };
    type GetSnapsResponse = Record<string, Snap>;

    export async function invokeSnap(provider: SDKProvider, { method, params }: InvokeSnapParams): Promise<unknown> {
        return await provider.request({
            method: 'wallet_invokeSnap',
            params: {
                snapId: "local:http://localhost:8080",
                request: params ? { method, params } : { method },
            },
        });
    }


    async function reinstallSnap() {
        const provider = await getProvider()

        await provider.request({
            method: 'wallet_requestSnaps',
            params: {
                ['local:http://localhost:8080']: {},
            },
        })

        const snaps = (await provider.request({
            method: 'wallet_getSnaps',
        })) as GetSnapsResponse;

        if (Object.keys(snaps).length > 0) {
        } else {
        }
    }

    async function connectWallet() {
        try {
            const provider = await getProvider()

            const providerversion = (await provider?.request({ method: "web3_clientVersion" })) as string || ""
            if (!providerversion.includes("flask")) {
                throw new Error("Please install MetaMask Flask!")
            }

            const snaps = (await provider.request({
                method: 'wallet_getSnaps',
            })) as GetSnapsResponse;

            if (Object.keys(snaps).length > 0) {
            } else {
                await reinstallSnap()
            }

            const pubKey = await invokeSnap(provider, { method: 'getPublicKey', params: { derivationPath: ["0'"], confirm: false } }) as string | undefined


            if (pubKey) {
                const address = Base58PubKeyToED25519Addr(pubKey)
                setMyAddress(address)
                onAddrChanged(address)
            } else {
                throw "Failed to get public key"
            }

        } catch (e) {
            const errorMessage = (e as Error).message || String(e);
        }
    }
}