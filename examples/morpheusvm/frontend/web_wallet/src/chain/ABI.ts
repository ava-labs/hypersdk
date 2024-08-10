import { sha256 } from '@noble/hashes/sha256';

type ABIField = {
    name: string
    type: string
}

type SingleActionABI = {
    id: number
    name: string
    types: Record<string, ABIField[]>
}

export class ABI {
    private abi: SingleActionABI[]

    constructor(private abiString: string) {
        this.abi = JSON.parse(abiString)
    }

    getHash(): Uint8Array {
        return sha256(this.abiString)
    }

    getActionBinary(actionName: string, data: Record<string, unknown>): Uint8Array {
        const actionABI = this.abi.find(abi => abi.name === actionName)
        if (!actionABI) throw new Error(`No action ABI found: ${actionName}`)

        const structABI = actionABI.types[actionName]
        if (!structABI) throw new Error(`No struct ${actionName} found in action ${actionName} ABI`)

        let resultingBinary = new Uint8Array()
        for (const field of structABI) {
            const value = data[field.name]
            const fieldBinary = encodeField(field.type, value)
            resultingBinary = new Uint8Array([...resultingBinary, ...fieldBinary])
        }

        return resultingBinary
    }
}

function encodeField(type: string, value: unknown): Uint8Array {
    if (type === "uint16") {
        const bigValue = BigInt(value as string)
        const buffer = new ArrayBuffer(2)
        new DataView(buffer).setUint16(0, Number(bigValue), false)
        return new Uint8Array(buffer)
    } else {
        throw new Error(`Type ${type} marshaling is not implemented yet`)
    }
}