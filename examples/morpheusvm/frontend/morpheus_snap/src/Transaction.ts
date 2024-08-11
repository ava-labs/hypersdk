import { MiniPacker } from "../lib/MiniPacker";
import { SignerIface } from "../signers/SignerIface";



// export class Transaction {
//     constructor(
//         public readonly timestamp: bigint,
//         public readonly chainId: bigint,
//         public readonly maxFee: bigint,
//         public readonly actions: ActionData[],
//         public readonly abi: string
//     ) {
//         // if (timestamp < new Date(2020, 0, 1).getTime()) {
//         //     throw new Error("Timestamp must be greater than 2020-01-01")
//         // } else if (timestamp > new Date(2030, 0, 1).getTime()) {
//         //     throw new Error("Timestamp must be less than 2030-01-01")
//         // }


//         // Ensure the last 3 digits of the timestamp are 000
//         this.timestamp = this.timestamp / 1000n * 1000n
//     }

//     public digest(): Uint8Array {


//         //add base
//         const packer = new MiniPacker();
//         packer.packUint64(this.timestamp);
//         packer.packUint256(this.chainId);
//         packer.packUint64(this.maxFee);

//         //add action count 
//         packer.packUint8(BigInt(this.actions.length))

//         for (const action of this.actions) {
//             packer.packFixedBytes(action.toBytes())//pack action bytes
//         }
//         return packer.bytes()
//     }

//     public async sign(provider: SignerIface, abiString: string): Promise<Uint8Array> {
//         const signature = await provider.signTx(this.digest())
//         const signer = provider.getPublicKey()
//         const authIDByte = provider.getAuthId()
//         return new Uint8Array([...this.digest(), authIDByte, ...signer, ...signature])
//     }
// }