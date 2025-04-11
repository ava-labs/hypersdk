// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/StephenButtolph/canoto"
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/internal/emap"
	"github.com/ava-labs/hypersdk/internal/math"
	"github.com/ava-labs/hypersdk/internal/mempool"
	"github.com/ava-labs/hypersdk/keys"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"
	"github.com/ava-labs/hypersdk/utils"

	internalfees "github.com/ava-labs/hypersdk/internal/fees"
)

var (
	_ emap.Item    = (*Transaction)(nil)
	_ mempool.Item = (*Transaction)(nil)
	_ canoto.Field = (*Transaction)(nil)
)

// TransactionData represents an unsigned transaction
type TransactionData struct {
	Base Base

	Actions []Action

	// unsignedBytes is the byte slice representation of the unsigned tx
	// This field is always populated either by the constructor or via signed transaction
	// parsing.
	unsignedBytes []byte
}

func NewTxData(base Base, actions []Action) TransactionData {
	txData := TransactionData{
		Base:    base,
		Actions: actions,
	}
	// Call CalculateCanotoCache so that equivalent blocks pass an equals check.
	// Without calling this function, canoto's required internal field will cause equals
	// checks to fail on otherwise identical blocks.
	txData.Base.CalculateCanotoCache()

	actionBytes := make([]codec.Bytes, len(txData.Actions))
	for i, action := range txData.Actions {
		actionBytes[i] = action.Bytes()
	}
	// Serialize the unsigned transaction
	// Note: canoto does not serialize empty fields, which allows us to
	// re-use the SerializeTx intermediate type directly. This produces an
	// identical serialization to creating a separate SerializeUnsignedTx
	// type that omitted the Auth field.
	serializeTxData := &SerializeTx{
		Base:    txData.Base,
		Actions: actionBytes,
	}
	txData.unsignedBytes = serializeTxData.MarshalCanoto()
	return txData
}

// UnsignedBytes returns the byte slice representation of the tx
func (t *TransactionData) UnsignedBytes() []byte {
	return t.unsignedBytes
}

// Sign returns a new signed transaction with the unsigned tx copied from
// the original and a signature provided by the authFactory
func (t *TransactionData) Sign(
	factory AuthFactory,
) (*Transaction, error) {
	auth, err := factory.Sign(t.UnsignedBytes())
	if err != nil {
		return nil, err
	}
	return NewTransaction(t.Base, t.Actions, auth)
}

func SignRawActionBytesTx(
	base Base,
	rawActionsBytes [][]byte,
	authFactory AuthFactory,
) ([]byte, error) {
	codecBytes := make([]codec.Bytes, len(rawActionsBytes))
	for i, actionBytes := range rawActionsBytes {
		codecBytes[i] = actionBytes
	}
	tx := &SerializeTx{
		Base:    base,
		Actions: codecBytes,
	}
	unsignedTxBytes := tx.MarshalCanoto()
	auth, err := authFactory.Sign(unsignedTxBytes)
	if err != nil {
		return nil, err
	}
	tx.Auth = auth.Bytes()
	return tx.MarshalCanoto(), nil
}

func (t *TransactionData) GetExpiry() int64 { return t.Base.Timestamp }

func (t *TransactionData) MaxFee() uint64 { return t.Base.MaxFee }

// Transaction is a signed transaction that can be executed on chain.
// Transaction must be treated as immutable.
//
// Transaction implements [canoto.Field] using the [SerializeTx] field
// as an intermediate representation, so that it can convert from the
// Action/Auth types to corresponding raw byte slices.
// This additionally allows the transaction type to cache the pre-calculated
// bytes, size, and id fields, so that they never need to be re-computed.
type Transaction struct {
	TransactionData

	Auth Auth `json:"auth"`

	bytes     []byte
	size      int
	id        ids.ID
	stateKeys state.Keys
}

