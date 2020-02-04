package miner

import (
	"math/big"
	"reflect"

	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/peer"
	xerrors "github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/proofs/verification"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/errors"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/dispatch"
	internal "github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/errors"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/pattern"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
)

func init() {
	encoding.RegisterIpldCborType(cbor.BigIntAtlasEntry)
}

// Actor is the miner actor.
//
// If `Bootstrap` is `true`, the miner will not verify seal proofs. This is
// useful when testing, as miners with non-zero power can be created using bogus
// commitments. This is a temporary measure; we want to ultimately be able to
// create a real genesis block whose miners are seeded with real commitments.
//
// The `Bootstrap` field must be set to `true` if the miner was created in the
// genesis block. If the miner was created in any other block, `Bootstrap` must
// be false.
type Actor struct {
	Bootstrap bool
}

// State is the miner actors storage.
type State struct {
	// Owner is the address of the account that owns this miner. Income and returned
	// collateral are paid to this address. This address is also allowed todd change the
	// worker address for the miner.
	Owner address.Address

	// Worker is the address of the worker account for this miner.
	// This will be the key that is used to sign blocks created by this miner, and
	// sign messages sent on behalf of this miner to commit sectors, submit PoSts, and
	// other day to day miner activities.
	Worker address.Address

	// PeerID references the libp2p identity that the miner is operating.
	PeerID peer.ID

	// ActiveCollateral is the amount of collateral currently committed to live
	// storage.
	ActiveCollateral types.AttoFIL

	// Asks is the set of asks this miner has open
	Asks      []*Ask
	NextAskID *big.Int

	// SectorCommitments maps sector id to commitments, for all sectors this
	// miner has committed.  Sector ids are removed from this collection
	// when they are included in the done or fault parameters of submitPoSt.
	// Due to a bug in refmt, the sector id-keys need to be
	// stringified.
	//
	// See also: https://github.com/polydawn/refmt/issues/35
	SectorCommitments SectorSet

	// Faults reported since last PoSt
	CurrentFaultSet types.IntSet

	// Faults reported since last PoSt, but too late to be included in the current PoSt
	NextFaultSet types.IntSet

	// NextDoneSet is a set of sector ids reported during the last PoSt
	// submission as being 'done'.  The collateral for them is still being
	// held until the next PoSt submission in case early sector removal
	// penalization is needed.
	NextDoneSet types.IntSet

	// ProvingSet is the set of sector ids of sectors this miner is
	// currently required to prove.
	ProvingSet types.IntSet

	LastUsedSectorID uint64

	// ProvingPeriodEnd is the block height at the end of the current proving period.
	// This is the last round in which a proof will be considered to be on-time.
	ProvingPeriodEnd *types.BlockHeight

	// The amount of space proven to the network by this miner in the
	// latest proving period.
	Power *types.BytesAmount

	// SectorSize is the amount of space in each sector committed to the network
	// by this miner.
	SectorSize *types.BytesAmount

	// SlashedSet is a set of sector ids that have been slashed
	SlashedSet types.IntSet

	// SlashedAt is the time at which this miner was slashed
	SlashedAt *types.BlockHeight

	// OwedStorageCollateral is the collateral for sectors that have been slashed.
	// This collateral can be collected from arbitrated deals, but not de-pledged.
	OwedStorageCollateral types.AttoFIL
}

// View is a readonly view into the actor state
type View struct {
	state State
	store runtime.LegacyStorage
}

// Ask is a price advertisement by the miner
type Ask struct {
	Price  types.AttoFIL
	Expiry *types.BlockHeight
	ID     *big.Int
}

// Actor methods
const (
	Constructor types.MethodID = 1
	AddAsk      types.MethodID = iota + 2
	GetOwner
	CommitSector
	GetWorker
	GetPeerID
	UpdatePeerID
	GetPower
	AddFaults
	SubmitPoSt
	SlashStorageFault
	ChangeWorker
	VerifyPieceInclusion
	GetSectorSize
	GetAsks
	GetAsk
	GetLastUsedSectorID
	GetProvingSetCommitments
	IsBootstrapMiner
	GetPoStState
	GetProvingWindow
	CalculateLateFee
	GetActiveCollateral
)

// NewActor returns a new miner actor with the provided balance.
func NewActor() *actor.Actor {
	return actor.NewActor(types.MinerActorCodeCid, types.ZeroAttoFIL)
}

// NewState creates a miner state struct
func NewState(owner, worker address.Address, pid peer.ID, sectorSize *types.BytesAmount) *State {
	return &State{
		Owner:                 owner,
		Worker:                worker,
		PeerID:                pid,
		ActiveCollateral:      types.ZeroAttoFIL,
		Asks:                  []*Ask{},
		NextAskID:             big.NewInt(0),
		SectorCommitments:     NewSectorSet(),
		CurrentFaultSet:       types.EmptyIntSet(),
		NextFaultSet:          types.EmptyIntSet(),
		NextDoneSet:           types.EmptyIntSet(),
		ProvingSet:            types.EmptyIntSet(),
		LastUsedSectorID:      0,
		ProvingPeriodEnd:      types.NewBlockHeight(0),
		Power:                 types.NewBytesAmount(0),
		SectorSize:            sectorSize,
		SlashedSet:            types.EmptyIntSet(),
		SlashedAt:             types.NewBlockHeight(0),
		OwedStorageCollateral: types.ZeroAttoFIL,
	}
}

// NewView creates a new init actor state view.
func NewView(stateHandle runtime.ReadonlyActorStateHandle, store runtime.LegacyStorage) View {
	// load state as readonly
	var state State
	stateHandle.Readonly(&state)
	// return view
	return View{
		state: state,
		store: store,
	}
}

// Owner returns the address for the miner ownner.
func (view *View) Owner() address.Address {
	return view.state.Owner
}

//
// ExecutableActor impl for Actor
//

var _ dispatch.ExecutableActor = (*Actor)(nil)

