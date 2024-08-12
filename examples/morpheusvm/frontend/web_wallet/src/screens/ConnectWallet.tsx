import { SNAP_ID } from '../const'
import { useState } from 'react'
import Loading from './Loading'
import FullScreenError from './FullScreenError'
import { EphemeralSigner, MetamaskSnapSigner, SignerIface } from '../lib/signers'


export default function ConnectWalletWindow({ onSignerInitComplete }: { onSignerInitComplete: (signers: { signer1: SignerIface, signer2: SignerIface }) => void }) {

    const [loading, setLoading] = useState(0)
    const [errors, setErrors] = useState<string[]>([])

    async function selectMetamaskSnap() {
        try {
            setLoading(loading + 1)
            const signer1 = new MetamaskSnapSigner(SNAP_ID)
            const signer2 = new MetamaskSnapSigner(SNAP_ID, 1)

            await signer1.connect()
            await signer2.connect()
            onSignerInitComplete({ signer1: signer1, signer2: signer2 })
        } catch (e) {
            console.error(e)
            setErrors(errors => [...errors, (e as Error)?.message || String(e)])
        } finally {
            setLoading(loading - 1)
        }
    }

    if (loading > 0) {
        return <Loading text="Please confirm the connection in Metamask Flask signer" />
    }

    if (errors.length > 0) {
        return <FullScreenError errors={errors} />
    }
    return (
        <div className="flex items-center justify-center min-h-screen">
            <div className="border border-black px-16 py-12 rounded-lg max-w-xl w-full text-center">
                <h3 className="text-4xl font-semibold text-gray-900 mb-5 ">HyperSDK e2e demo</h3>
                <p className="mt-4 text-sm">Connect with Metamask Flask development signer via a Snap, or create a signer in memory.</p>
                <div className="mt-8 flex flex-col sm:flex-row justify-between space-y-4 sm:space-y-0 sm:space-x-4">
                    <button
                        type="button"
                        className="w-48 px-4 py-2 bg-black text-white font-bold rounded hover:bg-gray-800 transition-colors duration-200 transform hover:scale-105"
                        onClick={() => selectMetamaskSnap()}
                    >
                        Metamask Snap
                    </button>
                    <button
                        type="button"
                        className="w-48 px-4 py-2 bg-white text-black font-bold rounded border border-black hover:bg-gray-100 transition-colors duration-200 transform hover:scale-105"
                        onClick={() => onSignerInitComplete({ signer1: new EphemeralSigner(), signer2: new EphemeralSigner() })}
                    >
                        Temporary key
                    </button>
                </div>
            </div>
        </div>
    )
}