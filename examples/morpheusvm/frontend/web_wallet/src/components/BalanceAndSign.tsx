import { useEffect, useState } from 'react'
import { ArrowRightIcon } from '@heroicons/react/20/solid'
import { COIN_SYMBOL, DECIMAL_PLACES } from '../const'
import { getBalance } from '../lib/api'
import { formatBalance } from '../lib/numberFormat'
import Errors from './Errors'
import Loading from './Loading'

export default function BalanceAndSign({ myAddress }: { myAddress: string }) {
    const [loading, setLoading] = useState(0)
    const [errors, setErrors] = useState<string[]>([])

    async function refreshBalance(): Promise<boolean> {
        setLoading(loading + 1)
        try {
            const balanceBigNumber = await getBalance(myAddress)
            const oldBalance = myBalance
            const newBalance = formatBalance(balanceBigNumber, DECIMAL_PLACES)
            setMyBalance(newBalance)
            return oldBalance !== newBalance
        } catch (e) {
            setErrors([...errors, (e as Error)?.message || String(e)])
            throw e
        } finally {
            setLoading(loading - 1)
        }
    }

    useEffect(() => {
        refreshBalance()
    }, [])

    const [myBalance, setMyBalance] = useState('...')

    const [addressTo, setAddressTo] = useState('morpheus1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqxez33a')
    const [amount, setAmount] = useState(0.0)

    const transferButtonDisabled = amount <= 0

    if (loading > 0) {
        return <Loading text="Contacting the blockchain..." />
    } else if (errors.length > 0) {
        return <Errors errors={errors} />
    }

    return (
        <div className="flex items-center justify-center min-h-screen">
            <div className="text-center w-full max-w-2xl px-4">
                <p className="mt-2 text-sm text-gray-500">{myAddress}</p>
                <div className="mt-2">
                    <h2 className="text-5xl font-bold text-gray-900">
                        {parseFloat(myBalance)} {COIN_SYMBOL}
                    </h2>
                </div>
                <div className="mt-8 p-4 border border-gray-300 rounded-md  bg-gray-100">
                    <input
                        type="text"
                        className="text-sm w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                        placeholder="Enter recipient address"
                        value={addressTo}
                        onChange={(e) => setAddressTo(e.target.value)}
                    />
                    <input
                        type="number"
                        className="text-sm w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 mt-4"
                        placeholder="Enter amount to transfer"
                        value={amount}
                        onChange={(e) => setAmount(parseFloat(e.target.value) || 0.0)}
                    />
                    <button
                        type="button"
                        className="disabled:bg-gray-500 mt-4 w-full inline-flex items-center justify-center rounded-md bg-blue-600 px-4 py-3 text-sm font-semibold text-white shadow-sm hover:bg-blue-500 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-600"
                        disabled={transferButtonDisabled}
                    >
                        <ArrowRightIcon aria-hidden="true" className="-ml-0.5 mr-1.5 h-5 w-5" />
                        Transfer {amount} {COIN_SYMBOL}
                    </button>
                </div>
            </div>
        </div>
    )
}