import { useState } from 'react'
import { ArrowRightIcon } from '@heroicons/react/20/solid'
import { COIN_SYMBOL } from '../const'

export default function BalanceAndSign({ myAddress }: { myAddress: string }) {
    const [address, setAddress] = useState('morpheus1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqxez33a')
    const [amount, setAmount] = useState(0.0)

    const userBalance = 'TODO:'

    const transferButtonDisabled = amount <= 0

    return (
        <div className="flex items-center justify-center min-h-screen bg-gray-200">
            <div className="text-center w-full max-w-2xl px-4">
                <p className="mt-2 text-sm text-gray-500">{myAddress}</p>
                <div className="mt-2">
                    <h2 className="text-5xl font-bold text-gray-900">
                        {parseFloat(userBalance)} {COIN_SYMBOL}
                    </h2>
                </div>
                <div className="mt-8 p-4 border border-gray-300 rounded-md  bg-gray-100">
                    <input
                        type="text"
                        className="text-sm w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                        placeholder="Enter recipient address"
                        value={address}
                        onChange={(e) => setAddress(e.target.value)}
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