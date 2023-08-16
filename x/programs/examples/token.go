package examples

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/x/programs/meter"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
	"github.com/ava-labs/hypersdk/x/programs/utils"

	"go.uber.org/zap"
)

func NewToken(log logging.Logger, programPath, rustPath string, profilingEnabled bool) *Basic {
	return &Basic{
		log:              log,
		programPath:      programPath,
		rustPath:         rustPath,
	}
}

type Basic struct {
	log              logging.Logger 
	programPath      string
	rustPath         string
}

func (b *Basic) Run(ctx context.Context) error {
	programBytes, err := utils.GetProgramBytes(b.programPath)
	if err != nil {
		return fmt.Errorf("failed to initializing contract: %w", err)
	}

	// functions exported in this example
	functions := []string{
		"get_total_supply",
		"mint_to",
		"get_balance",
		"transfer",
		"alloc",
		"dealloc",
		"init_contract",
	}

	// example cost map
	costMap := map[string]uint64{
		"ConstI32 0x0": 1,
		"ConstI64 0x0": 2,
	}

	db := utils.NewTestDB()
	meter := meter.New(50, costMap)

	runtime := runtime.New(b.log, db, meter, programPrefix)
	err = runtime.Initialize(ctx, programBytes, functions)
	if err != nil {
		return err
	}

	result, err := runtime.Call(ctx, "init_contract")
	if err != nil {
		return err
	}
	b.log.Debug("Initial cost", zap.Int("gas", 0))

	contract_id := result[0]
	result, err = runtime.Call(ctx, "get_total_supply", contract_id)
	if err != nil {
		return err
	}
	b.log.Debug("Total Supply", zap.Uint64("Total Supply", result[0]))

	mintAlice := uint64(100)

	priv1, err := ed25519.GeneratePrivateKey()
	if err != nil {
		return err
	}
	aliceKey := priv1.PublicKey()
	alicePtr, err := runtime.WriteGuestBuffer(ctx, aliceKey[:])
	if err != nil {
		return err
	}

	priv2, err := ed25519.GeneratePrivateKey()
	if err != nil {
		return err
	}
	bobKey := priv2.PublicKey()
	bobPtr, err := runtime.WriteGuestBuffer(ctx, bobKey[:])
	if err != nil {
		return err
	}

	_, err = runtime.Call(ctx, "mint_to", contract_id, alicePtr, mintAlice)
	if err != nil {
		return fmt.Errorf("failed to call mint_to: %w", err)
	}
	b.log.Debug("Minted", zap.Uint64("Alice", mintAlice))

	result, err = runtime.Call(ctx, "get_balance", contract_id, alicePtr)
	if err != nil {
		return fmt.Errorf("failed to call get_balance: %w", err)
	}
	b.log.Debug("Balance", zap.Int64("Alice", int64(result[0])))

	// deallocate bytes
	defer func() {
		_, err = runtime.Call(ctx, "dealloc", alicePtr, ed25519.PublicKeyLen)
		if err != nil {
			b.log.Error("failed to deallocate alice ptr", zap.Error(err))
		}
		_, err = runtime.Call(ctx, "dealloc", bobPtr, ed25519.PublicKeyLen)
		if err != nil {
			b.log.Error("failed to deallocate bob ptr", zap.Error(err))
		}
	}()

	result, err = runtime.Call(ctx, "get_balance", contract_id, bobPtr)
	if err != nil {
		return fmt.Errorf("failed to call get_balance: %w", err)
	}
	b.log.Debug("Balance", zap.Int64("Bob", int64(result[0])))

	transferToBob := uint64(50)
	_, err = runtime.Call(ctx, "transfer", contract_id, alicePtr, bobPtr, transferToBob)
	if err != nil {
		return fmt.Errorf("failed to call transfer: %w", err)
	}
	b.log.Debug("Transferred",
		zap.Uint64("Alice", transferToBob),
		zap.Uint64("to Bob", transferToBob),
	)

	result, err = runtime.Call(ctx, "get_balance", contract_id, alicePtr)
	if err != nil {
		return fmt.Errorf("failed to call get_balance: %w", err)
	}
	b.log.Debug("Balance", zap.Int64("Alice", int64(result[0])))

	result, err = runtime.Call(ctx, "get_balance", contract_id, bobPtr)
	if err != nil {
		return fmt.Errorf("failed to call get_balance: %w", err)
	}
	b.log.Debug("Balance", zap.Int64("Bob", int64(result[0])))

	return nil
}