var signatures = dispatch.Exports{
	Constructor: &dispatch.FunctionSignature{
		Params: []abi.Type{abi.Address, abi.Address, abi.PeerID, abi.BytesAmount},
		Return: []abi.Type{},
	},
	// addAsk is not in the spec, but there's not yet another mechanism to discover asks.
	AddAsk: &dispatch.FunctionSignature{
		Params: []abi.Type{abi.AttoFIL, abi.Integer},
		Return: []abi.Type{abi.Integer},
	},
	GetOwner: &dispatch.FunctionSignature{
		Params: nil,
		Return: []abi.Type{abi.Address},
	},
	CommitSector: &dispatch.FunctionSignature{
		Params: []abi.Type{abi.SectorID, abi.Bytes, abi.Bytes, abi.Bytes, abi.PoRepProof},
		Return: []abi.Type{},
	},
	GetWorker: &dispatch.FunctionSignature{
		Params: []abi.Type{},
		Return: []abi.Type{abi.Address},
	},
	GetPeerID: &dispatch.FunctionSignature{
		Params: []abi.Type{},
		Return: []abi.Type{abi.PeerID},
	},
	UpdatePeerID: &dispatch.FunctionSignature{
		Params: []abi.Type{abi.PeerID},
		Return: []abi.Type{},
	},
	GetPower: &dispatch.FunctionSignature{
		Params: []abi.Type{},
		Return: []abi.Type{abi.BytesAmount},
	},
	AddFaults: &dispatch.FunctionSignature{
		Params: []abi.Type{abi.FaultSet},
		Return: []abi.Type{},
	},
	SubmitPoSt: &dispatch.FunctionSignature{
		Params: []abi.Type{abi.PoStProof, abi.FaultSet, abi.IntSet},
		Return: []abi.Type{},
	},
	SlashStorageFault: &dispatch.FunctionSignature{
		Params: []abi.Type{},
		Return: []abi.Type{},
	},
	ChangeWorker: &dispatch.FunctionSignature{
		Params: []abi.Type{abi.Address},
		Return: []abi.Type{},
	},
	// verifyPieceInclusion is not in spec, but should be.
	VerifyPieceInclusion: &dispatch.FunctionSignature{
		Params: []abi.Type{abi.Bytes, abi.BytesAmount, abi.SectorID, abi.Bytes},
		Return: []abi.Type{},
	},
	GetSectorSize: &dispatch.FunctionSignature{
		Params: nil,
		Return: []abi.Type{abi.BytesAmount},
	},

	// Non-exported methods below here.
	// These methods are not part of the actor's protocol specification and should not be exported,
	// but are because we lack a mechanism to invoke actor methods without going through the
	// queryMessage infrastructure. These should be removed when we have another way of invoking
	// them from worker code. https://github.com/filecoin-project/go-filecoin/issues/2973
	GetAsks: &dispatch.FunctionSignature{
		Params: nil,
		Return: []abi.Type{abi.UintArray},
	},
	GetAsk: &dispatch.FunctionSignature{
		Params: []abi.Type{abi.Integer},
		Return: []abi.Type{abi.Bytes},
	},
	GetLastUsedSectorID: &dispatch.FunctionSignature{
		Params: nil,
		Return: []abi.Type{abi.SectorID},
	},
	GetProvingSetCommitments: &dispatch.FunctionSignature{
		Params: nil,
		Return: []abi.Type{abi.CommitmentsMap},
	},
	IsBootstrapMiner: &dispatch.FunctionSignature{
		Params: nil,
		Return: []abi.Type{abi.Boolean},
	},
	GetPoStState: &dispatch.FunctionSignature{
		Params: nil,
		Return: []abi.Type{abi.Integer},
	},
	GetProvingWindow: &dispatch.FunctionSignature{
		Params: []abi.Type{},
		Return: []abi.Type{abi.UintArray},
	},
	CalculateLateFee: &dispatch.FunctionSignature{
		Params: []abi.Type{abi.BlockHeight},
		Return: []abi.Type{abi.AttoFIL},
	},
	GetActiveCollateral: &dispatch.FunctionSignature{
		Params: []abi.Type{},
		Return: []abi.Type{abi.AttoFIL},
	},
}

// Method returns method definition for a given method id.
func (a *Actor) Method(id types.MethodID) (dispatch.Method, *dispatch.FunctionSignature, bool) {
	switch id {
	case Constructor:
		return reflect.ValueOf((*Impl)(a).Constructor), signatures[Constructor], true
	case AddAsk:
		return reflect.ValueOf((*Impl)(a).AddAsk), signatures[AddAsk], true
	case GetOwner:
		return reflect.ValueOf((*Impl)(a).GetOwner), signatures[GetOwner], true
	case CommitSector:
		return reflect.ValueOf((*Impl)(a).CommitSector), signatures[CommitSector], true
	case GetWorker:
		return reflect.ValueOf((*Impl)(a).GetWorker), signatures[GetWorker], true
	case GetPeerID:
		return reflect.ValueOf((*Impl)(a).GetPeerID), signatures[GetPeerID], true
	case UpdatePeerID:
		return reflect.ValueOf((*Impl)(a).UpdatePeerID), signatures[UpdatePeerID], true
	case GetPower:
		return reflect.ValueOf((*Impl)(a).GetPower), signatures[GetPower], true
	case AddFaults:
		return reflect.ValueOf((*Impl)(a).AddFaults), signatures[AddFaults], true
	case SubmitPoSt:
		return reflect.ValueOf((*Impl)(a).SubmitPoSt), signatures[SubmitPoSt], true
	case SlashStorageFault:
		return reflect.ValueOf((*Impl)(a).SlashStorageFault), signatures[SlashStorageFault], true
	case ChangeWorker:
		return reflect.ValueOf((*Impl)(a).ChangeWorker), signatures[ChangeWorker], true
	case GetSectorSize:
		return reflect.ValueOf((*Impl)(a).GetSectorSize), signatures[GetSectorSize], true
	case GetAsks:
		return reflect.ValueOf((*Impl)(a).GetAsks), signatures[GetAsks], true
	case GetAsk:
		return reflect.ValueOf((*Impl)(a).GetAsk), signatures[GetAsk], true
	case GetLastUsedSectorID:
		return reflect.ValueOf((*Impl)(a).GetLastUsedSectorID), signatures[GetLastUsedSectorID], true
	case GetProvingSetCommitments:
		return reflect.ValueOf((*Impl)(a).GetProvingSetCommitments), signatures[GetProvingSetCommitments], true
	case IsBootstrapMiner:
		return reflect.ValueOf((*Impl)(a).IsBootstrapMiner), signatures[IsBootstrapMiner], true
	case GetPoStState:
		return reflect.ValueOf((*Impl)(a).GetPoStState), signatures[GetPoStState], true
	case GetProvingWindow:
		return reflect.ValueOf((*Impl)(a).GetProvingWindow), signatures[GetProvingWindow], true
	case CalculateLateFee:
		return reflect.ValueOf((*Impl)(a).CalculateLateFee), signatures[CalculateLateFee], true
	case GetActiveCollateral:
		return reflect.ValueOf((*Impl)(a).GetActiveCollateral), signatures[GetActiveCollateral], true
	default:
		return nil, nil, false
	}
}

