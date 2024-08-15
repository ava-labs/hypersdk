package anchor

import (
	"fmt"
	"sync"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
)

type AnchorRegisterMsg struct {
	Slot int64 // used to calcualte height and aggregate key

	Url string

	Issuer    *bls.PublicKey `json:"issuer"`
	IssuerSig *bls.Signature `json:"issuerSig"`

	Signers   set.Bits       `json:"signers"`
	Signature *bls.Signature `json:"signature"`
}

func (a *AnchorRegisterMsg) ID() ids.ID {
	return ids.ID([]byte(a.Url))
}

func (a *AnchorRegisterMsg) Expiry() int64 {
	return a.Slot
}

func (a *AnchorRegisterMsg) Size() int {
	signers := a.Signers.Bytes()
	return consts.Int64Len + codec.BytesLen([]byte(a.Url)) + bls.PublicKeyLen + bls.SignatureLen + codec.BytesLen(signers) + bls.SignatureLen
}

func (a *AnchorRegisterMsg) Digest() ([]byte, error) {
	size := consts.Int64Len + codec.BytesLen([]byte(a.Url)) + bls.PublicKeyLen
	p := codec.NewWriter(size, consts.NetworkSizeLimit)
	p.PackInt64(a.Slot)
	p.PackBytes([]byte(a.Url))
	p.PackFixedBytes(bls.PublicKeyToCompressedBytes(a.Issuer))

	return p.Bytes(), p.Err()
}

func (a *AnchorRegisterMsg) VerifySignature(networkID uint32, chainID ids.ID, aggrPubKey *bls.PublicKey) bool {
	digest, err := a.Digest()
	if err != nil {
		return false
	}
	// TODO: don't use warp message for this (nice to have chainID protection)?
	msg, err := warp.NewUnsignedMessage(networkID, chainID, digest)
	if err != nil {
		return false
	}
	return bls.Verify(aggrPubKey, a.Signature, msg.Bytes())
}

func (a *AnchorRegisterMsg) Marshal() ([]byte, error) {
	p := codec.NewWriter(a.Size(), consts.NetworkSizeLimit)

	p.PackInt64(a.Slot)
	p.PackBytes([]byte(a.Url))
	p.PackFixedBytes(bls.PublicKeyToCompressedBytes(a.Issuer))
	p.PackFixedBytes(bls.SignatureToBytes(a.IssuerSig))

	p.PackBytes(a.Signers.Bytes())
	p.PackFixedBytes(bls.SignatureToBytes(a.Signature))

	return p.Bytes(), p.Err()
}

func UnmarshalAnchorRegisterMsg(raw []byte) (*AnchorRegisterMsg, error) {
	var (
		p = codec.NewReader(raw, consts.NetworkSizeLimit)
		a AnchorRegisterMsg
	)

	a.Slot = p.UnpackInt64(false)
	var urlBytes []byte
	p.UnpackBytes(-1, false, &urlBytes)
	a.Url = string(urlBytes)

	issuerBytes := make([]byte, bls.PublicKeyLen)
	p.UnpackFixedBytes(bls.PublicKeyLen, &issuerBytes)
	issuer, err := bls.PublicKeyFromCompressedBytes(issuerBytes)
	if err != nil {
		return nil, err
	}
	a.Issuer = issuer

	issuerSigBytes := make([]byte, bls.SignatureLen)
	p.UnpackFixedBytes(bls.SignatureLen, &issuerSigBytes)
	issuerSig, err := bls.SignatureFromBytes(issuerSigBytes)
	if err != nil {
		return nil, err
	}
	a.IssuerSig = issuerSig

	var signersBytes []byte
	p.UnpackBytes(32, true, &signersBytes)
	a.Signers = set.BitsFromBytes(signersBytes)
	if len(signersBytes) != len(a.Signers.Bytes()) {
		return nil, fmt.Errorf("invalid anchor register msg: signers not minimal")
	}
	sig := make([]byte, bls.SignatureLen)
	p.UnpackFixedBytes(bls.SignatureLen, &sig)

	signature, err := bls.SignatureFromBytes(sig)
	if err != nil {
		return nil, err
	}
	a.Signature = signature

	if !p.Empty() {
		return nil, fmt.Errorf("invalid anchor register msg: remaining=%d", len(raw)-p.Offset())
	}
	return &a, p.Err()
}

type Anchor struct {
	vm   VM
	Url  string `json:"url"`
	UrlL sync.Mutex
}

func NewAnchor(url string, vm VM) *Anchor {
	return &Anchor{
		Url: url,
		vm:  vm,
	}
}

// only the Anchor provide the signature signed by the same key the validator has will be accepted
func (a *Anchor) Replace(url string) {
	a.UrlL.Lock()
	defer a.UrlL.Unlock()

	a.Url = url
}

func (a *Anchor) RequestAnchorDigest() ([]byte, error) {
	return nil, nil
}

// returns Slot, Txs, FeeReceiver, error
func (a *Anchor) RequestAnchorChunk(sig *bls.Signature) (*chain.Anchor, int64, []*chain.Transaction, codec.Address, error) {
	return nil, 0, nil, codec.EmptyAddress, nil
}
