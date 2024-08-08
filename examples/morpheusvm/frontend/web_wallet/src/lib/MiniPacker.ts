

export class MiniPacker {
    private _bytes: Uint8Array;

    constructor() {
        this._bytes = new Uint8Array([]);
    }
    private packUintGeneric(value: bigint, byteLength: number): void {
        const buffer = new ArrayBuffer(byteLength);
        const view = new DataView(buffer);
        for (let i = 0; i < byteLength; i++) {
            view.setUint8(byteLength - 1 - i, Number(value & 255n));
            value >>= 8n;
        }
        const newBytes = new Uint8Array(buffer);
        this._bytes = new Uint8Array([...this._bytes, ...newBytes]);
    }

    packUint64(value: bigint): void {
        this.packUintGeneric(value, 8);
    }

    packUint32(value: bigint): void {
        this.packUintGeneric(value, 4);
    }

    packUint256(value: bigint): void {
        this.packUintGeneric(value, 32);
    }

    packUint8(value: bigint): void {
        this.packUintGeneric(value, 1);
    }

    packFixedBytes(value: Uint8Array | number[]): void {
        this._bytes = new Uint8Array([...this._bytes, ...value]);
    }

    packBytes(value: string): void {
        this.packUint32(BigInt(value.length))
        this.packFixedBytes(new TextEncoder().encode(value))
    }

    bytes(): Uint8Array {
        return this._bytes;
    }
}