// InitializeState stores this miner's initial data structure.
func (*Actor) InitializeState(handle runtime.ActorStateHandle, initializerData interface{}) error {
	minerState, ok := initializerData.(*State)
	if !ok {
		return errors.NewFaultError("Initial state to miner actor is not a miner.State struct")
	}

	handle.Create(minerState)

	return nil
}

//
// vm methods for actor
//

// Impl is the VM implementation of the actor.
type Impl Actor

var log = logging.Logger("actor.miner")

var Storagemarket_UpdateStorage = types.MethodID(1 + 32)
var Storagemarket_GetProofsMode = types.MethodID(3 + 32)

// LargestSectorSizeProvingPeriodBlocks defines the number of blocks in a
// proving period for a miner configured to use the largest sector size
// supported by the network.
//
// TODO: If the following PR is merged - and the network doesn't define a
// largest sector size - this constant and consensus.AncestorRoundsNeeded will
// need to be reconsidered.
// https://github.com/filecoin-project/specs/pull/318
const LargestSectorSizeProvingPeriodBlocks = 300

// PoStChallengeWindowBlocks defines the block time prior to the proving
// period end at which the PoSt challenge seed is chosen. This dictates the
// earliest point at which a PoSt may be submitted.
const PoStChallengeWindowBlocks = 150

// MinimumCollateralPerSector is the minimum amount of collateral required per sector
var MinimumCollateralPerSector, _ = types.NewAttoFILFromFILString("0.001")

const (
	// ErrInvalidSector indicates and invalid sector id.
	ErrInvalidSector = 34
	// ErrSectorIDInUse indicates a sector has already been committed at this ID.
	ErrSectorIDInUse = 35
	// ErrStoragemarketCallFailed indicates the call to commit the deal failed.
	ErrStoragemarketCallFailed = 36
	// ErrCallerUnauthorized signals an unauthorized caller.
	ErrCallerUnauthorized = 37
	// ErrInsufficientPledge signals insufficient pledge for what you are trying to do.
	ErrInsufficientPledge = 38
	// ErrInvalidPoSt signals that the passed in PoSt was invalid.
	ErrInvalidPoSt = 39
	// ErrAskNotFound indicates that no ask was found with the given ID.
	ErrAskNotFound = 40
	// ErrInvalidSealProof signals that the passed in seal proof was invalid.
	ErrInvalidSealProof = 41
	// ErrGetProofsModeFailed indicates the call to get the proofs mode failed.
	ErrGetProofsModeFailed = 42
	// ErrInsufficientCollateral indicates that the miner does not have sufficient collateral to commit additional sectors.
	ErrInsufficientCollateral = 43
	// ErrMinerAlreadySlashed indicates that an attempt has been made to slash an already slashed miner
	ErrMinerAlreadySlashed = 44
	// ErrMinerNotSlashable indicates that an attempt has been made to slash a miner that does not meet the criteria for slashing.
	ErrMinerNotSlashable = 45
	// ErrInvalidPieceInclusionProof indicates that the piece inclusion proof was
	// malformed or did not succesfully verify.
	ErrInvalidPieceInclusionProof = 46
)

// Errors map error codes to revert errors this actor may return.
var Errors = map[uint8]error{
	ErrInvalidSector:              errors.NewCodedRevertErrorf(ErrInvalidSector, "sectorID out of range"),
	ErrSectorIDInUse:              errors.NewCodedRevertErrorf(ErrSectorIDInUse, "sector already committed at this ID"),
	ErrStoragemarketCallFailed:    errors.NewCodedRevertErrorf(ErrStoragemarketCallFailed, "call to StorageMarket failed"),
	ErrCallerUnauthorized:         errors.NewCodedRevertErrorf(ErrCallerUnauthorized, "not authorized to call the method"),
	ErrInsufficientPledge:         errors.NewCodedRevertErrorf(ErrInsufficientPledge, "not enough pledged"),
	ErrInvalidPoSt:                errors.NewCodedRevertErrorf(ErrInvalidPoSt, "PoSt proof did not validate"),
	ErrAskNotFound:                errors.NewCodedRevertErrorf(ErrAskNotFound, "no ask was found"),
	ErrInvalidSealProof:           errors.NewCodedRevertErrorf(ErrInvalidSealProof, "seal proof was invalid"),
	ErrGetProofsModeFailed:        errors.NewCodedRevertErrorf(ErrGetProofsModeFailed, "failed to get proofs mode"),
	ErrInsufficientCollateral:     errors.NewCodedRevertErrorf(ErrInsufficientCollateral, "insufficient collateral"),
	ErrInvalidPieceInclusionProof: errors.NewCodedRevertErrorf(ErrInvalidPieceInclusionProof, "piece inclusion proof did not validate"),
}

