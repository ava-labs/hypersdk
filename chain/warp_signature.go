package chain

type WarpSignature struct {
	PublicKey []byte `json:"publicKey"`
	Signature []byte `json:"signature"`
}
