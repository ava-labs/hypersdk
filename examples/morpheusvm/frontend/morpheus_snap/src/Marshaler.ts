
import { sha256 } from '@noble/hashes/sha256';
import { parse } from 'lossless-json'
import { TransactionPayload } from './sign';
import { parseBech32 } from './bech32';
import { base64 } from '@scure/base';


export type SingleActionABI = {
    id: number
    name: string
    types: Record<string, {
        name: string
        type: string
    }[]>
}



export class Marshaler {
    private abi: SingleActionABI[]

    constructor(private abiString: string) {
        this.abi = JSON.parse(abiString)
        if (!Array.isArray(this.abi)) {
            throw new Error('Invalid ABI: ABI must be an array of single action ABIs')
        }
    }

    getHash(): Uint8Array {
        return sha256(this.abiString)
    }

    getActionBinary(actionName: string, dataJSON: string): Uint8Array {
        //todo: has to throw error of dataJSON has any extra fields
        const data = parse(dataJSON) as Record<string, unknown>

        return this.encodeField(actionName, data)
    }

    encodeTransaction(tx: TransactionPayload): Uint8Array {
        if (tx.timestamp.slice(-3) !== "000") {
            tx.timestamp = String(Math.floor(parseInt(tx.timestamp) / 1000) * 1000)
        }

        const timestampBytes = encodeNumber("uint64", tx.timestamp);
        const chainIdBytes = encodeNumber("uint256", tx.chainId);
        const maxFeeBytes = encodeNumber("uint64", tx.maxFee);
        const actionsCountBytes = encodeNumber("uint8", tx.actions.length);

        let actionsBytes = new Uint8Array();
        for (const action of tx.actions) {
            const actionTypeIdBytes = encodeNumber("uint8", this.getActionTypeId(action.actionName));
            const actionDataBytes = this.encodeField(action.actionName, action.data);
            actionsBytes = new Uint8Array([...actionsBytes, ...actionTypeIdBytes, ...actionDataBytes]);
        }

        // const abiHashBytes = this.getHash()

        return new Uint8Array([
            // ...abiHashBytes //TODO: add abi hash to the end of the signable body of transaction
            ...timestampBytes,
            ...chainIdBytes,
            ...maxFeeBytes,
            ...actionsCountBytes,
            ...actionsBytes,
        ]);
    }

    private getActionTypeId(actionName: string): number {
        const actionABI = this.abi.find(abi => abi.name === actionName)
        if (!actionABI) throw new Error(`No action ABI found: ${actionName}`)
        return actionABI.id
    }

    private encodeField(type: string, value: unknown): Uint8Array {
        if (type === 'Address' && typeof value === 'string') {
            return encodeAddress(value)
        }

        if (type === '[]uint8' && typeof value === 'string') {
            const byteArray = Array.from(atob(value), char => char.charCodeAt(0)) as number[]
            return new Uint8Array([...encodeNumber("uint32", byteArray.length), ...byteArray])
        }

        if (type.startsWith('[]')) {
            return this.encodeArray(type.slice(2), value as unknown[]);
        }

        switch (type) {
            case "uint8":
            case "uint16":
            case "uint32":
            case "uint64":
            case "uint256":
            //TODO: implement uint128, int128, and int256 if needed
            case "int8":
            case "int16":
            case "int32":
            case "int64":
                return encodeNumber(type, value as number | string)
            case "string":
                return encodeString(value as string)
            default:
                {
                    const actionABI = this.abi.find(abi => abi.name === type)
                    if (!actionABI) throw new Error(`No action ABI found: ${type}`)

                    const structABI = actionABI.types[type]
                    if (!structABI) throw new Error(`No struct ${type} found in action ${type} ABI`)

                    const dataRecord = value as Record<string, unknown>;
                    let resultingBinary = new Uint8Array()
                    for (const field of structABI) {
                        const fieldBinary = this.encodeField(field.type, dataRecord[field.name]);
                        resultingBinary = new Uint8Array([...resultingBinary, ...fieldBinary])
                    }
                    return resultingBinary
                }

        }
    }

