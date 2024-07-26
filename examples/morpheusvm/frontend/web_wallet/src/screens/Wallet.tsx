
import { ArrowPathIcon } from '@heroicons/react/20/solid'
import { pubKeyToED25519Addr } from '../lib/bech32'
import { SignerIface } from '../signers/SignerIface'
import { formatBalance } from '../lib/numberFormat'
import { COIN_SYMBOL, DECIMAL_PLACES } from '../const'

export default function Wallet({ signer, balanceBigNumber, onBalanceRefreshRequested, walletName, derivationPath }: { signer: SignerIface, balanceBigNumber: bigint, onBalanceRefreshRequested: () => void, walletName: string, derivationPath: string }) {
    return (<div>
        <h1 className="text-3xl font-bold">{walletName}</h1>
        <div className="text-xs mb-4">Derivation path {derivationPath}</div>
        <div className="text-xl font-mono break-all ">{pubKeyToED25519Addr(signer.getPublicKey())}</div>
        <div className="flex items-center my-12">
            <div className='text-8xl font-bold'>{parseFloat(formatBalance(balanceBigNumber, DECIMAL_PLACES)).toFixed(4)} {COIN_SYMBOL}</div>
            <button className="ml-4" onClick={() => onBalanceRefreshRequested()}>
                <ArrowPathIcon className="h-6 w-6 text-gray-500 hover:text-gray-700" />
            </button>
        </div>
        <div className="flex space-x-4">
            <button className="px-4 py-2 bg-black text-white font-bold rounded hover:bg-gray-800 transition-colors duration-200 transform hover:scale-105">
                Send 0.1 RED
            </button>
            <button className="px-4 py-2 bg-white text-black font-bold rounded border border-black hover:bg-gray-100 transition-colors duration-200 transform hover:scale-105">
                Send 1 RED
            </button>
        </div>
        <div className="mt-8 border border-gray-300 rounded p-4 min-h-16">
            <pre className="font-mono text-sm">
                {`[2023-06-01 10:15:23] Connected to wallet
[2023-06-01 10:15:24] Fetched balance: 22.1234 RED
[2023-06-01 10:16:30] Initiated transaction: Send 0.1 RED
[2023-06-01 10:16:32] Transaction confirmed
[2023-06-01 10:17:45] Refreshed balance: 22.0234 RED
[2023-06-01 10:18:12] Initiated transaction: Send 1 RED
[2023-06-01 10:18:15] Transaction confirmed
`.repeat(Math.random() * 10 + 1)}
            </pre>
        </div>
    </div>)
}