import { useState } from 'react'
import ConnectWallet from './components/ConnectWallet'
import { SignerIface } from './signers/SignerIface'

function App() {
  const [wallet, setWallet] = useState<SignerIface | null>(null)

  return (
    <>
      {wallet === null && <ConnectWallet onWalletInitComplete={setWallet} />}
    </>
  )
}

export default App
