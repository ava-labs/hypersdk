package pb

// import (
// 	"github.com/AnomalyFi/hypersdk/crypto"
// )
// crypto.PublicKey
//TODO need to figure out how to override
// message type for hypersdk and also how to use this key system
// Might not have to use it though because these addresses aren't usually on this chains
//
// var _ sdk.Msg = &SequencerMsg{}

func NewSequencerMsg(chainID, data []byte, fromAddr string) *SequencerMsg {
	return &SequencerMsg{
		ChainId:     chainID,
		Data:        data,
		FromAddress: fromAddr,
	}
}

func (*SequencerMsg) Route() string { return "SequencerMsg" }
func (*SequencerMsg) ValidateBasic() error {
	return nil
}
func (m *SequencerMsg) GetSigners() string {
	return m.FromAddress
}
func (m *SequencerMsg) XXX_MessageName() string {
	return "SequencerMsg"
}
