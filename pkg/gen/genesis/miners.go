package genesis

import (
	"bytes"
	"context"
	"fmt"
	"github.com/filecoin-project/venus/pkg/vm/vmcontext"
	"math/rand"

	market0 "github.com/filecoin-project/specs-actors/actors/builtin/market"

	"github.com/filecoin-project/venus/pkg/specactors/builtin/power"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/reward"

	"github.com/filecoin-project/venus/pkg/specactors/builtin/market"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/miner"

	"github.com/filecoin-project/go-state-types/network"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	cbg "github.com/whyrusleeping/cbor-gen"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/crypto"
	builtin0 "github.com/filecoin-project/specs-actors/actors/builtin"
	miner0 "github.com/filecoin-project/specs-actors/actors/builtin/miner"
	power0 "github.com/filecoin-project/specs-actors/actors/builtin/power"
	reward0 "github.com/filecoin-project/specs-actors/actors/builtin/reward"

	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/fork"
	"github.com/filecoin-project/venus/pkg/slashing"
	"github.com/filecoin-project/venus/pkg/util/ffiwrapper"
	"github.com/filecoin-project/venus/pkg/vm"
	"github.com/filecoin-project/venus/pkg/vm/gas"
	"github.com/filecoin-project/venus/pkg/vm/state"
	"github.com/filecoin-project/venus/pkg/vmsupport"
)

func MinerAddress(genesisIndex uint64) address.Address {
	maddr, err := address.NewIDAddress(MinerStart + genesisIndex)
	if err != nil {
		panic(err)
	}

	return maddr
}

type fakedSigSyscalls struct {
	vmcontext.SyscallsImpl
}

func (fss *fakedSigSyscalls) VerifySignature(ctx context.Context, view vmcontext.SyscallsStateView, signature crypto.Signature, signer address.Address, plaintext []byte) error {
	return nil
}

func mkFakedSigSyscalls(sys vmcontext.SyscallsImpl) vmcontext.SyscallsImpl {
	return &fakedSigSyscalls{
		sys,
	}
}

