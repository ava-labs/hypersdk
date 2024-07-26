import { KeyIcon, BackspaceIcon } from '@heroicons/react/20/solid'
import { SignerIface } from '../signers/SignerIface'
import { SNAP_ID } from '../const'
import { MetamaskSnapSigner } from '../signers/MetamaskSnapSigner'
import { EphemeralSigner } from '../signers/EphemeralSigner'
import { useState } from 'react'
import Loading from './Loading'
import Errors from './Errors'


export default function ConnectWalletWindow({ onWalletInitComplete }: { onWalletInitComplete: (wallet: SignerIface) => void }) {

    const [loading, setLoading] = useState(0)
    const [errors, setErrors] = useState<string[]>([])

    async function selectMetamaskSnap() {
        try {
            setLoading(loading + 1)
            const signer = new MetamaskSnapSigner(SNAP_ID)
            await signer.connect()
            onWalletInitComplete(signer)
        } catch (e) {
            console.error(e)
            setErrors(errors => [...errors, (e as Error)?.message || String(e)])
        } finally {
            setLoading(loading - 1)
        }
    }

    if (loading > 0) {
        return <Loading text="Please confirm the connection in Metamask Flask wallet" />
    }

    if (errors.length > 0) {
        return <Errors errors={errors} />
    }
    return (
        <div className="flex items-center justify-center min-h-screen">
            <div className="bg-white px-16 py-12 rounded-lg shadow-md max-w-xl w-full text-center">
                <h3 className="text-4xl font-semibold text-gray-900 mb-5 ">HyperSDK e2e demo</h3>
                <p className="mt-4 text-lg text-gray-500">Connect with Metamask Flask development wallet via a Snap, or create a wallet in memory.</p>
                <div className="mt-8 flex flex-col sm:flex-row justify-between space-y-4 sm:space-y-0 sm:space-x-4">
                    <button
                        type="button"
                        className="w-full sm:w-1/2 inline-flex items-center justify-center rounded-md bg-red-600 px-4 py-3 text-sm font-semibold text-white shadow-sm hover:bg-red-500 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-indigo-600"
                        onClick={() => selectMetamaskSnap()}
                    >
                        <KeyIcon aria-hidden="true" className="-ml-0.5 mr-1.5 h-5 w-5" />
                        Metamask Snap
                    </button>
                    <button
                        type="button"
                        className="w-full sm:w-1/2 inline-flex items-center justify-center rounded-md bg-blue-600 px-4 py-3 text-sm font-semibold text-white shadow-sm hover:bg-blue-500 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-indigo-600"
                        onClick={() => onWalletInitComplete(new EphemeralSigner())}
                    >
                        <BackspaceIcon aria-hidden="true" className="-ml-0.5 mr-1.5 h-5 w-5" />
                        Ephemeral wallet
                    </button>
                </div>
            </div>
        </div>
    )
}