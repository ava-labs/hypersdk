
export interface SignerIface {
    signTx(binary: Uint8Array): Promise<Uint8Array>
    getPublicKey(): Uint8Array
    getAuthId(): number
}

export const ED25519_AUTH_ID = 0x00