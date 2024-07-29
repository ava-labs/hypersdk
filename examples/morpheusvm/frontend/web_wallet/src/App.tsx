import { useEffect, useState } from 'react'
import ConnectWallet from './screens/ConnectWallet'
import { SignerIface } from './signers/SignerIface'
import Wallet from './screens/Wallet'
import { getBalance,requestFaucetTransfer } from './lib/api'
import { pubKeyToED25519Addr } from './lib/bech32'
import Loading from './screens/Loading'
import FullScreenError from './screens/FullScreenError'

function App() {
  const [signers, setSigners] = useState<{ signer1: SignerIface, signer2: SignerIface } | null>(null)

  const [balance1, setBalance1] = useState<bigint>(0n)
  const [balance2, setBalance2] = useState<bigint>(0n)


  const [balancesLoading, setBalancesLoading] = useState<number>(0)
  const [errors, setErrors] = useState<string[]>([])


  useEffect(() => {
    signers && refreshBalances();
  }, [signers]);

  const [faucetRequested, setFaucetRequested] = useState<boolean>(false)


  async function refreshBalances(): Promise<void> {
    if (!signers) {
      setErrors(['No signers'])
      return
    }

    setBalancesLoading(balancesLoading + 1)
    try {
      const addr1 = pubKeyToED25519Addr(signers.signer1.getPublicKey())
      const addr2 = pubKeyToED25519Addr(signers.signer2.getPublicKey())

      if(!faucetRequested) {
        await requestFaucetTransfer(addr1)
        setFaucetRequested(true)
      }

      await Promise.all([
        setBalance1(await getBalance(addr1)),
        setBalance2(await getBalance(addr2))
      ])
    } catch (e) {
      setErrors([...errors, (e as Error)?.message || String(e)])
      throw e
    } finally {
      setBalancesLoading(balancesLoading - 1)
    }
  }

  if (balancesLoading > 0) {
    return <Loading text="Contacting the faucet..." />
  }

  if (errors.length > 0) {
    return <FullScreenError errors={errors} />
  }

  if (signers === null) {
    return <ConnectWallet onSignerInitComplete={newSigners => {
      setSigners(newSigners)
    }} />
  }
  return (
    <div className="flex flex-col md:flex-row">
      <div className="w-full md:w-1/2 bg-white p-8">
        <Wallet signer={signers.signer1} derivationPath="m/44'/9000'/0'" balanceBigNumber={balance1} onBalanceRefreshRequested={refreshBalances} walletName={'Address #1'} />
      </div>
      <div className="w-full md:w-1/2 bg-gray-200 p-8 min-h-screen">
        <Wallet signer={signers.signer2} derivationPath="m/44'/9000'/1'" balanceBigNumber={balance2} onBalanceRefreshRequested={refreshBalances} walletName={'Address #2'} />
      </div>
    </div>
  )
}

export default App