// NewTransaction creates a Transaction and initializes the private fields.
func NewTransaction(base Base, actions []Action, auth Auth) (*Transaction, error) {
	txData := NewTxData(base, actions)
	actionBytes := make([]codec.Bytes, len(actions))
	for i, action := range actions {
		actionBytes[i] = action.Bytes()
	}
	serializeSignedTx := &SerializeTx{
		Base:    base,
		Actions: actionBytes,
		Auth:    auth.Bytes(),
	}
	signedTxBytes := serializeSignedTx.MarshalCanoto()

	return &Transaction{
		TransactionData: txData,
		Auth:            auth,
		bytes:           signedTxBytes,
		size:            len(signedTxBytes),
		id:              utils.ToID(signedTxBytes),
	}, nil
}

func (t *Transaction) Bytes() []byte { return t.bytes }

func (t *Transaction) Size() int { return t.size }

func (t *Transaction) GetID() ids.ID { return t.id }

// StateKeys calculates the set of state keys pre-declared by the transaction.
// This function caches the state keys internally, which makes it unsafe to call in parallel.
func (t *Transaction) StateKeys(bh BalanceHandler) (state.Keys, error) {
	if t.stateKeys != nil {
		return t.stateKeys, nil
	}
	stateKeys := make(state.Keys)

	// Verify the formatting of state keys passed by the controller
	for i, action := range t.Actions {
		for k, v := range action.StateKeys(t.Auth.Actor(), CreateActionID(t.GetID(), uint8(i))) {
			if !stateKeys.Add(k, v) {
				return nil, ErrInvalidKeyValue
			}
		}
	}
	for k, v := range bh.SponsorStateKeys(t.Auth.Sponsor()) {
		if !stateKeys.Add(k, v) {
			return nil, ErrInvalidKeyValue
		}
	}

	// Cache keys if called again
	t.stateKeys = stateKeys
	return stateKeys, nil
}

// Units returns the multi-dimensional fee units required by the transaction. The corresponding
// fee will be charged in full regardless of the transaction's execution result.
func (t *Transaction) Units(bh BalanceHandler, r Rules) (fees.Dimensions, error) {
	// Calculate compute usage
	computeOp := math.NewUint64Operator(r.GetBaseComputeUnits())
	for _, action := range t.Actions {
		computeOp.Add(action.ComputeUnits(r))
	}
	computeOp.Add(t.Auth.ComputeUnits(r))
	maxComputeUnits, err := computeOp.Value()
	if err != nil {
		return fees.Dimensions{}, err
	}

	// Calculate storage usage
	stateKeys, err := t.StateKeys(bh)
	if err != nil {
		return fees.Dimensions{}, err
	}
	readsOp := math.NewUint64Operator(0)
	allocatesOp := math.NewUint64Operator(0)
	writesOp := math.NewUint64Operator(0)
	for k := range stateKeys {
		// Compute key costs
		readsOp.Add(r.GetStorageKeyReadUnits())
		allocatesOp.Add(r.GetStorageKeyAllocateUnits())
		writesOp.Add(r.GetStorageKeyWriteUnits())

		// Compute value costs
		maxChunks, ok := keys.MaxChunks([]byte(k))
		if !ok {
			return fees.Dimensions{}, ErrInvalidKeyValue
		}
		readsOp.MulAdd(uint64(maxChunks), r.GetStorageValueReadUnits())
		allocatesOp.MulAdd(uint64(maxChunks), r.GetStorageValueAllocateUnits())
		writesOp.MulAdd(uint64(maxChunks), r.GetStorageValueWriteUnits())
	}
	reads, err := readsOp.Value()
	if err != nil {
		return fees.Dimensions{}, err
	}
	allocates, err := allocatesOp.Value()
	if err != nil {
		return fees.Dimensions{}, err
	}
	writes, err := writesOp.Value()
	if err != nil {
		return fees.Dimensions{}, err
	}
	return fees.Dimensions{uint64(t.Size()), maxComputeUnits, reads, allocates, writes}, nil
}

