package anchor

type Anchor struct {
	vm  VM
	Url string `json:"url"`
}

func NewAnchor(url string, vm VM) *Anchor {
	return &Anchor{
		Url: url,
		vm:  vm,
	}
}

func (a *Anchor) RequestAnchorDigest() {

}

func (a *Anchor) RequestAnchorChunk() {

}
