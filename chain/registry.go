package chain

import (
	"github.com/ava-labs/hypersdk/codec"
)

type Typed interface {
	GetTypeID() uint8
}

type AuthRegistry struct {
	auths     map[uint8]Typed
	authFuncs map[uint8]func(*codec.Packer) (Auth, error)
}

func NewAuthRegistry() *AuthRegistry {
	return &AuthRegistry{
		auths:     make(map[uint8]Typed),
		authFuncs: make(map[uint8]func(*codec.Packer) (Auth, error)),
	}
}

func (a *AuthRegistry) Register(instance Typed, f func(*codec.Packer) (Auth, error)) error {
	typeId := instance.GetTypeID()

	a.auths[typeId] = instance
	a.authFuncs[typeId] = f
	return nil
}

func (p *AuthRegistry) LookupIndex(typeId uint8) (func(*codec.Packer) (Auth, error), bool) {
	f, found := p.authFuncs[typeId]
	return f, found
}

type ActionRegistry struct {
	actions     map[uint8]Typed
	actionFuncs map[uint8]func(*codec.Packer) (Action, error)
}

func NewActionRegistry() *ActionRegistry {
	return &ActionRegistry{
		actions:     make(map[uint8]Typed),
		actionFuncs: make(map[uint8]func(*codec.Packer) (Action, error)),
	}
}

func (a *ActionRegistry) Register(instance Typed, outputInstance interface{}, f func(*codec.Packer) (Action, error)) error {
	typeId := instance.GetTypeID()

	a.actions[typeId] = instance
	a.actionFuncs[typeId] = f
	return nil
}

func (p *ActionRegistry) LookupIndex(typeId uint8) (func(*codec.Packer) (Action, error), bool) {
	f, found := p.actionFuncs[typeId]
	return f, found
}

func (p *ActionRegistry) GetRegisteredTypes() []Typed {
	types := make([]Typed, 0, len(p.actions))
	for _, action := range p.actions {
		types = append(types, action)
	}
	return types
}
