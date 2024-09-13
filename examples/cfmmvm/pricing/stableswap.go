package pricing

var _ Model = (*Stableswap)(nil)

type Stableswap struct{}

func NewStableswap() Model {
	panic("unimplemented")
}

func (s *Stableswap) AddLiquidity(amountX uint64, amountY uint64, lpTokenSupply uint64) (uint64, uint64, uint64, error) {
	panic("unimplemented")
}

// GetState implements Model.
func (s *Stableswap) GetState() (uint64, uint64, uint64) {
	panic("unimplemented")
}

func (s *Stableswap) Initialize(reserveX uint64, reserveY uint64, fee uint64, kLast uint64) {
	panic("unimplemented")
}

func (s *Stableswap) RemoveLiquidity(uint64, uint64) (uint64, uint64, uint64, error) {
	panic("unimplemented")
}

func (s *Stableswap) Swap(amountX uint64, amountY uint64) (uint64, uint64, error) {
	panic("unimplemented")
}
