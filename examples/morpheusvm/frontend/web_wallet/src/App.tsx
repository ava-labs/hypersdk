import { useState } from 'react'
import ConnectWallet from './components/ConnectWallet'
import { SignerIface } from './signers/SignerIface'
import BalanceAndSign from './components/BalanceAndSign'
import { pubKeyToED25519Addr } from './lib/bech32'

function App() {
  const [wallet, setWallet] = useState<SignerIface | null>(null)

  return (
    <>
      {wallet === null && <ConnectWallet onWalletInitComplete={setWallet} />}
      {wallet !== null && <BalanceAndSign myAddress={pubKeyToED25519Addr(wallet.getPublicKey())} />}
    </>
  )
}

export default App
