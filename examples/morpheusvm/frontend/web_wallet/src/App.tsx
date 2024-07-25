import { useState } from 'react'
import ConnectWallet from './ConnectWallet'
import { WalletIface } from './Wallet'

function App() {
  const [wallet, setWallet] = useState<WalletIface | null>(null)

  return (
    <>
      {wallet === null && <ConnectWallet onWalletSelected={setWallet} />}
    </>
  )
}

export default App