func (t *Transaction) PreExecute(
	ctx context.Context,
	feeManager *internalfees.Manager,
	bh BalanceHandler,
	r Rules,
	im state.Immutable,
	timestamp int64,
) error {
	if err := t.Base.Execute(r, timestamp); err != nil {
		return err
	}
	if len(t.Actions) > int(r.GetMaxActionsPerTx()) {
		return ErrTooManyActions
	}
	for i, action := range t.Actions {
		start, end := action.ValidRange(r)
		if start >= 0 && timestamp < start {
			return fmt.Errorf("%w: action type %d at index %d", ErrActionNotActivated, action.GetTypeID(), i)
		}
		if end >= 0 && timestamp > end {
			return fmt.Errorf("%w: action type %d at index %d", ErrActionNotActivated, action.GetTypeID(), i)
		}
	}
	start, end := t.Auth.ValidRange(r)
	if start >= 0 && timestamp < start {
		return ErrAuthNotActivated
	}
	if end >= 0 && timestamp > end {
		return ErrAuthNotActivated
	}
	units, err := t.Units(bh, r)
	if err != nil {
		return err
	}
	fee, err := feeManager.Fee(units)
	if err != nil {
		return err
	}
	return bh.CanDeduct(ctx, t.Auth.Sponsor(), im, fee)
}

// Execute after knowing a transaction can pay a fee. Attempt
// to charge the fee in as many cases as possible.
//
// Invariant: [PreExecute] is called just before [Execute]
func (t *Transaction) Execute(
	ctx context.Context,
	feeManager *internalfees.Manager,
	bh BalanceHandler,
	r Rules,
	ts *tstate.TStateView,
	timestamp int64,
) (*Result, error) {
	// Always charge fee first
	units, err := t.Units(bh, r)
	if err != nil {
		// Should never happen
		return nil, fmt.Errorf("failed to calculate tx units: %w", err)
	}
	fee, err := feeManager.Fee(units)
	if err != nil {
		// Should never happen
		return nil, fmt.Errorf("failed to calculate tx fee: %w", err)
	}
	if err := bh.Deduct(ctx, t.Auth.Sponsor(), ts, fee); err != nil {
		// This should never fail for low balance (as we check [CanDeductFee]
		// immediately before).
		return nil, fmt.Errorf("failed to deduct tx fee: %w", err)
	}

	// We create a temp state checkpoint to ensure we don't commit failed actions to state.
	//
	// We should favor reverting over returning an error because the caller won't be charged
	// for a transaction that returns an error.
	var (
		actionStart   = ts.OpIndex()
		actionOutputs = [][]byte{}
	)
	for i, action := range t.Actions {
		actionOutput, err := action.Execute(ctx, r, ts, timestamp, t.Auth.Actor(), CreateActionID(t.GetID(), uint8(i)))
		if err != nil {
			ts.Rollback(ctx, actionStart)
			return &Result{
				Success: false,
				Error:   utils.ErrBytes(err),
				Outputs: actionOutputs,
				Units:   units,
				Fee:     fee,
			}, nil
		}

		actionOutputs = append(actionOutputs, actionOutput)
	}
	return &Result{
		Success: true,
		Error:   []byte{},

		Outputs: actionOutputs,

		Units: units,
		Fee:   fee,
	}, nil
}

// Sponsor is the [codec.Address] that pays fees for this transaction.
func (t *Transaction) GetSponsor() codec.Address { return t.Auth.Sponsor() }

type txJSON struct {
	ID      ids.ID        `json:"id"`
	Actions []codec.Bytes `json:"actions"`
	Auth    codec.Bytes   `json:"auth"`
	Base    Base          `json:"base"`
}

func (t *Transaction) MarshalJSON() ([]byte, error) {
	actionBytes := make([]codec.Bytes, len(t.Actions))
	for i, action := range t.Actions {
		actionBytes[i] = action.Bytes()
	}
	return json.Marshal(txJSON{
		ID:      t.GetID(),
		Actions: actionBytes,
		Auth:    t.Auth.Bytes(),
		Base:    t.Base,
	})
}

func (t *Transaction) UnmarshalJSON(data []byte, parser Parser) error {
	var tx txJSON
	err := json.Unmarshal(data, &tx)
	if err != nil {
		return err
	}

	actions := make([]Action, len(tx.Actions))
	for i, actionBytes := range tx.Actions {
		action, err := parser.ParseAction(actionBytes)
		if err != nil {
			return fmt.Errorf("failed to parse action %x at index %d: %w", actionBytes, i, err)
		}
		actions[i] = action
	}
	auth, err := parser.ParseAuth(tx.Auth)
	if err != nil {
		return fmt.Errorf("failed to parse auth %x: %w", tx.Auth, err)
	}
	unmarshalledTx, err := NewTransaction(tx.Base, actions, auth)
	if err != nil {
		return err
	}
	*t = *unmarshalledTx
	return nil
}

