import { MiniPacker } from "../lib/MiniPacker";
import { parseBech32 } from "../lib/bech32";
import { AbstractAction, } from "./AbstractAction";

export const TRANSFER_ACTION_ID = 0x00

export class TransferAction extends AbstractAction {
    constructor(
        public readonly to: string,
        public readonly value: bigint,
        public readonly memo: string,
    ) {
        super()
    }

    toBytes(): Uint8Array {
        const packer = new MiniPacker()

        const [, addrBytes] = parseBech32(this.to)

        packer.packFixedBytes([TRANSFER_ACTION_ID])
        packer.packFixedBytes(addrBytes)
        packer.packUint64(this.value)
        packer.packBytes(this.memo)

        return packer.bytes()
    }
}