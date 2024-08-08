
import { ArrowPathIcon } from '@heroicons/react/20/solid'
import { pubKeyToED25519Addr } from '../lib/bech32'
import { SignerIface } from '../signers/SignerIface'
import { formatBalance, fromFormattedBalance } from '../lib/numberFormat'
import { COIN_SYMBOL, DECIMAL_PLACES, MAX_TRANSFER_FEE } from '../const'
import { getBalance, getNetwork, sendTx } from '../lib/api'
import { TransferAction } from '../actions/TransferAction'
import { Transaction } from '../chain/Transaction'
import { idStringToBigInt } from '../lib/cb58'
import { useState } from 'react'

export default function Wallet({ otherWalletAddress, signer, balanceBigNumber, onBalanceRefreshRequested, walletName, derivationPath }: { otherWalletAddress: string, signer: SignerIface, balanceBigNumber: bigint, onBalanceRefreshRequested: () => void, walletName: string, derivationPath: string }) {
    const myAddr = pubKeyToED25519Addr(signer.getPublicKey())

    const [loadingCounter, setLoadingCounter] = useState(0)
    const [logText, setLogText] = useState("")

    function log(level: "success" | "error" | "info", text: string) {
        const now = new Date();
        const time = now.toLocaleTimeString('en-US', { hour12: false });
        let emoji = '';
        emoji = level === 'success' ? '✅' : level === 'error' ? '❌' : 'ℹ️';
        setLogText(prevLog => `${prevLog}\n${time} ${emoji} ${text}`);
    }

    async function sendTokens(amountString: "0.1" | "1") {
        setLogText("")
        try {
            log("info", `Sending ${amountString} ${COIN_SYMBOL} to ${otherWalletAddress}`)
            setLoadingCounter(counter => counter + 1)
            const amount = fromFormattedBalance(amountString, DECIMAL_PLACES)
            const initialBalance = await getBalance(myAddr)

            const action = new TransferAction(otherWalletAddress, amount, "just testing")

            log("info", `Initial balance: ${formatBalance(initialBalance, DECIMAL_PLACES)} ${COIN_SYMBOL}`)

            const chainIdStr = (await getNetwork()).chainId
            const chainId = idStringToBigInt(chainIdStr)

            const tx = new Transaction(
                BigInt(Date.now()) + 60n * 1000n,//5 minutes from now
                chainId,
                MAX_TRANSFER_FEE,
                [action]
            )
            const signed = await tx.sign(signer)

            log("success", `Transaction signed`)


            await sendTx(signed)
            log("success", `Transaction sent, waiting for the balance change`)


            let balanceChanged = false
            const totalWaitTime = 15 * 1000
            const timeStarted = Date.now()

            for (let i = 0; i < 100000; i++) {//curcuit breaker
                const balance = await getBalance(myAddr)
                if (balance !== initialBalance || Date.now() - timeStarted > totalWaitTime) {
                    balanceChanged = true
                    log("success", `Balance changed to ${parseFloat(formatBalance(balance, DECIMAL_PLACES)).toFixed(6)} ${COIN_SYMBOL} in ${((Date.now() - timeStarted) / 1000).toFixed(2)}s`)
                    break
                } else {
                    await new Promise(resolve => setTimeout(resolve, 10))
                }
            }

            if (!balanceChanged) {
                throw new Error("Transaction failed")
            }

            console.log("Transaction successful")


            await onBalanceRefreshRequested()
        } catch (e: unknown) {
            log("error", `Transaction failed: ${(e as { message?: string })?.message || String(e)}`);
            console.error(e)
        } finally {
            setLoadingCounter(counter => counter - 1)
        }
    }

    return (<div className={loadingCounter > 0 ? "animate-pulse" : ""}>
        <h1 className="text-3xl font-bold">{walletName}</h1>
        <div className="text-xs mb-4">Derivation path {derivationPath}</div>
        <div className="text-xl font-mono break-all ">{myAddr}</div>
        <div className="flex items-center my-12">
            <div className='text-8xl font-bold'>{parseFloat(formatBalance(balanceBigNumber, DECIMAL_PLACES)).toFixed(6)} {COIN_SYMBOL}</div>
            <button className="ml-4" onClick={() => onBalanceRefreshRequested()}>
                <ArrowPathIcon className="h-6 w-6 text-gray-500 hover:text-gray-700" />
            </button>
        </div>
        <div className="flex space-x-4">
            <button className={`px-4 py-2 font-bold rounded transition-colors duration-200 ${loadingCounter > 0 ? 'bg-gray-400 text-gray-600 cursor-not-allowed' : 'bg-black text-white hover:bg-gray-800 transform hover:scale-105'}`}
                onClick={() => sendTokens("0.1")}
                disabled={loadingCounter > 0}
            >
                Send 0.1 RED
            </button>
            <button className={`px-4 py-2 font-bold rounded border transition-colors duration-200 ${loadingCounter > 0 ? 'bg-gray-100 text-gray-400 border-gray-300 cursor-not-allowed' : 'bg-white text-black border-black hover:bg-gray-100 transform hover:scale-105'}`}
                onClick={() => sendTokens("1")}
                disabled={loadingCounter > 0}
            >
                Send 1 RED
            </button>
        </div>
        <div className="mt-8 border border-gray-300 rounded p-4 min-h-16">
            <pre className="font-mono text-sm">
                {logText}
            </pre>
        </div>
    </div>)
}