// VerifyAuth verifies that the transaction was signed correctly.
func (t *Transaction) VerifyAuth(ctx context.Context) error {
	msg := t.UnsignedBytes()
	return t.Auth.Verify(ctx, msg)
}

func UnmarshalTx(
	bytes []byte,
	parser Parser,
) (*Transaction, error) {
	reader := canoto.Reader{
		B:       bytes,
		Context: parser,
	}
	tx := &Transaction{}
	if err := tx.UnmarshalCanotoFrom(reader); err != nil {
		return nil, err
	}
	return tx, nil
}

// EstimateUnits provides a pessimistic estimate (some key accesses may be duplicates) of the cost
// to execute a transaction.
//
// This is typically used during transaction construction.
func EstimateUnits(r Rules, actions []Action, authFactory AuthFactory) (fees.Dimensions, error) {
	var (
		bandwidth          = uint64(MaxBaseSize)
		stateKeysMaxChunks = []uint16{} // TODO: preallocate
		computeOp          = math.NewUint64Operator(r.GetBaseComputeUnits())
		readsOp            = math.NewUint64Operator(0)
		allocatesOp        = math.NewUint64Operator(0)
		writesOp           = math.NewUint64Operator(0)
	)

	// Calculate over action/auth
	bandwidth += consts.Uint8Len
	for i, action := range actions {
		actionBytes := action.Bytes()
		actionSize := len(actionBytes)

		actor := authFactory.Address()
		stateKeys := action.StateKeys(actor, CreateActionID(ids.Empty, uint8(i)))
		actionStateKeysMaxChunks, ok := stateKeys.ChunkSizes()
		if !ok {
			return fees.Dimensions{}, ErrInvalidKeyValue
		}
		bandwidth += uint64(actionSize)
		stateKeysMaxChunks = append(stateKeysMaxChunks, actionStateKeysMaxChunks...)
		computeOp.Add(action.ComputeUnits(r))
	}
	authBandwidth, authCompute := authFactory.MaxUnits()
	bandwidth += authBandwidth
	sponsorStateKeyMaxChunks := r.GetSponsorStateKeysMaxChunks()
	stateKeysMaxChunks = append(stateKeysMaxChunks, sponsorStateKeyMaxChunks...)
	computeOp.Add(authCompute)

	// Estimate compute costs
	compute, err := computeOp.Value()
	if err != nil {
		return fees.Dimensions{}, err
	}

	// Estimate storage costs
	for _, maxChunks := range stateKeysMaxChunks {
		// Compute key costs
		readsOp.Add(r.GetStorageKeyReadUnits())
		allocatesOp.Add(r.GetStorageKeyAllocateUnits())
		writesOp.Add(r.GetStorageKeyWriteUnits())

		// Compute value costs
		readsOp.MulAdd(uint64(maxChunks), r.GetStorageValueReadUnits())
		allocatesOp.MulAdd(uint64(maxChunks), r.GetStorageValueAllocateUnits())
		writesOp.MulAdd(uint64(maxChunks), r.GetStorageValueWriteUnits())
	}
	reads, err := readsOp.Value()
	if err != nil {
		return fees.Dimensions{}, err
	}
	allocates, err := allocatesOp.Value()
	if err != nil {
		return fees.Dimensions{}, err
	}
	writes, err := writesOp.Value()
	if err != nil {
		return fees.Dimensions{}, err
	}
	return fees.Dimensions{bandwidth, compute, reads, allocates, writes}, nil
}

func (t *Transaction) MarshalCanotoInto(w canoto.Writer) canoto.Writer {
	canoto.Append(&w, t.bytes)
	return w
}

// CalculateCanotoCache is a no-op for [Transaction] because it is immutable
// and already cached in the internal bytes field.
func (*Transaction) CalculateCanotoCache() {}

