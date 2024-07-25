import { useState } from 'react'
import ConnectWallet from './ConnectWallet'
import { SignerIface } from './Signer'

function App() {
  const [wallet, setWallet] = useState<SignerIface | null>(null)

  return (
    <>
      {wallet === null && <ConnectWallet onWalletInitComplete={setWallet} />}
    </>
  )
}

export default App
