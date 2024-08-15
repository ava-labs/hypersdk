package anchor

import (
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/crypto/bls"
	"github.com/stretchr/testify/require"
)

func TestAnchorRegisterMsgCanMarshal(t *testing.T) {
	key, err := bls.GeneratePrivateKey()
	require.NoError(t, err)
	pubkey := bls.PublicFromPrivateKey(key)

	cert := AnchorRegisterMsg{
		Slot:   1,
		Url:    "http://localhost:9090",
		Issuer: pubkey,
	}

	digest, err := cert.Digest()
	require.NoError(t, err)

	sig := bls.Sign(digest, key)
	cert.IssuerSig = sig

	cert.Signers = set.NewBits()
	cert.Signers.Add(0)
	sigs := make([]*bls.Signature, 0)
	sigs = append(sigs, cert.IssuerSig)

	cert.Signature, err = bls.AggregateSignatures(sigs)
	require.NoError(t, err)

	certBytes, err := cert.Marshal()
	require.NoError(t, err)

	require.Equal(t, cert.Size(), len(certBytes))

	recovered, err := UnmarshalAnchorRegisterMsg(certBytes)
	require.NoError(t, err)

	_, err = recovered.Marshal()
	require.NoError(t, err)

	require.Equal(t, cert.Slot, recovered.Slot)
	require.Equal(t, cert.Url, recovered.Url)
	require.Equal(t, cert.Issuer, recovered.Issuer)
	require.Equal(t, cert.IssuerSig, recovered.IssuerSig)
	require.Equal(t, cert.Signers, recovered.Signers)
	require.Equal(t, cert.Signature, recovered.Signature)
}

func TestAnchorRegisterMsgCanBeVerified(t *testing.T) {
	key, err := bls.GeneratePrivateKey()
	require.NoError(t, err)
	pubkey := bls.PublicFromPrivateKey(key)

	cert := AnchorRegisterMsg{
		Slot:   1,
		Url:    "http://localhost:9090",
		Issuer: pubkey,
	}

	digest, err := cert.Digest()
	require.NoError(t, err)

	networkID := 200
	chainID := ids.Empty

	wm, err := warp.NewUnsignedMessage(uint32(networkID), chainID, digest)
	require.NoError(t, err)

	sig := bls.Sign(wm.Bytes(), key)
	cert.IssuerSig = sig

	pubkeys := make([]*bls.PublicKey, 0)
	pubkeys = append(pubkeys, pubkey)
	aggrPubkey, err := bls.AggregatePublicKeys(pubkeys)
	require.NoError(t, err)

	sigs := make([]*bls.Signature, 0)
	sigs = append(sigs, sig)
	aggrSig, err := bls.AggregateSignatures(sigs)
	require.NoError(t, err)

	cert.Signature = aggrSig

	verified := cert.VerifySignature(uint32(networkID), chainID, aggrPubkey)
	require.True(t, verified)
}
