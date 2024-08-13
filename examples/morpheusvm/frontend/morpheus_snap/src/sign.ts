import { ED25519_AUTH_ID } from './bech32';
import { ed25519 } from "@noble/curves/ed25519";

export type ActionData = {
    actionName: string
    data: Record<string, unknown>
}

export type TransactionPayload = {
    timestamp: string
    chainId: string
    maxFee: string
    actions: ActionData[]
}

export function signTransactionBytes(txBytes: Uint8Array, privateKey: Uint8Array): Uint8Array {
    if (privateKey.length !== 32) throw new Error('Invalid private key: must be 32 bytes long')
    const signature = ed25519.sign(txBytes, privateKey)
    const pubKey = ed25519.getPublicKey(privateKey)
    return new Uint8Array([...txBytes, ED25519_AUTH_ID, ...pubKey, ...signature])
}