func SetupStorageMiners(ctx context.Context, cs *chain.Store, sroot cid.Cid, miners []Miner, para *config.ForkUpgradeConfig) (cid.Cid, error) {
	csc := func(context.Context, abi.ChainEpoch, state.Tree) (abi.TokenAmount, error) {
		return big.Zero(), nil
	}

	genesisNetworkVersion := func(context.Context, abi.ChainEpoch) network.Version { // TODO: Get from build/
		// returns the version _before_ the first upgrade.
		if para.UpgradeBreezeHeight >= 0 {
			return network.Version0
		}
		if para.UpgradeSmokeHeight >= 0 {
			return network.Version1
		}
		if para.UpgradeIgnitionHeight >= 0 {
			return network.Version2
		}
		if para.UpgradeActorsV2Height >= 0 {
			return network.Version3
		}
		if para.UpgradeLiftoffHeight >= 0 {
			return network.Version3
		}
		return constants.ActorUpgradeNetworkVersion - 1 // genesis requires actors v0.
	}

	faultChecker := slashing.NewFaultChecker(cs, fork.NewMockFork())
	syscalls := vmsupport.NewSyscalls(faultChecker, ffiwrapper.ProofVerifier)
	gasPirceSchedule := gas.NewPricesSchedule(para)
	vmopt := vm.VmOption{
		CircSupplyCalculator: csc,
		NtwkVersionGetter:    genesisNetworkVersion,
		Rnd:                  &fakeRand{},
		BaseFee:              big.NewInt(0),
		Epoch:                0,
		PRoot:                sroot,
		Bsstore:              cs.Blockstore(),
		SysCallsImpl:         mkFakedSigSyscalls(syscalls),
		GasPriceSchedule:     gasPirceSchedule,
	}

	vmi, err := vm.NewVM(vmopt)
	if err != nil {
		return cid.Undef, xerrors.Errorf("failed to create NewVM: %w", err)
	}

	if len(miners) == 0 {
		return cid.Undef, xerrors.New("no genesis miners")
	}

	minerInfos := make([]struct {
		maddr address.Address

		presealExp abi.ChainEpoch

		dealIDs []abi.DealID
	}, len(miners))

	for i, m := range miners {
		// Create miner through power actor
		i := i
		m := m

		spt, err := miner.SealProofTypeFromSectorSize(m.SectorSize, genesisNetworkVersion(ctx, 0))
		if err != nil {
			return cid.Undef, err
		}

		{
			constructorParams := &power0.CreateMinerParams{
				Owner:         m.Worker,
				Worker:        m.Worker,
				Peer:          []byte(m.PeerId),
				SealProofType: spt,
			}

			params := mustEnc(constructorParams)
			rval, err := doExecValue(ctx, vmi, power.Address, m.Owner, m.PowerBalance, builtin0.MethodsPower.CreateMiner, params)
			if err != nil {
				return cid.Undef, xerrors.Errorf("failed to create genesis miner: %w", err)
			}

			var ma power0.CreateMinerReturn
			if err := ma.UnmarshalCBOR(bytes.NewReader(rval)); err != nil {
				return cid.Undef, xerrors.Errorf("unmarshaling CreateMinerReturn: %w", err)
			}

			expma := MinerAddress(uint64(i))
			if ma.IDAddress != expma {
				return cid.Undef, xerrors.Errorf("miner assigned wrong address: %s != %s", ma.IDAddress, expma)
			}
			minerInfos[i].maddr = ma.IDAddress

			// TODO: ActorUpgrade
			err = vmi.MutateState(ctx, minerInfos[i].maddr, func(cst cbor.IpldStore, st *miner0.State) error {
				maxPeriods := miner0.MaxSectorExpirationExtension / miner0.WPoStProvingPeriod
				minerInfos[i].presealExp = (maxPeriods-1)*miner0.WPoStProvingPeriod + st.ProvingPeriodStart - 1

				return nil
			})
			if err != nil {
				return cid.Undef, xerrors.Errorf("mutating state: %w", err)
			}
		}

		// Add market funds

		if m.MarketBalance.GreaterThan(big.Zero()) {
			params := mustEnc(&minerInfos[i].maddr)
			_, err := doExecValue(ctx, vmi, market.Address, m.Worker, m.MarketBalance, builtin0.MethodsMarket.AddBalance, params)
			if err != nil {
				return cid.Undef, xerrors.Errorf("failed to create genesis miner (add balance): %w", err)
			}
		}

		// Publish preseal deals

		{
			publish := func(params *market.PublishStorageDealsParams) error {
				fmt.Printf("publishing %d storage deals on miner %s with worker %s\n", len(params.Deals), params.Deals[0].Proposal.Provider, m.Worker)

				ret, err := doExecValue(ctx, vmi, market.Address, m.Worker, big.Zero(), builtin0.MethodsMarket.PublishStorageDeals, mustEnc(params))
				if err != nil {
					return xerrors.Errorf("failed to create genesis miner (publish deals): %w", err)
				}
				var ids market.PublishStorageDealsReturn
				if err := ids.UnmarshalCBOR(bytes.NewReader(ret)); err != nil {
					return xerrors.Errorf("unmarsahling publishStorageDeals result: %w", err)
				}

				minerInfos[i].dealIDs = append(minerInfos[i].dealIDs, ids.IDs...)
				return nil
			}

			params := &market.PublishStorageDealsParams{}
			for _, preseal := range m.Sectors {
				preseal.Deal.VerifiedDeal = true
				preseal.Deal.EndEpoch = minerInfos[i].presealExp
				params.Deals = append(params.Deals, market.ClientDealProposal{
					Proposal:        preseal.Deal,
					ClientSignature: crypto.Signature{Type: crypto.SigTypeBLS}, // TODO: do we want to sign these? Or do we want to fake signatures for genesis setup?
				})

				if len(params.Deals) == cbg.MaxLength {
					if err := publish(params); err != nil {
						return cid.Undef, err
					}

					params = &market.PublishStorageDealsParams{}
				}
			}

			if len(params.Deals) > 0 {
				if err := publish(params); err != nil {
					return cid.Undef, err
				}
			}
		}
	}

	// adjust total network power for equal pledge per sector
	rawPow, qaPow := big.NewInt(0), big.NewInt(0)
	{
		for i, m := range miners {
			for pi := range m.Sectors {
				rawPow = big.Add(rawPow, big.NewInt(int64(m.SectorSize)))

				dweight, err := dealWeight(ctx, vmi, minerInfos[i].maddr, []abi.DealID{minerInfos[i].dealIDs[pi]}, 0, minerInfos[i].presealExp)
				if err != nil {
					return cid.Undef, xerrors.Errorf("getting deal weight: %w", err)
				}

				sectorWeight := miner0.QAPowerForWeight(m.SectorSize, minerInfos[i].presealExp, dweight.DealWeight, dweight.VerifiedDealWeight)

				qaPow = big.Add(qaPow, sectorWeight)
			}
		}

		err = vmi.MutateState(ctx, power.Address, func(cst cbor.IpldStore, st *power0.State) error {
			st.TotalQualityAdjPower = qaPow
			st.TotalRawBytePower = rawPow

			st.ThisEpochQualityAdjPower = qaPow
			st.ThisEpochRawBytePower = rawPow
			return nil
		})
		if err != nil {
			return cid.Undef, xerrors.Errorf("mutating state: %w", err)
		}

		err = vmi.MutateState(ctx, reward.Address, func(sct cbor.IpldStore, st *reward0.State) error {
			*st = *reward0.ConstructState(qaPow)
			return nil
		})
		if err != nil {
			return cid.Undef, xerrors.Errorf("mutating state: %w", err)
		}
	}

	for i, m := range miners {
		// Commit sectors
		{
			for pi, preseal := range m.Sectors {
				params := &miner.SectorPreCommitInfo{
					SealProof:     preseal.ProofType,
					SectorNumber:  preseal.SectorID,
					SealedCID:     preseal.CommR,
					SealRandEpoch: -1,
					DealIDs:       []abi.DealID{minerInfos[i].dealIDs[pi]},
					Expiration:    minerInfos[i].presealExp, // TODO: Allow setting externally!
				}

				dweight, err := dealWeight(ctx, vmi, minerInfos[i].maddr, params.DealIDs, 0, minerInfos[i].presealExp)
				if err != nil {
					return cid.Undef, xerrors.Errorf("getting deal weight: %w", err)
				}

				sectorWeight := miner0.QAPowerForWeight(m.SectorSize, minerInfos[i].presealExp, dweight.DealWeight, dweight.VerifiedDealWeight)

				// we've added fake power for this sector above, remove it now
				err = vmi.MutateState(ctx, power.Address, func(cst cbor.IpldStore, st *power0.State) error {
					st.TotalQualityAdjPower = big.Sub(st.TotalQualityAdjPower, sectorWeight) //nolint:scopelint
					st.TotalRawBytePower = big.Sub(st.TotalRawBytePower, big.NewInt(int64(m.SectorSize)))
					return nil
				})
				if err != nil {
					return cid.Undef, xerrors.Errorf("removing fake power: %w", err)
				}

				epochReward, err := currentEpochBlockReward(ctx, vmi, minerInfos[i].maddr)
				if err != nil {
					return cid.Undef, xerrors.Errorf("getting current epoch reward: %w", err)
				}

				tpow, err := currentTotalPower(ctx, vmi, minerInfos[i].maddr)
				if err != nil {
					return cid.Undef, xerrors.Errorf("getting current total power: %w", err)
				}

				pcd := miner0.PreCommitDepositForPower(epochReward.ThisEpochRewardSmoothed, tpow.QualityAdjPowerSmoothed, sectorWeight)

				pledge := miner0.InitialPledgeForPower(
					sectorWeight,
					epochReward.ThisEpochBaselinePower,
					tpow.PledgeCollateral,
					epochReward.ThisEpochRewardSmoothed,
					tpow.QualityAdjPowerSmoothed,
					circSupply(ctx, vmi, minerInfos[i].maddr),
				)

				pledge = big.Add(pcd, pledge)

				fmt.Println(pledge)
				_, err = doExecValue(ctx, vmi, minerInfos[i].maddr, m.Worker, pledge, builtin0.MethodsMiner.PreCommitSector, mustEnc(params))
				if err != nil {
					return cid.Undef, xerrors.Errorf("failed to confirm presealed sectors: %w", err)
				}

				// Commit one-by-one, otherwise pledge math tends to explode
				confirmParams := &builtin0.ConfirmSectorProofsParams{
					Sectors: []abi.SectorNumber{preseal.SectorID},
				}

				_, err = doExecValue(ctx, vmi, minerInfos[i].maddr, power.Address, big.Zero(), builtin0.MethodsMiner.ConfirmSectorProofsValid, mustEnc(confirmParams))
				if err != nil {
					return cid.Undef, xerrors.Errorf("failed to confirm presealed sectors: %w", err)
				}
			}
		}
	}

	// Sanity-check total network power
	err = vmi.MutateState(ctx, power.Address, func(cst cbor.IpldStore, st *power0.State) error {
		if !st.TotalRawBytePower.Equals(rawPow) {
			return xerrors.Errorf("st.TotalRawBytePower doesn't match previously calculated rawPow")
		}

		if !st.TotalQualityAdjPower.Equals(qaPow) {
			return xerrors.Errorf("st.TotalQualityAdjPower doesn't match previously calculated qaPow")
		}

		return nil
	})
	if err != nil {
		return cid.Undef, xerrors.Errorf("mutating state: %w", err)
	}

	// TODO: Should we re-ConstructState for the reward actor using rawPow as currRealizedPower here?

	c, err := vmi.Flush()
	if err != nil {
		return cid.Undef, xerrors.Errorf("flushing vm: %w", err)
	}
	return c, nil
}

