import { KeyIcon, BackspaceIcon } from '@heroicons/react/20/solid'
import { EphemeralWallet, MetamaskSnapWallet, WalletIface } from './Wallet'
import { SNAP_ID } from './const'


export default function ConnectWalletWindow({ onWalletSelected }: { onWalletSelected: (wallet: WalletIface) => void }) {
    return (
        <div className="flex items-center justify-center min-h-screen bg-gray-200">
            {/* Really needs a Morpheus reference */}
            <div className="text-center w-full max-w-md px-4">
                <h3 className="text-4xl font-semibold text-gray-900">Morpheus Demo</h3>
                <p className="mt-4 text-lg text-gray-500">Connect with Metamask Flask development wallet via a Snap, or create a wallet in memory.</p>
                <div className="mt-8 flex flex-col sm:flex-row justify-between space-y-4 sm:space-y-0 sm:space-x-4">
                    <button
                        type="button"
                        className="w-full sm:w-1/2 inline-flex items-center justify-center rounded-md bg-red-600 px-4 py-3 text-sm font-semibold text-white shadow-sm hover:bg-red-500 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-indigo-600"
                        onClick={() => onWalletSelected(new MetamaskSnapWallet(SNAP_ID))}
                    >
                        <KeyIcon aria-hidden="true" className="-ml-0.5 mr-1.5 h-5 w-5" />
                        Metamask Snap
                    </button>
                    <button
                        type="button"
                        className="w-full sm:w-1/2 inline-flex items-center justify-center rounded-md bg-blue-600 px-4 py-3 text-sm font-semibold text-white shadow-sm hover:bg-blue-500 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-indigo-600"
                        onClick={() => onWalletSelected(new EphemeralWallet())}
                    >
                        <BackspaceIcon aria-hidden="true" className="-ml-0.5 mr-1.5 h-5 w-5" />
                        Ephemeral wallet
                    </button>
                </div>
            </div>
        </div>
    )
}