export type txBase = {
    chainId: string,
    expiration: string,
    maxFee: string,
}

type transferData = {
    to: string,
    amount: string,
}

export type transferParams = {
    base: txBase,
    data: transferData,
}

export function transferDigest(params: transferParams): Uint8Array {
    throw new Error("Not implemented")
}