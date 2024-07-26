import { useState } from 'react'
import ConnectWallet from './screens/ConnectWallet'
import { SignerIface } from './signers/SignerIface'
import { pubKeyToED25519Addr } from './lib/bech32'
import { ArrowPathIcon } from '@heroicons/react/20/solid'

function App() {
  const [signers, setSigners] = useState<{ signer1: SignerIface, signer2: SignerIface } | null>(null)


  if (signers === null) {
    return <ConnectWallet onSignerInitComplete={setSigners} />
  }

  return (
    <div className="flex flex-col md:flex-row">
      <div className="w-full md:w-1/2 bg-white p-8">
        <div>
          <h1 className="text-3xl font-bold">Address #1</h1>
          <div className="text-xs mb-4">Derivation path m/44'/9000'/0'</div>
          <div className="text-xl font-mono break-all ">{pubKeyToED25519Addr(signers.signer1.getPublicKey())}</div>
          <div className="flex items-center my-12">
            <div className='text-8xl font-bold'>22.1234 RED</div>
            <button className="ml-4" onClick={() => {/* Add refresh logic here */ }}>
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
[2023-06-01 10:18:16] Refreshed balance: 21.0234 RED
[2023-06-01 10:15:24] Fetched balance: 22.1234 RED
[2023-06-01 10:16:30] Initiated transaction: Send 0.1 RED
[2023-06-01 10:16:32] Transaction confirmed
[2023-06-01 10:17:45] Refreshed balance: 22.0234 RED
[2023-06-01 10:18:12] Initiated transaction: Send 1 RED
[2023-06-01 10:18:15] Transaction confirmed
[2023-06-01 10:18:16] Refreshed balance: 21.0234 RED
[2023-06-01 10:15:24] Fetched balance: 22.1234 RED
[2023-06-01 10:16:30] Initiated transaction: Send 0.1 RED
[2023-06-01 10:16:32] Transaction confirmed
[2023-06-01 10:17:45] Refreshed balance: 22.0234 RED
[2023-06-01 10:18:12] Initiated transaction: Send 1 RED
[2023-06-01 10:18:15] Transaction confirmed
[2023-06-01 10:18:16] Refreshed balance: 21.0234 RED
[2023-06-01 10:15:24] Fetched balance: 22.1234 RED
[2023-06-01 10:16:30] Initiated transaction: Send 0.1 RED
[2023-06-01 10:16:32] Transaction confirmed
[2023-06-01 10:17:45] Refreshed balance: 22.0234 RED
[2023-06-01 10:18:12] Initiated transaction: Send 1 RED
[2023-06-01 10:18:15] Transaction confirmed
[2023-06-01 10:18:16] Refreshed balance: 21.0234 RED
[2023-06-01 10:15:24] Fetched balance: 22.1234 RED
[2023-06-01 10:16:30] Initiated transaction: Send 0.1 RED
[2023-06-01 10:16:32] Transaction confirmed
[2023-06-01 10:17:45] Refreshed balance: 22.0234 RED
[2023-06-01 10:18:12] Initiated transaction: Send 1 RED
[2023-06-01 10:18:15] Transaction confirmed
[2023-06-01 10:18:16] Refreshed balance: 21.0234 RED
[2023-06-01 10:15:24] Fetched balance: 22.1234 RED
[2023-06-01 10:16:30] Initiated transaction: Send 0.1 RED
[2023-06-01 10:16:32] Transaction confirmed
[2023-06-01 10:17:45] Refreshed balance: 22.0234 RED
[2023-06-01 10:18:12] Initiated transaction: Send 1 RED
[2023-06-01 10:18:15] Transaction confirmed
[2023-06-01 10:18:16] Refreshed balance: 21.0234 RED
[2023-06-01 10:15:24] Fetched balance: 22.1234 RED
[2023-06-01 10:16:30] Initiated transaction: Send 0.1 RED
[2023-06-01 10:16:32] Transaction confirmed
[2023-06-01 10:17:45] Refreshed balance: 22.0234 RED
[2023-06-01 10:18:12] Initiated transaction: Send 1 RED
[2023-06-01 10:18:15] Transaction confirmed
[2023-06-01 10:18:16] Refreshed balance: 21.0234 RED
[2023-06-01 10:15:24] Fetched balance: 22.1234 RED
[2023-06-01 10:16:30] Initiated transaction: Send 0.1 RED
[2023-06-01 10:16:32] Transaction confirmed
[2023-06-01 10:17:45] Refreshed balance: 22.0234 RED
[2023-06-01 10:18:12] Initiated transaction: Send 1 RED
[2023-06-01 10:18:15] Transaction confirmed
[2023-06-01 10:18:16] Refreshed balance: 21.0234 RED
[2023-06-01 10:15:24] Fetched balance: 22.1234 RED
[2023-06-01 10:16:30] Initiated transaction: Send 0.1 RED
[2023-06-01 10:16:32] Transaction confirmed
[2023-06-01 10:17:45] Refreshed balance: 22.0234 RED
[2023-06-01 10:18:12] Initiated transaction: Send 1 RED
[2023-06-01 10:18:15] Transaction confirmed
[2023-06-01 10:18:16] Refreshed balance: 21.0234 RED
[2023-06-01 10:15:24] Fetched balance: 22.1234 RED
[2023-06-01 10:16:30] Initiated transaction: Send 0.1 RED
[2023-06-01 10:16:32] Transaction confirmed
[2023-06-01 10:17:45] Refreshed balance: 22.0234 RED
[2023-06-01 10:18:12] Initiated transaction: Send 1 RED
[2023-06-01 10:18:15] Transaction confirmed
[2023-06-01 10:18:16] Refreshed balance: 21.0234 RED`}
            </pre>
          </div>
        </div>
      </div>
      <div className="w-full md:w-1/2 bg-gray-200 p-8 min-h-screen">
        <h1 className="text-3xl font-bold">Address #2</h1>
        <div className="text-xs mb-4">Derivation path m/44'/9000'/1'</div>
        <div className="text-xl font-mono break-all">{pubKeyToED25519Addr(signers.signer2.getPublicKey())}</div>
      </div>
    </div>
  )
}

export default App



// async function refreshBalance(): Promise<boolean> {
//   setLoading(loading + 1)
//   try {
//       const balanceBigNumber = await getBalance(myAddress)
//       const oldBalance = myBalance
//       const newBalance = formatBalance(balanceBigNumber, DECIMAL_PLACES)
//       setMyBalance(newBalance)
//       return oldBalance !== newBalance
//   } catch (e) {
//       setErrors([...errors, (e as Error)?.message || String(e)])
//       throw e
//   } finally {
//       setLoading(loading - 1)
//   }
// }