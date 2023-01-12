package vmcontext

import (
	"context"
	"fmt"
	"time"

	"github.com/filecoin-project/go-address"
	acrypto "github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/venus/pkg/state"
	"github.com/filecoin-project/venus/venus-shared/actors/adt"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/account"
	blockstoreutil "github.com/filecoin-project/venus/venus-shared/blockstore"
	"github.com/filecoin-project/venus/venus-shared/types"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/pkg/fork"
	"github.com/filecoin-project/venus/pkg/state/tree"
	"github.com/filecoin-project/venus/pkg/vm/dispatch"
	"github.com/filecoin-project/venus/pkg/vm/gas"
)

type (
	ExecCallBack         func(cid.Cid, *types.Message, *Ret) error
	CircSupplyCalculator func(context.Context, abi.ChainEpoch, tree.Tree) (abi.TokenAmount, error)
	LookbackStateGetter  func(context.Context, abi.ChainEpoch) (*state.View, error)
	TipSetGetter         func(context.Context, abi.ChainEpoch) (types.TipSetKey, error)
)

type VmOption struct { //nolint
	CircSupplyCalculator CircSupplyCalculator
	LookbackStateGetter  LookbackStateGetter
	NetworkVersion       network.Version
	Rnd                  HeadChainRandomness
	BaseFee              abi.TokenAmount
	Fork                 fork.IFork
	ActorCodeLoader      *dispatch.CodeLoader
	Epoch                abi.ChainEpoch
	Timestamp            uint64
	GasPriceSchedule     *gas.PricesSchedule
	PRoot                cid.Cid
	Bsstore              blockstoreutil.Blockstore
	SysCallsImpl         SyscallsImpl
	TipSetGetter         TipSetGetter
	Tracing              bool

	ActorDebugging bool
}

type ILookBack interface {
	StateView(ctx context.Context, ts *types.TipSet) (*state.View, error)
	GetLookbackTipSetForRound(ctx context.Context, ts *types.TipSet, round abi.ChainEpoch, version network.Version) (*types.TipSet, cid.Cid, error)
}

func LookbackStateGetterForTipset(ctx context.Context, backer ILookBack, fork fork.IFork, ts *types.TipSet) LookbackStateGetter {
	return func(ctx context.Context, round abi.ChainEpoch) (*state.View, error) {
		ver := fork.GetNetworkVersion(ctx, round)
		ts, _, err := backer.GetLookbackTipSetForRound(ctx, ts, round, ver)
		if err != nil {
			return nil, err
		}
		return backer.StateView(ctx, ts)
	}
}

func TipSetGetterForTipset(tsGet func(context.Context, *types.TipSet, abi.ChainEpoch, bool) (*types.TipSet, error),
	ts *types.TipSet,
) TipSetGetter {
	return func(ctx context.Context, round abi.ChainEpoch) (types.TipSetKey, error) {
		ts, err := tsGet(ctx, ts, round, true)
		if err != nil {
			return types.EmptyTSK, err
		}
		return ts.Key(), nil
	}
}

// ChainRandomness define randomness method in filecoin
type HeadChainRandomness interface {
	ChainGetRandomnessFromBeacon(ctx context.Context, personalization acrypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error)
	ChainGetRandomnessFromTickets(ctx context.Context, personalization acrypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error)
}

type Ret struct {
	GasTracker *gas.GasTracker
	OutPuts    gas.GasOutputs
	Receipt    types.MessageReceipt
	ActorErr   error
	Duration   time.Duration
}

// Failure returns with a non-zero exit code.
func Failure(exitCode exitcode.ExitCode, gasAmount int64) types.MessageReceipt {
	return types.MessageReceipt{
		ExitCode: exitCode,
		Return:   []byte{},
		GasUsed:  gasAmount,
	}
}

type Interface interface {
	ApplyMessage(ctx context.Context, cmsg types.ChainMsg) (*Ret, error)
	ApplyImplicitMessage(ctx context.Context, msg types.ChainMsg) (*Ret, error)
	Flush(ctx context.Context) (cid.Cid, error)
}

func ResolveToKeyAddr(ctx context.Context, state tree.Tree, addr address.Address, cst cbor.IpldStore) (address.Address, error) {
	if addr.Protocol() == address.BLS || addr.Protocol() == address.SECP256K1 || addr.Protocol() == address.Delegated {
		return addr, nil
	}

	act, found, err := state.GetActor(ctx, addr)
	if err != nil {
		return address.Undef, errors.Wrapf(err, "failed to find actor: %s", addr)
	}
	if !found {
		return address.Undef, fmt.Errorf("actor not found %s", addr)
	}

	if state.Version() >= tree.StateTreeVersion5 {
		if act.Address != nil {
			// If there _is_ an f4 address, return it as "key" address
			return *act.Address, nil
		}
	}

	aast, err := account.Load(adt.WrapStore(ctx, cst), act)
	if err != nil {
		return address.Undef, fmt.Errorf("failed to get account actor state for %s: %w", addr, err)
	}

	return aast.PubkeyAddress()
}
