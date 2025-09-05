package genesis

import (
	"context"
	"errors"
	"fmt"

	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"

	"github.com/filecoin-project/venus/fixtures/networks"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/state/tree"
	"github.com/filecoin-project/venus/pkg/vm"
	"github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/actors/adt"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/power"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/reward"
	"github.com/filecoin-project/venus/venus-shared/types"
)

func mustEnc(i cbg.CBORMarshaler) []byte {
	enc, err := actors.SerializeParams(i)
	if err != nil {
		panic(err) // ok
	}
	return enc
}

func doExecValue(ctx context.Context, vmi vm.Interface, to, from address.Address, value types.BigInt, method abi.MethodNum, params []byte) ([]byte, error) {
	ret, err := vmi.ApplyImplicitMessage(context.TODO(), &types.Message{
		To:       to,
		From:     from,
		Method:   method,
		Params:   params,
		GasLimit: 1_000_000_000_000_000,
		Value:    value,
		Nonce:    0,
	})
	if err != nil {
		return nil, fmt.Errorf("doExec apply message failed: %w", err)
	}

	if ret.Receipt.ExitCode != 0 {
		return nil, fmt.Errorf("failed to call method: %s", ret.Receipt.String())
	}

	return ret.Receipt.Return, nil
}

// CalculateMinerCreationDepositForGenesis calculates the deposit required for creating a new miner
// during genesis block creation. This is used when API is not available yet.
func CalculateMinerCreationDepositForGenesis(ctx context.Context, vm vm.Interface, store adt.Store) (abi.TokenAmount, error) {
	nh, err := vm.Flush(ctx)
	if err != nil {
		return big.Zero(), fmt.Errorf("flushing vm: %w", err)
	}

	nst, err := tree.LoadState(ctx, store, nh)
	if err != nil {
		return big.Zero(), fmt.Errorf("loading new state tree: %w", err)
	}
	// Get reward actor state
	rewardActor, found, err := nst.GetActor(ctx, reward.Address)
	if err != nil {
		return big.Zero(), fmt.Errorf("loading reward actor: %w", err)
	}
	if !found {
		return big.Zero(), errors.New("not found reward actor")
	}

	rewardState, err := reward.Load(store, rewardActor)
	if err != nil {
		return big.Zero(), fmt.Errorf("loading reward actor state: %w", err)
	}

	// Get power actor state
	powerActor, found, err := nst.GetActor(ctx, power.Address)
	if err != nil {
		return big.Zero(), fmt.Errorf("loading power actor: %w", err)
	}
	if !found {
		return big.Zero(), errors.New("not found power actor")
	}

	powerState, err := power.Load(store, powerActor)
	if err != nil {
		return big.Zero(), fmt.Errorf("loading power actor state: %w", err)
	}

	networkQAPowerSmoothed, err := powerState.TotalPowerSmoothed()
	if err != nil {
		return big.Zero(), fmt.Errorf("getting network QA power smoothed: %w", err)
	}

	pledgeCollateral, err := powerState.TotalLocked()
	if err != nil {
		return big.Zero(), fmt.Errorf("getting pledge collateral: %w", err)
	}

	// For genesis, use the initial reserved amount as default circulating supply
	circSupply := types.BigInt{Int: constants.InitialFilReserved}

	// Calculate the deposit power as 1/10 of consensus miner minimum power
	createMinerDepositPower := big.Div(big.NewInt(int64(networks.Net2k().Network.ConsensusMinerMinPower)), big.NewInt(10))

	// For genesis, assume we're at the start of the ramp
	rampDurationEpochs := powerState.RampDurationEpochs()
	epochsSinceRampStart := int64(0) // Genesis is at the beginning

	// Use the reward state's InitialPledgeForPower function to calculate the deposit
	deposit, err := rewardState.InitialPledgeForPower(
		createMinerDepositPower,
		pledgeCollateral,
		&networkQAPowerSmoothed,
		circSupply,
		epochsSinceRampStart,
		rampDurationEpochs,
	)
	if err != nil {
		return big.Zero(), fmt.Errorf("calculating initial pledge for power: %w", err)
	}

	return deposit, nil
}