func (t *Transaction) CachedCanotoSize() uint64 { return uint64(t.size) }

func (t *Transaction) UnmarshalCanotoFrom(r canoto.Reader) error {
	serializeTx := &SerializeTx{}
	if err := serializeTx.UnmarshalCanotoFrom(r); err != nil {
		return err
	}

	parser, ok := r.Context.(Parser)
	if !ok {
		return fmt.Errorf("failed to extract Parser from canoto context of type: %T", r.Context)
	}
	actions := make([]Action, len(serializeTx.Actions))
	for i, actionBytes := range serializeTx.Actions {
		action, err := parser.ParseAction(actionBytes)
		if err != nil {
			return fmt.Errorf("failed to parse action %x at index %d: %w", actionBytes, i, err)
		}
		actions[i] = action
	}

	auth, err := parser.ParseAuth(serializeTx.Auth)
	if err != nil {
		return fmt.Errorf("failed to parse auth %x: %w", serializeTx.Auth, err)
	}

	// We do not assume that the auth field is non-zero le
	var unsignedTxBytes []byte
	// If the auth field is zero-length, then the unsigned tx bytes are identical to the
	// bytes of the transaction. This is an unexpected case because it's unlikely an auth
	// parser would allow a zero-length byte slice to be parsed as a valid auth and should
	// error above. However, we do not assume a zero-length auth field is invalid, so we
	// handle the case here.
	if len(serializeTx.Auth) == 0 {
		unsignedTxBytes = r.B
	} else {
		authSuffixSize := len(canoto__SerializeTx__Auth__tag) + int(canoto.SizeBytes(serializeTx.Auth))
		unsignedTxBytesLimit := len(r.B) - authSuffixSize
		// Defensive: check to ensure the calculated auth suffix size is within expected bounds
		// and return an error rather than panic on index out of bounds if not.
		if unsignedTxBytesLimit < 0 || unsignedTxBytesLimit > len(r.B) {
			return fmt.Errorf("failed to extract unsigned tx bytes due to invalid offset: %d, tx size: %d auth suffix size: %d",
				unsignedTxBytesLimit,
				len(r.B),
				authSuffixSize,
			)
		}
		unsignedTxBytes = r.B[:len(r.B)-authSuffixSize]
	}

	tx := &Transaction{
		TransactionData: TransactionData{
			Base:          serializeTx.Base,
			Actions:       actions,
			unsignedBytes: unsignedTxBytes,
		},
		Auth: auth,
	}
	tx.bytes = r.B
	tx.size = len(tx.bytes)
	tx.id = utils.ToID(tx.bytes)
	*t = *tx

	return nil
}

func (*Transaction) ValidCanoto() bool { return true }

// CanotoSpec returns the specification of this canoto message.
// Required for canoto.Field interface implementation.
// Delegates to SerializeTx since Transaction uses it for serialization.
func (*Transaction) CanotoSpec(types ...reflect.Type) *canoto.Spec {
	serializeTx := &SerializeTx{}
	return serializeTx.CanotoSpec(types...)
}

func GenerateTransaction(
	ruleFactory RuleFactory,
	unitPrices fees.Dimensions,
	timestamp int64,
	actions []Action,
	authFactory AuthFactory,
) (*Transaction, error) {
	rules := ruleFactory.GetRules(timestamp)
	units, err := EstimateUnits(rules, actions, authFactory)
	if err != nil {
		return nil, err
	}
	maxFee, err := fees.MulSum(unitPrices, units)
	if err != nil {
		return nil, err
	}
	tx, err := GenerateTransactionManual(rules, timestamp, actions, authFactory, maxFee)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func GenerateTransactionManual(
	rules Rules,
	timestamp int64,
	actions []Action,
	authFactory AuthFactory,
	maxFee uint64,
) (*Transaction, error) {
	unsignedTx := NewTxData(
		Base{
			Timestamp: utils.UnixRMilli(timestamp, rules.GetValidityWindow()),
			ChainID:   rules.GetChainID(),
			MaxFee:    maxFee,
		},
		actions,
	)
	tx, err := unsignedTx.Sign(authFactory)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to sign transaction", err)
	}
	return tx, nil
}