    private encodeArray(type: string, value: unknown[]): Uint8Array {
        if (!Array.isArray(value)) {
            throw new Error(`Error in encodeArray: Expected an array for type ${type}, but received ${typeof value} of declared type ${type}`)
        }

        const lengthBytes = encodeNumber("uint16", value.length);
        const encodedItems = value.map(item => this.encodeField(type, item));
        const flattenedItems = encodedItems.reduce((acc, item) => {
            if (item instanceof Uint8Array) {
                return [...acc, ...item];
            } else if (typeof item === 'number') {
                return [...acc, item];
            } else {
                throw new Error(`Unexpected item type in encoded array: ${typeof item}`);
            }
        }, [] as number[]);
        return new Uint8Array([...lengthBytes, ...flattenedItems]);
    }
}

function encodeAddress(value: string): Uint8Array {
    let decodedCount = 0

    let addrBytes: Uint8Array = new Uint8Array()

    //try as a normal bech32 address
    try {
        const [, decodedBytes] = parseBech32(value)
        addrBytes = decodedBytes
        decodedCount++
    } catch (e) {
    }

    //try as 33 byte base64 encoded address (golang would marshal as such)
    if (isValidBase64(value)) {
        const decoded = base64.decode(value);
        if (decoded.length === 33) {//doesn't throw
            addrBytes = decoded;
            decodedCount++;
        }
    }

    if (decodedCount > 1) {
        throw new Error(`Address must be either bech32 or base64 encoded, could be decoded as both. the result is ambiguous: ${value}`)
    } else if (decodedCount === 1) {
        return addrBytes
    } else {
        throw new Error(`Address must be either bech32 or base64 encoded, could be decoded as neither: ${value}`)
    }
}

function isValidBase64(str: string): boolean {
    try {
        const decoded = base64.decode(str);
        return decoded.length > 0 && /^[A-Za-z0-9+/]*={0,2}$/.test(str);
    } catch {
        return false;
    }
}

function encodeNumber(type: string, value: number | string): Uint8Array {
    let bigValue = BigInt(value)
    let buffer: ArrayBuffer
    let dataView: DataView

    switch (type) {
        case "uint8":
            buffer = new ArrayBuffer(1)
            dataView = new DataView(buffer)
            dataView.setUint8(0, Number(bigValue))
            break
        case "uint16":
            buffer = new ArrayBuffer(2)
            dataView = new DataView(buffer)
            dataView.setUint16(0, Number(bigValue), false)
            break
        case "uint32":
            buffer = new ArrayBuffer(4)
            dataView = new DataView(buffer)
            dataView.setUint32(0, Number(bigValue), false)
            break
        case "uint64":
            buffer = new ArrayBuffer(8)
            dataView = new DataView(buffer)
            dataView.setBigUint64(0, bigValue, false)
            break
        case "uint256":
            buffer = new ArrayBuffer(32)
            dataView = new DataView(buffer)
            for (let i = 0; i < 32; i++) {
                dataView.setUint8(31 - i, Number(bigValue & 255n))
                bigValue >>= 8n
            }
            break
        case "int8":
            buffer = new ArrayBuffer(1)
            dataView = new DataView(buffer)
            dataView.setInt8(0, Number(bigValue))
            break
        case "int16":
            buffer = new ArrayBuffer(2)
            dataView = new DataView(buffer)
            dataView.setInt16(0, Number(bigValue), false)
            break
        case "int32":
            buffer = new ArrayBuffer(4)
            dataView = new DataView(buffer)
            dataView.setInt32(0, Number(bigValue), false)
            break
        case "int64":
            buffer = new ArrayBuffer(8)
            dataView = new DataView(buffer)
            dataView.setBigInt64(0, bigValue, false)
            break
        default:
            throw new Error(`Unsupported number type: ${type}`)
    }

    return new Uint8Array(buffer)
}

function encodeString(value: string): Uint8Array {
    const encoder = new TextEncoder()
    const stringBytes = encoder.encode(value)
    const lengthBytes = encodeNumber("uint16", stringBytes.length)
    return new Uint8Array([...lengthBytes, ...stringBytes])
}

//TODO: consider using this instead of DataView
// private packUintGeneric(value: bigint, byteLength: number): void {
//     const buffer = new ArrayBuffer(byteLength);
//     const view = new DataView(buffer);
//     for (let i = 0; i < byteLength; i++) {
//         view.setUint8(byteLength - 1 - i, Number(value & 255n));
//         value >>= 8n;
//     }
//     const newBytes = new Uint8Array(buffer);
//     this._bytes = new Uint8Array([...this._bytes, ...newBytes]);
// }

// packUint64(value: bigint): void {
//     this.packUintGeneric(value, 8);
// }