package main

import (
	"context"

	"github.com/ava-labs/hypersdk/cli"
	"github.com/ava-labs/hypersdk/examples/tokenvm/cmd/token-cli/cmd"
	"github.com/ava-labs/hypersdk/examples/tokenvm/utils"

	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	databasePath = ".token-browser"
)

// App struct
type App struct {
	ctx context.Context

	log logger.Logger
	h   *cli.Handler
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		log: logger.NewDefaultLogger(),
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	h, err := cli.New(cmd.NewController(databasePath))
	if err != nil {
		a.log.Error(err.Error())
		runtime.Quit(ctx)
		return
	}
	a.h = h

	// Generate key
	if err := h.GenerateKey(); err != nil {
		a.log.Error(err.Error())
		runtime.Quit(ctx)
		return
	}

	// Import ANR
	if err := h.ImportANR(); err != nil {
		a.log.Error(err.Error())
		runtime.Quit(ctx)
		return
	}
}

// shutdown is called after the frontend is destroyed.
func (a *App) shutdown(ctx context.Context) {
	if err := a.h.CloseDatabase(); err != nil {
		a.log.Error(err.Error())
	}
}

func (a *App) GetChains() ([]string, error) {
	rawChains, err := a.h.GetChains()
	if err != nil {
		return nil, err
	}
	chains := make([]string, len(rawChains))
	i := 0
	for c := range rawChains {
		chains[i] = c.String()
		i++
	}
	return chains, nil
}

func (a *App) GetKeys() ([]string, error) {
	pks, err := a.h.GetKeys()
	if err != nil {
		return nil, err
	}
	addresses := make([]string, len(pks))
	for i, pk := range pks {
		addresses[i] = utils.Address(pk.PublicKey())
	}
	return addresses, nil
}