const (
	PoStStateNoStorage = iota
	PoStStateWithinProvingPeriod
	PoStStateAfterProvingPeriod
	PoStStateUnrecoverable
)

// minerInvocationContext has some special sauce for the miner.
type invocationContext interface {
	runtime.InvocationContext
	LegacyVerifier() verification.Verifier
	LegacyMessage() *types.UnsignedMessage
}

// Constructor initializes the actor's state
func (impl *Impl) Constructor(ctx runtime.InvocationContext, owner, worker address.Address, pid peer.ID, sectorSize *types.BytesAmount) (uint8, error) {
	ctx.ValidateCaller(pattern.IsAInitActor{})

	err := (*Actor)(impl).InitializeState(ctx.StateHandle(), NewState(owner, worker, pid, sectorSize))
	if err != nil {
		return errors.CodeError(err), err
	}
	return 0, nil
}

// AddAsk adds an ask to this miners ask list
func (*Impl) AddAsk(ctx invocationContext, price types.AttoFIL, expiry *big.Int) (*big.Int, uint8,
	error) {
	if err := ctx.Charge(actor.DefaultGasCost); err != nil {
		return nil, internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	var state State
	out, err := actor.WithState(ctx, &state, func() (interface{}, error) {
		if ctx.Message().Caller() != state.Worker {
			return nil, Errors[ErrCallerUnauthorized]
		}

		id := big.NewInt(0).Set(state.NextAskID)
		state.NextAskID = state.NextAskID.Add(state.NextAskID, big.NewInt(1))

		epoch := ctx.Runtime().CurrentEpoch()

		// filter out expired asks
		asks := state.Asks
		state.Asks = state.Asks[:0]
		for _, a := range asks {
			if epoch.LessThan(a.Expiry) {
				state.Asks = append(state.Asks, a)
			}
		}

		if !expiry.IsUint64() {
			return nil, errors.NewRevertError("expiry was invalid")
		}
		expiryBH := types.NewBlockHeight(expiry.Uint64())

		state.Asks = append(state.Asks, &Ask{
			Price:  price,
			Expiry: epoch.Add(expiryBH),
			ID:     id,
		})

		return id, nil
	})
	if err != nil {
		return nil, errors.CodeError(err), err
	}

	askID, ok := out.(*big.Int)
	if !ok {
		return nil, 1, errors.NewRevertErrorf("expected an Integer return value from call, but got %T instead", out)
	}

	return askID, 0, nil
}

// GetAsks returns all the asks for this miner.
func (*Impl) GetAsks(ctx invocationContext) ([]types.Uint64, uint8, error) {
	if err := ctx.Charge(actor.DefaultGasCost); err != nil {
		return nil, internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}
	var state State
	out, err := actor.WithState(ctx, &state, func() (interface{}, error) {
		var askids []types.Uint64
		for _, ask := range state.Asks {
			if !ask.ID.IsUint64() {
				return nil, errors.NewFaultErrorf("miner ask has invalid ID (bad invariant)")
			}
			askids = append(askids, types.Uint64(ask.ID.Uint64()))
		}

		return askids, nil
	})
	if err != nil {
		return nil, errors.CodeError(err), err
	}

	askids, ok := out.([]types.Uint64)
	if !ok {
		return nil, 1, errors.NewRevertErrorf("expected a []types.Uint64 return value from call, but got %T instead", out)
	}

	return askids, 0, nil
}

// GetAsk returns an ask by ID
func (*Impl) GetAsk(ctx invocationContext, askid *big.Int) ([]byte, uint8, error) {
	if err := ctx.Charge(actor.DefaultGasCost); err != nil {
		return nil, internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	var state State
	out, err := actor.WithState(ctx, &state, func() (interface{}, error) {
		var ask *Ask
		for _, a := range state.Asks {
			if a.ID.Cmp(askid) == 0 {
				ask = a
				break
			}
		}

		if ask == nil {
			return nil, Errors[ErrAskNotFound]
		}

		out, err := encoding.Encode(ask)
		if err != nil {
			return nil, err
		}

		return out, nil
	})
	if err != nil {
		return nil, errors.CodeError(err), err
	}

	ask, ok := out.([]byte)
	if !ok {
		return nil, 1, errors.NewRevertErrorf("expected a Bytes return value from call, but got %T instead", out)
	}

	return ask, 0, nil
}

// GetOwner returns the miners owner.
func (*Impl) GetOwner(ctx invocationContext) (address.Address, uint8, error) {
	if err := ctx.Charge(actor.DefaultGasCost); err != nil {
		return address.Undef, internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	var state State
	out, err := actor.WithState(ctx, &state, func() (interface{}, error) {
		return state.Owner, nil
	})
	if err != nil {
		return address.Undef, errors.CodeError(err), err
	}

	a, ok := out.(address.Address)
	if !ok {
		return address.Undef, 1, errors.NewFaultErrorf("expected an Address return value from call, but got %T instead", out)
	}

	return a, 0, nil
}

// GetLastUsedSectorID returns the last used sector id.
func (*Impl) GetLastUsedSectorID(ctx invocationContext) (uint64, uint8, error) {
	if err := ctx.Charge(actor.DefaultGasCost); err != nil {
		return 0, internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}
	var state State
	out, err := actor.WithState(ctx, &state, func() (interface{}, error) {
		return state.LastUsedSectorID, nil
	})
	if err != nil {
		return 0, errors.CodeError(err), err
	}

	a, ok := out.(uint64)
	if !ok {
		return 0, 1, errors.NewFaultErrorf("expected a uint64 sector id, but got %T instead", out)
	}

	return a, 0, nil
}

// IsBootstrapMiner indicates whether the receiving miner was created in the
// genesis block, i.e. used to bootstrap the network
func (a *Impl) IsBootstrapMiner(ctx invocationContext) (bool, uint8, error) {
	return a.Bootstrap, 0, nil
}

// GetPoStState returns whether the miner's last submitPoSt is within the proving period,
// late or after the generation attack threshold.
func (*Impl) GetPoStState(ctx invocationContext) (*big.Int, uint8, error) {
	var state State
	out, err := actor.WithState(ctx, &state, func() (interface{}, error) {
		// Don't check lateness unless there is storage to prove
		if state.ProvingSet.Size() == 0 {
			return int64(PoStStateNoStorage), nil
		}
		epoch := ctx.Runtime().CurrentEpoch()
		lateState, _ := lateState(state.ProvingPeriodEnd, &epoch, LatePoStGracePeriod(state.SectorSize))
		return lateState, nil
	})

	if err != nil {
		return nil, errors.CodeError(err), err
	}

	result, ok := out.(int64)
	if !ok {
		return nil, 1, errors.NewFaultErrorf("expected a int64, but got %T instead", out)
	}

	return big.NewInt(result), 0, nil
}

// GetProvingSetCommitments returns all sector commitments posted by this miner.
func (*Impl) GetProvingSetCommitments(ctx invocationContext) (map[string]types.Commitments, uint8, error) {
	if err := ctx.Charge(actor.DefaultGasCost); err != nil {
		return nil, internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	var state State
	err := actor.ReadState(ctx, &state)
	if err != nil {
		return map[string]types.Commitments{}, errors.CodeError(err), err
	}

	commitments := NewSectorSet()
	for _, sectorID := range state.ProvingSet.Values() {
		c, found := state.SectorCommitments.Get(sectorID)
		if !found {
			return map[string]types.Commitments{}, 1, errors.NewFaultErrorf("proving set id, %d, missing in sector commitments", sectorID)
		}
		commitments.Add(sectorID, c)
	}
	return (map[string]types.Commitments)(commitments), 0, nil
}

// GetSectorSize returns the size of the sectors committed to the network by
// this miner.
func (*Impl) GetSectorSize(ctx invocationContext) (*types.BytesAmount, uint8, error) {
	if err := ctx.Charge(actor.DefaultGasCost); err != nil {
		return nil, internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	var state State
	out, err := actor.WithState(ctx, &state, func() (interface{}, error) {
		return state.SectorSize, nil
	})
	if err != nil {
		return nil, errors.CodeError(err), err
	}

	amt, ok := out.(*types.BytesAmount)
	if !ok {
		return nil, 1, errors.NewFaultErrorf("expected a *types.BytesAmount, but got %T instead", out)
	}

	return amt, 0, nil
}

// CommitSector adds a commitment to the specified sector. The sector must not
// already be committed.
func (a *Impl) CommitSector(ctx invocationContext, sectorID uint64, commD, commR, commRStar []byte, proof types.PoRepProof) (uint8, error) {
	if err := ctx.Charge(actor.DefaultGasCost); err != nil {
		return internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}
	if len(commD) != int(types.CommitmentBytesLen) {
		return 1, errors.NewRevertError("invalid sized commD")
	}
	if len(commR) != int(types.CommitmentBytesLen) {
		return 1, errors.NewRevertError("invalid sized commR")
	}
	if len(commRStar) != int(types.CommitmentBytesLen) {
		return 1, errors.NewRevertError("invalid sized commRStar")
	}

	var state State
	_, err := actor.WithState(ctx, &state, func() (interface{}, error) {
		// As with submitPoSt messages, bootstrap miner actors don't verify
		// the commitSector messages that they are sent.
		//
		// This switching will be removed when issue #2270 is completed.
		if !a.Bootstrap {
			var commRAry [32]byte
			copy(commRAry[:], commR)

			var commDAry [32]byte
			copy(commDAry[:], commD)

			var proverID [32]byte
			copy(proverID[:], ctx.LegacyMessage().To.Bytes()[:])

			var ticket [32]byte
			panic("need a ticket")

			var seed [32]byte
			panic("need a seed")

			isValid, err := ctx.LegacyVerifier().VerifySeal(state.SectorSize.Uint64(), commRAry, commDAry, proverID, ticket, seed, sectorID, proof[:])
			if err != nil {
				return nil, errors.RevertErrorWrap(err, "failed to verify seal proof")
			}
			if !isValid {
				return nil, Errors[ErrInvalidSealProof]
			}
		}

		// verify that the caller is authorized to perform update
		if ctx.Message().Caller() != state.Worker {
			return nil, Errors[ErrCallerUnauthorized]
		}

		if state.SectorCommitments.Has(sectorID) {
			return nil, Errors[ErrSectorIDInUse]
		}

		// make sure the miner has enough collateral to add more storage
		collateral := CollateralForSector(state.SectorSize)
		if collateral.GreaterThan(ctx.Balance().Sub(state.ActiveCollateral)) {
			return nil, Errors[ErrInsufficientCollateral]
		}

		state.ActiveCollateral = state.ActiveCollateral.Add(collateral)

		// Case 1: If the miner is not currently proving any sectors,
		// start proving immediately on this sector.
		//
		// Case 2: If the miner is adding sectors during genesis
		// construction all committed sectors accumulate in their
		// proving set.  This  allows us to add power immediately in
		// genesis with commitSector and submitPoSt calls without
		// adding special casing for bootstrappers.
		epoch := ctx.Runtime().CurrentEpoch()
		if state.ProvingSet.Size() == 0 || epoch.Equal(types.NewBlockHeight(0)) {
			state.ProvingSet = state.ProvingSet.Add(sectorID)
			state.ProvingPeriodEnd = epoch.Add(types.NewBlockHeight(ProvingPeriodDuration(state.SectorSize)))
		}
		comms := types.Commitments{
			CommD:     types.CommD{},
			CommR:     types.CommR{},
			CommRStar: types.CommRStar{},
		}
		copy(comms.CommD[:], commD)
		copy(comms.CommR[:], commR)
		copy(comms.CommRStar[:], commRStar)

		state.LastUsedSectorID = sectorID
		state.SectorCommitments.Add(sectorID, comms)
		return nil, nil
	})
	if err != nil {
		return errors.CodeError(err), err
	}

	return 0, nil
}

// ChangeWorker alters the worker address in state
func (*Impl) ChangeWorker(ctx invocationContext, worker address.Address) (uint8, error) {
	if err := ctx.Charge(actor.DefaultGasCost); err != nil {
		return internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	var state State
	_, err := actor.WithState(ctx, &state, func() (interface{}, error) {
		if ctx.Message().Caller() != state.Owner {
			return nil, Errors[ErrCallerUnauthorized]
		}

		state.Worker = worker

		return nil, nil
	})
	if err != nil {
		return errors.CodeError(err), err
	}

	return 0, nil
}

// GetWorker returns the worker address for this miner.
func (*Impl) GetWorker(ctx invocationContext) (address.Address, uint8, error) {
	if err := ctx.Charge(actor.DefaultGasCost); err != nil {
		return address.Address{}, internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	var state State
	out, err := actor.WithState(ctx, &state, func() (interface{}, error) {
		return state.Worker, nil
	})
	if err != nil {
		return address.Address{}, errors.CodeError(err), err
	}

	validOut, ok := out.(address.Address)
	if !ok {
		return address.Address{}, 1, errors.NewRevertError("expected an address")
	}

	return validOut, 0, nil
}

// GetPeerID returns the libp2p peer ID that this miner can be reached at.
func (*Impl) GetPeerID(ctx invocationContext) (peer.ID, uint8, error) {
	if err := ctx.Charge(actor.DefaultGasCost); err != nil {
		return peer.ID(""), internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	var state State

	err := actor.ReadState(ctx, &state)
	if err != nil {
		return peer.ID(""), errors.CodeError(err), err
	}

	return state.PeerID, 0, nil
}

// UpdatePeerID is used to update the peerID this miner is operating under.
func (*Impl) UpdatePeerID(ctx invocationContext, pid peer.ID) (uint8, error) {
	if err := ctx.Charge(actor.DefaultGasCost); err != nil {
		return internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	var storage State
	_, err := actor.WithState(ctx, &storage, func() (interface{}, error) {
		// verify that the caller is authorized to perform update
		if ctx.Message().Caller() != storage.Worker {
			return nil, Errors[ErrCallerUnauthorized]
		}

		storage.PeerID = pid

		return nil, nil
	})
	if err != nil {
		return errors.CodeError(err), err
	}

	return 0, nil
}

// GetPower returns the amount of proven sectors for this miner.
func (*Impl) GetPower(ctx invocationContext) (*types.BytesAmount, uint8, error) {
	if err := ctx.Charge(actor.DefaultGasCost); err != nil {
		return nil, internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	var state State
	ret, err := actor.WithState(ctx, &state, func() (interface{}, error) {
		return state.Power, nil
	})
	if err != nil {
		return nil, errors.CodeError(err), err
	}

	power, ok := ret.(*types.BytesAmount)
	if !ok {
		return nil, 1, errors.NewFaultErrorf("expected *types.BytesAmount to be returned, but got %T instead", ret)
	}

	return power, 0, nil
}

// GetActiveCollateral returns the active collateral a miner is holding to
// protect storage.
func (*Impl) GetActiveCollateral(ctx invocationContext) (types.AttoFIL, uint8, error) {
	if err := ctx.Charge(actor.DefaultGasCost); err != nil {
		return types.ZeroAttoFIL, internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}
	var state State
	ret, err := actor.WithState(ctx, &state, func() (interface{}, error) {
		return state.ActiveCollateral, nil
	})
	if err != nil {
		return types.ZeroAttoFIL, errors.CodeError(err), err
	}

	collateral, ok := ret.(types.AttoFIL)
	if !ok {
		return types.ZeroAttoFIL, 1, errors.NewFaultErrorf("expected types.AttoFIL to be returned, but got %T instead", ret)
	}

	return collateral, 0, nil
}

func (*Impl) AddFaults(ctx invocationContext, faults types.FaultSet) (uint8, error) {
	if err := ctx.Charge(actor.DefaultGasCost); err != nil {
		return internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	var state State
	_, err := actor.WithState(ctx, &state, func() (interface{}, error) {
		challengeBlockHeight := provingWindowStart(state)

		epoch := ctx.Runtime().CurrentEpoch()
		if epoch.LessThan(challengeBlockHeight) {
			// Up to the challenge time new faults can be added.
			state.CurrentFaultSet = state.CurrentFaultSet.Union(faults.SectorIds)
		} else {
			// After that they are only accounted for in the next proving period
			state.NextFaultSet = state.NextFaultSet.Union(faults.SectorIds)
		}

		return nil, nil
	})

	if err != nil {
		return errors.CodeError(err), err
	}

	return 0, nil
}

// SubmitPoSt is used to submit a coalesced PoST to the chain to convince the chain
// that you have been actually storing the files you claim to be.
func (a *Impl) SubmitPoSt(ctx invocationContext, poStProof types.PoStProof, faults types.FaultSet, done types.IntSet) (uint8, error) {
	if err := ctx.Charge(actor.DefaultGasCost); err != nil {
		return internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	chainHeight := ctx.Runtime().CurrentEpoch()
	sender := ctx.Message().Caller()
	var state State
	_, err := actor.WithState(ctx, &state, func() (interface{}, error) {
		// verify that the caller is authorized to perform update
		if sender != state.Worker {
			return nil, Errors[ErrCallerUnauthorized]
		}

		provingPeriodDuration := types.NewBlockHeight(ProvingPeriodDuration(state.SectorSize))
		nextProvingPeriodEnd := state.ProvingPeriodEnd.Add(provingPeriodDuration)

		// ensure PoSt is not too late entirely
		if chainHeight.GreaterEqual(nextProvingPeriodEnd) {
			// The PoSt has been submitted a full proving period after the proving period end.
			// The miner can expect to be slashed, and so for now the PoSt is rejected.
			// An alternative would be to apply the penalties here, duplicating the behaviour
			// of SlashStorageFault.
			return nil, errors.NewRevertErrorf("PoSt submitted later than grace period of %d rounds after proving period end",
				ProvingPeriodDuration(state.SectorSize))
		}

		feeRequired := latePoStFee(a.getPledgeCollateralRequirement(state, &chainHeight), state.ProvingPeriodEnd, &chainHeight, provingPeriodDuration)

		// The message value has been added to the actor's balance.
		// Ensure this value fully covers the fee which will be charged to this balance so that the resulting
		// balance (which forms pledge & storage collateral) is not less than it was before.
		messageValue := ctx.Message().ValueReceived()
		if messageValue.LessThan(feeRequired) {
			return nil, errors.NewRevertErrorf("PoSt message requires value of at least %s attofil to cover fees, got %s", feeRequired, messageValue)
		}

		// Since the message value was at least equal to this fee, this burn should not fail due to
		// insufficient balance.
		err := a.burnFunds(ctx, feeRequired)
		if err != nil {
			return nil, errors.RevertErrorWrapf(err, "Failed to burn fee %s", feeRequired)
		}

		// Refund any overpayment of fees to the owner.
		if messageValue.GreaterThan(feeRequired) {
			overpayment := messageValue.Sub(feeRequired)
			_, _, err := ctx.LegacySend(sender, types.SendMethodID, overpayment, []interface{}{})
			if err != nil {
				return nil, errors.NewRevertErrorf("Failed to refund overpayment of %s to %s", overpayment, sender)
			}
		}

		// As with commitSector messages, bootstrap miner actors don't verify
		// the submitPoSt messages that they are sent.
		//
		// This switching will be removed when issue #2270 is completed.
		if !a.Bootstrap {
			panic("this needs to use new sectorbuilder #3731")
		}

		// transition to the next proving period
		state.ProvingPeriodEnd = nextProvingPeriodEnd

		// Update miner power to the amount of data actually proved
		// during the last proving period.
		oldPower := state.Power
		newPower := types.NewBytesAmount(uint64(state.ProvingSet.Size() - faults.SectorIds.Size())).Mul(state.SectorSize)
		state.Power = newPower
		delta := newPower.Sub(oldPower)

		if !delta.IsZero() {
			_, ret, err := ctx.LegacySend(address.StorageMarketAddress, Storagemarket_UpdateStorage, types.ZeroAttoFIL, []interface{}{delta})
			if err != nil {
				return nil, err
			}
			if ret != 0 {
				return nil, Errors[ErrStoragemarketCallFailed]
			}
		}

		// Update SectorSet, DoneSet and ProvingSet
		if err = state.SectorCommitments.Drop(done.Values()); err != nil {
			return nil, err
		}

		if err = state.SectorCommitments.Drop(faults.SectorIds.Values()); err != nil {
			return nil, err
		}

		sectorIDsToProve, err := state.SectorCommitments.IDs()
		if err != nil {
			return nil, err
		}
		state.ProvingSet = types.NewIntSet(sectorIDsToProve...)
		state.NextDoneSet = done

		return nil, nil
	})
	if err != nil {
		return errors.CodeError(err), err
	}

	return 0, nil
}

// SlashStorageFault is called by an independent actor to remove power and
// take collateral from this miner when the miner has failed to submit a
// PoSt on time.
func (*Impl) SlashStorageFault(ctx invocationContext) (uint8, error) {
	if err := ctx.Charge(actor.DefaultGasCost); err != nil {
		return internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	chainHeight := ctx.Runtime().CurrentEpoch()
	var state State
	_, err := actor.WithState(ctx, &state, func() (interface{}, error) {
		// You can only be slashed once for missing your PoSt.
		if !state.SlashedAt.IsZero() {
			return nil, errors.NewCodedRevertError(ErrMinerAlreadySlashed, "miner already slashed")
		}

		// Only a miner who is expected to prove, can be slashed.
		if state.ProvingSet.Size() == 0 {
			return nil, errors.NewCodedRevertError(ErrMinerNotSlashable, "miner is inactive")
		}

		// Only if the miner is actually late, they can be slashed.
		deadline := state.ProvingPeriodEnd.Add(LatePoStGracePeriod(state.SectorSize))
		if chainHeight.LessEqual(deadline) {
			return nil, errors.NewCodedRevertError(ErrMinerNotSlashable, "miner not yet tardy")
		}

		// Strip the miner of their power.
		powerDelta := types.ZeroBytes.Sub(state.Power) // negate bytes amount
		_, ret, err := ctx.LegacySend(address.StorageMarketAddress, Storagemarket_UpdateStorage, types.ZeroAttoFIL, []interface{}{powerDelta})
		if err != nil {
			return nil, err
		}
		if ret != 0 {
			return nil, Errors[ErrStoragemarketCallFailed]
		}
		state.Power = types.NewBytesAmount(0)

		// record what has been slashed
		state.SlashedSet = state.ProvingSet

		// reserve collateral for arbitration
		// TODO: We currently do not know the correct amount of collateral to reserve here: https://github.com/filecoin-project/go-filecoin/issues/3050
		state.OwedStorageCollateral = types.ZeroAttoFIL

		// remove proving set from our sectors
		state.SectorCommitments.Drop(state.SlashedSet.Values())

		// clear proving set
		state.ProvingSet = types.NewIntSet()

		// save chain height, so we know when this miner was slashed
		state.SlashedAt = &chainHeight

		return nil, nil
	})

	if err != nil {
		return errors.CodeError(err), err
	}

	return 0, nil
}

// GetProvingWindow returns the proving period start and proving period end
func (*Impl) GetProvingWindow(ctx invocationContext) ([]types.Uint64, uint8, error) {
	var state State
	err := actor.ReadState(ctx, &state)
	if err != nil {
		return nil, errors.CodeError(err), err
	}

	return []types.Uint64{
		types.Uint64(provingWindowStart(state).AsBigInt().Uint64()),
		types.Uint64(state.ProvingPeriodEnd.AsBigInt().Uint64()),
	}, 0, nil
}

// CalculateLateFee calculates the late fee due for a PoSt arriving at `height` for the actor's current
// power and proving period.
func (a *Impl) CalculateLateFee(ctx invocationContext, height *types.BlockHeight) (types.AttoFIL, uint8, error) {
	var state State
	err := actor.ReadState(ctx, &state)
	if err != nil {
		return types.ZeroAttoFIL, errors.CodeError(err), err
	}

	epoch := ctx.Runtime().CurrentEpoch()
	collateral := a.getPledgeCollateralRequirement(state, &epoch)
	gracePeriod := types.NewBlockHeight(ProvingPeriodDuration(state.SectorSize))
	fee := latePoStFee(collateral, state.ProvingPeriodEnd, height, gracePeriod)
	return fee, 0, nil
}

//
// Un-exported methods
// These are methods, rather than free functions, even when they don't use the actor struct in
// expectation of this being important for future protocol upgrade mechanisms.
//

func (*Impl) burnFunds(ctx invocationContext, amount types.AttoFIL) error {
	_, _, err := ctx.LegacySend(address.BurntFundsAddress, types.SendMethodID, amount, []interface{}{})
	return err
}

func (*Impl) getPledgeCollateralRequirement(state State, height *types.BlockHeight) types.AttoFIL {
	// The pledge collateral is expected to be a function of power and block height, but is currently
	// a state variable.
	return state.ActiveCollateral
}

// getPoStChallengeSeed returns some chain randomness
func getPoStChallengeSeed(ctx invocationContext, state State, sampleAt *types.BlockHeight) (types.PoStChallengeSeed, error) {
	randomness := ctx.Runtime().Randomness(*sampleAt, 0)

	seed := types.PoStChallengeSeed{}
	copy(seed[:], randomness)

	return seed, nil
}

//
// Exported free functions.
//

// GetProofsMode returns the genesis block-configured proofs mode.
func GetProofsMode(ctx invocationContext) (types.ProofsMode, error) {
	var proofsMode types.ProofsMode
	msgResult, _, err := ctx.LegacySend(address.StorageMarketAddress, Storagemarket_GetProofsMode, types.ZeroAttoFIL, nil)
	if err != nil {
		return types.TestProofsMode, xerrors.Wrap(err, "'GetProofsMode' message failed")
	}
	if err := encoding.Decode(msgResult[0], &proofsMode); err != nil {
		return types.TestProofsMode, xerrors.Wrap(err, "could not unmarshall sector store type")
	}
	return proofsMode, nil
}

// CollateralForSector returns the collateral required to commit a sector of the
// given size.
func CollateralForSector(sectorSize *types.BytesAmount) types.AttoFIL {
	// TODO: Replace this function with the baseline pro-rata construction.
	// https://github.com/filecoin-project/go-filecoin/issues/2866
	return MinimumCollateralPerSector
}

// LatePoStGracePeriod is the number of blocks after a proving period ends
// after which a storage miner will be subject to storage fault slashing.
func LatePoStGracePeriod(sectorSize *types.BytesAmount) *types.BlockHeight {
	return types.NewBlockHeight(ProvingPeriodDuration(sectorSize))
}

// ProvingPeriodDuration returns the number of blocks in a proving period for a
// given sector size.
//
// TODO: Make this function return a non-bogus value.
// https://github.com/filecoin-project/specs/issues/321
func ProvingPeriodDuration(sectorSize *types.BytesAmount) uint64 {
	return LargestSectorSizeProvingPeriodBlocks
}

// LatePostFee calculates the fee from pledge collateral that a miner must pay for submitting a PoSt
// after the proving period has ended.
// The fee is calculated as a linear proportion of pledge collateral given by the lateness as a
// fraction of the maximum possible lateness (i.e. the generation attack grace period).
// If the submission is on-time, the fee is zero. If the submission is after the maximum allowed lateness
// the fee amounts to the entire pledge collateral.
func latePoStFee(pledgeCollateral types.AttoFIL, provingPeriodEnd *types.BlockHeight, chainHeight *types.BlockHeight, maxRoundsLate *types.BlockHeight) types.AttoFIL {
	lateState, roundsLate := lateState(provingPeriodEnd, chainHeight, maxRoundsLate)

	if lateState == PoStStateUnrecoverable {
		return pledgeCollateral
	} else if lateState == PoStStateAfterProvingPeriod {
		// fee = collateral * (roundsLate / maxRoundsLate)
		var fee big.Int
		fee.Mul(pledgeCollateral.AsBigInt(), roundsLate.AsBigInt())
		fee.Div(&fee, maxRoundsLate.AsBigInt()) // Integer division in AttoFIL, rounds towards zero.
		return types.NewAttoFIL(&fee)
	}

	return types.ZeroAttoFIL
}

//
// Internal functions
//

// calculates proving period start from the proving period end and the proving period duration
func provingWindowStart(state State) *types.BlockHeight {
	if state.ProvingPeriodEnd.IsZero() {
		return types.NewBlockHeight(0)
	}
	return state.ProvingPeriodEnd.Sub(types.NewBlockHeight(PoStChallengeWindowBlocks))
}

// lateState determines whether given a proving period and chain height, what is the
// degree of lateness and how many rounds they are late
func lateState(provingPeriodEnd *types.BlockHeight, chainHeight *types.BlockHeight, maxRoundsLate *types.BlockHeight) (int64, *types.BlockHeight) {
	roundsLate := chainHeight.Sub(provingPeriodEnd)
	if roundsLate.GreaterEqual(maxRoundsLate) {
		return PoStStateUnrecoverable, roundsLate
	} else if roundsLate.GreaterThan(types.NewBlockHeight(0)) {
		return PoStStateAfterProvingPeriod, roundsLate
	}
	return PoStStateWithinProvingPeriod, roundsLate
}