// TODO: copied from actors test harness, deduplicate or remove from here
type fakeRand struct{}

func (fr *fakeRand) Randomness(ctx context.Context, tag crypto.DomainSeparationTag, epoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	out := make([]byte, 32)
	_, _ = rand.New(rand.NewSource(int64(epoch * 1000))).Read(out) //nolint
	return out, nil
}

func (fr *fakeRand) GetRandomnessFromBeacon(ctx context.Context, personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	out := make([]byte, 32)
	_, _ = rand.New(rand.NewSource(int64(randEpoch))).Read(out) //nolint
	return out, nil
}

func currentTotalPower(ctx context.Context, vmi vm.Interpreter, maddr address.Address) (*power0.CurrentTotalPowerReturn, error) {
	pwret, err := doExecValue(ctx, vmi, power.Address, maddr, big.Zero(), builtin0.MethodsPower.CurrentTotalPower, nil)
	if err != nil {
		return nil, err
	}
	var pwr power0.CurrentTotalPowerReturn
	if err := pwr.UnmarshalCBOR(bytes.NewReader(pwret)); err != nil {
		return nil, err
	}

	return &pwr, nil
}

func dealWeight(ctx context.Context, vmi vm.Interpreter, maddr address.Address, dealIDs []abi.DealID, sectorStart, sectorExpiry abi.ChainEpoch) (market0.VerifyDealsForActivationReturn, error) {
	params := &market.VerifyDealsForActivationParams{
		DealIDs:      dealIDs,
		SectorStart:  sectorStart,
		SectorExpiry: sectorExpiry,
	}

	var dealWeights market0.VerifyDealsForActivationReturn
	ret, err := doExecValue(ctx, vmi,
		market.Address,
		maddr,
		abi.NewTokenAmount(0),
		builtin0.MethodsMarket.VerifyDealsForActivation,
		mustEnc(params),
	)
	if err != nil {
		return market0.VerifyDealsForActivationReturn{}, err
	}
	if err := dealWeights.UnmarshalCBOR(bytes.NewReader(ret)); err != nil {
		return market0.VerifyDealsForActivationReturn{}, err
	}

	return dealWeights, nil
}

func currentEpochBlockReward(ctx context.Context, vmi vm.Interpreter, maddr address.Address) (*reward0.ThisEpochRewardReturn, error) {
	rwret, err := doExecValue(ctx, vmi, reward.Address, maddr, big.Zero(), builtin0.MethodsReward.ThisEpochReward, nil)
	if err != nil {
		return nil, err
	}

	var epochReward reward0.ThisEpochRewardReturn
	if err := epochReward.UnmarshalCBOR(bytes.NewReader(rwret)); err != nil {
		return nil, err
	}

	return &epochReward, nil
}

// todo what is actually called here is vmi.vmOption.CircSupplyCalculator(context.TODO(), height, st) -> L55, So there is no structure UnsafeVM ???
func circSupply(ctx context.Context, vmi vm.Interpreter, maddr address.Address) abi.TokenAmount {

	return big.Zero()
}
