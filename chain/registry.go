package chain

import (
	"github.com/ava-labs/hypersdk/codec"
)

type AuthRegistry struct {
	auths     []Auth
	authFuncs []func(*codec.Packer) (Auth, error)
}

func (a *AuthRegistry) Register(instance Auth, f func(*codec.Packer) (Auth, error)) error {
	a.auths = append(a.auths, instance)
	a.authFuncs = append(a.authFuncs, f)
	return nil
}

func (p *AuthRegistry) LookupIndex(index uint8) (func(*codec.Packer) (Auth, error), bool) {
	if index >= uint8(len(p.authFuncs)) {
		return nil, false
	}
	return p.authFuncs[index], true
}

type ActionRegistry struct {
	actions     []Action
	actionFuncs []func(*codec.Packer) (Action, error)
}

func (a *ActionRegistry) Register(instance Action, f func(*codec.Packer) (Action, error)) error {
	a.actions = append(a.actions, instance)
	a.actionFuncs = append(a.actionFuncs, f)

	return nil
}

func (p *ActionRegistry) LookupIndex(index uint8) (func(*codec.Packer) (Action, error), bool) {
	if index >= uint8(len(p.actionFuncs)) {
		return nil, false
	}
	return p.actionFuncs[index], true
}
