package genesis

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"

	power4 "github.com/filecoin-project/specs-actors/v4/actors/builtin/power"
	builtin6 "github.com/filecoin-project/specs-actors/v6/actors/builtin"

	reward4 "github.com/filecoin-project/specs-actors/v4/actors/builtin/reward"

	market4 "github.com/filecoin-project/specs-actors/v4/actors/builtin/market"

	"github.com/filecoin-project/venus/pkg/types/specactors"

	"github.com/filecoin-project/venus/pkg/types/specactors/builtin"

	"github.com/filecoin-project/venus/pkg/types/specactors/policy"

	"github.com/filecoin-project/venus/pkg/types/specactors/adt"

	"github.com/filecoin-project/go-state-types/network"

	market0 "github.com/filecoin-project/specs-actors/actors/builtin/market"

	"github.com/filecoin-project/venus/pkg/types/specactors/builtin/power"
	"github.com/filecoin-project/venus/pkg/types/specactors/builtin/reward"

	"github.com/filecoin-project/venus/pkg/types/specactors/builtin/market"
	"github.com/filecoin-project/venus/pkg/types/specactors/builtin/miner"

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
	"github.com/filecoin-project/venus/pkg/consensusfault"
	"github.com/filecoin-project/venus/pkg/fork"
	"github.com/filecoin-project/venus/pkg/state/tree"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/util/ffiwrapper/impl"
	"github.com/filecoin-project/venus/pkg/vm"
	"github.com/filecoin-project/venus/pkg/vm/gas"
	"github.com/filecoin-project/venus/pkg/vm/vmcontext"
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

// Note: Much of this is brittle, if the methodNum / param / return changes, it will break things
func SetupStorageMiners(ctx context.Context, cs *chain.Store, sroot cid.Cid, miners []Miner, nv network.Version, para *config.ForkUpgradeConfig) (cid.Cid, error) {
	cst := cbor.NewCborStore(cs.Blockstore())
	av, err := specactors.VersionForNetwork(nv)
	if err != nil {
		return cid.Undef, xerrors.Errorf("get actor version: %w", err)
	}

	csc := func(context.Context, abi.ChainEpoch, tree.Tree) (abi.TokenAmount, error) {
		return big.Zero(), nil
	}

	genesisNetworkVersion := func(context.Context, abi.ChainEpoch) network.Version {
		return nv
	}

	faultChecker := consensusfault.NewFaultChecker(cs, fork.NewMockFork())
	syscalls := vmsupport.NewSyscalls(faultChecker, impl.ProofVerifier)
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

	maxPeriods := policy.GetMaxSectorExpirationExtension() / miner.WPoStProvingPeriod
	for i, m := range miners {
		// Create miner through power actor
		i := i
		m := m

		spt, err := miner.SealProofTypeFromSectorSize(m.SectorSize, nv)
		if err != nil {
			return cid.Undef, err
		}

		{
			constructorParams := &power0.CreateMinerParams{
				Owner:         m.Worker,
				Worker:        m.Worker,
				Peer:          []byte(m.PeerID),
				SealProofType: spt,
			}

			params := mustEnc(constructorParams)
			rval, err := doExecValue(ctx, vmi, power.Address, m.Owner, m.PowerBalance, power.Methods.CreateMiner, params)
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

			_, err = vmi.Flush()
			if err != nil {
				return cid.Undef, xerrors.Errorf("flushing vm: %w", err)
			}

			mact, find, err := vmi.StateTree().GetActor(ctx, minerInfos[i].maddr)
			if err != nil {
				return cid.Undef, xerrors.Errorf("getting newly created miner actor: %w", err)
			}

			if !find {
				return cid.Undef, xerrors.New("actor not found")
			}

			mst, err := miner.Load(adt.WrapStore(ctx, cst), mact)
			if err != nil {
				return cid.Undef, xerrors.Errorf("getting newly created miner state: %w", err)
			}

			pps, err := mst.GetProvingPeriodStart()
			if err != nil {
				return cid.Undef, xerrors.Errorf("getting newly created miner proving period start: %w", err)
			}

			minerInfos[i].presealExp = (maxPeriods-1)*miner0.WPoStProvingPeriod + pps - 1
		}

		// Add market funds

		if m.MarketBalance.GreaterThan(big.Zero()) {
			params := mustEnc(&minerInfos[i].maddr)
			_, err := doExecValue(ctx, vmi, market.Address, m.Worker, m.MarketBalance, market.Methods.AddBalance, params)
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
				retval, err := market.DecodePublishStorageDealsReturn(ret, nv)
				if err != nil {
					return xerrors.Errorf("failed to create genesis miner (decoding published deals): %w", err)
				}
				ids, err := retval.DealIDs()
				if err != nil {
					return xerrors.Errorf("failed to create genesis miner (getting published dealIDs): %w", err)
				}

				if len(ids) != len(params.Deals) {
					return xerrors.Errorf("failed to create genesis miner (at least one deal was invalid on publication")
				}

				minerInfos[i].dealIDs = append(minerInfos[i].dealIDs, ids...)
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
				rawPow = types.BigAdd(rawPow, types.NewInt(uint64(m.SectorSize)))

				dweight, vdweight, err := dealWeight(ctx, vmi, minerInfos[i].maddr, []abi.DealID{minerInfos[i].dealIDs[pi]}, 0, minerInfos[i].presealExp, av)
				if err != nil {
					return cid.Undef, xerrors.Errorf("getting deal weight: %w", err)
				}

				sectorWeight := builtin.QAPowerForWeight(m.SectorSize, minerInfos[i].presealExp, dweight, vdweight)

				qaPow = types.BigAdd(qaPow, sectorWeight)
			}
		}

		_, err = vmi.Flush()
		if err != nil {
			return cid.Undef, xerrors.Errorf("flushing vm: %w", err)
		}

		pact, find, err := vmi.StateTree().GetActor(ctx, power.Address)
		if err != nil {
			return cid.Undef, xerrors.Errorf("getting power actor: %w", err)
		}

		if !find {
			return cid.Undef, xerrors.New("power actor not exist")
		}

		pst, err := power.Load(adt.WrapStore(ctx, cst), pact)
		if err != nil {
			return cid.Undef, xerrors.Errorf("getting power state: %w", err)
		}

		if err = pst.SetTotalQualityAdjPower(qaPow); err != nil {
			return cid.Undef, xerrors.Errorf("setting TotalQualityAdjPower in power state: %w", err)
		}

		if err = pst.SetTotalRawBytePower(rawPow); err != nil {
			return cid.Undef, xerrors.Errorf("setting TotalRawBytePower in power state: %w", err)
		}

		if err = pst.SetThisEpochQualityAdjPower(qaPow); err != nil {
			return cid.Undef, xerrors.Errorf("setting ThisEpochQualityAdjPower in power state: %w", err)
		}

		if err = pst.SetThisEpochRawBytePower(rawPow); err != nil {
			return cid.Undef, xerrors.Errorf("setting ThisEpochRawBytePower in power state: %w", err)
		}

		pcid, err := cst.Put(ctx, pst.GetState())
		if err != nil {
			return cid.Undef, xerrors.Errorf("putting power state: %w", err)
		}

		pact.Head = pcid

		if err = vmi.StateTree().SetActor(ctx, power.Address, pact); err != nil {
			return cid.Undef, xerrors.Errorf("setting power state: %w", err)
		}

		ver, err := specactors.VersionForNetwork(nv)
		if err != nil {
			return cid.Undef, xerrors.Errorf("get actor version: %w", err)
		}

		rewact, err := SetupRewardActor(ctx, cs.Blockstore(), big.Zero(), ver)
		if err != nil {
			return cid.Undef, xerrors.Errorf("setup reward actor: %w", err)
		}

		if err = vmi.StateTree().SetActor(ctx, reward.Address, rewact); err != nil {
			return cid.Undef, xerrors.Errorf("set reward actor: %w", err)
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

				dweight, vdweight, err := dealWeight(ctx, vmi, minerInfos[i].maddr, params.DealIDs, 0, minerInfos[i].presealExp, av)
				if err != nil {
					return cid.Undef, xerrors.Errorf("getting deal weight: %w", err)
				}

				sectorWeight := builtin.QAPowerForWeight(m.SectorSize, minerInfos[i].presealExp, dweight, vdweight)

				// we've added fake power for this sector above, remove it now

				_, err = vmi.Flush()
				if err != nil {
					return cid.Undef, xerrors.Errorf("flushing vm: %w", err)
				}

				pact, find, err := vmi.StateTree().GetActor(ctx, power.Address)
				if err != nil {
					return cid.Undef, xerrors.Errorf("getting power actor: %w", err)
				}

				if !find {
					return cid.Undef, xerrors.New("power actor not exist")
				}

				pst, err := power.Load(adt.WrapStore(ctx, cst), pact)
				if err != nil {
					return cid.Undef, xerrors.Errorf("getting power state: %w", err)
				}

				pc, err := pst.TotalPower()
				if err != nil {
					return cid.Undef, xerrors.Errorf("getting total power: %w", err)
				}

				if err = pst.SetTotalRawBytePower(types.BigSub(pc.RawBytePower, types.NewInt(uint64(m.SectorSize)))); err != nil {
					return cid.Undef, xerrors.Errorf("setting TotalRawBytePower in power state: %w", err)
				}

				if err = pst.SetTotalQualityAdjPower(types.BigSub(pc.QualityAdjPower, sectorWeight)); err != nil {
					return cid.Undef, xerrors.Errorf("setting TotalQualityAdjPower in power state: %w", err)
				}

				pcid, err := cst.Put(ctx, pst.GetState())
				if err != nil {
					return cid.Undef, xerrors.Errorf("putting power state: %w", err)
				}

				pact.Head = pcid

				if err = vmi.StateTree().SetActor(ctx, power.Address, pact); err != nil {
					return cid.Undef, xerrors.Errorf("setting power state: %w", err)
				}

				baselinePower, rewardSmoothed, err := currentEpochBlockReward(ctx, vmi, minerInfos[i].maddr, av)
				if err != nil {
					return cid.Undef, xerrors.Errorf("getting current epoch reward: %w", err)
				}

				tpow, err := currentTotalPower(ctx, vmi, minerInfos[i].maddr)
				if err != nil {
					return cid.Undef, xerrors.Errorf("getting current total power: %w", err)
				}

				pcd := miner0.PreCommitDepositForPower(&rewardSmoothed, tpow.QualityAdjPowerSmoothed, sectorWeight)

				pledge := miner0.InitialPledgeForPower(
					sectorWeight,
					baselinePower,
					tpow.PledgeCollateral,
					&rewardSmoothed,
					tpow.QualityAdjPowerSmoothed,
					circSupply(ctx, vmi, minerInfos[i].maddr),
				)

				pledge = big.Add(pcd, pledge)

				fmt.Println(types.FIL(pledge))
				_, err = doExecValue(ctx, vmi, minerInfos[i].maddr, m.Worker, pledge, miner.Methods.PreCommitSector, mustEnc(params))
				if err != nil {
					return cid.Undef, xerrors.Errorf("failed to confirm presealed sectors: %w", err)
				}

				// Commit one-by-one, otherwise pledge math tends to explode
				var paramBytes []byte

				if av >= specactors.Version6 {
					// TODO: fixup
					confirmParams := &builtin6.ConfirmSectorProofsParams{
						Sectors: []abi.SectorNumber{preseal.SectorID},
					}

					paramBytes = mustEnc(confirmParams)
				} else {
					confirmParams := &builtin0.ConfirmSectorProofsParams{
						Sectors: []abi.SectorNumber{preseal.SectorID},
					}

					paramBytes = mustEnc(confirmParams)
				}

				_, err = doExecValue(ctx, vmi, minerInfos[i].maddr, power.Address, big.Zero(), miner.Methods.ConfirmSectorProofsValid, paramBytes)
				if err != nil {
					return cid.Undef, xerrors.Errorf("failed to confirm presealed sectors: %w", err)
				}

				if av > specactors.Version2 {
					// post v2, we need to explicitly Claim this power since ConfirmSectorProofsValid doesn't do it anymore
					claimParams := &power4.UpdateClaimedPowerParams{
						RawByteDelta:         types.NewInt(uint64(m.SectorSize)),
						QualityAdjustedDelta: sectorWeight,
					}

					_, err = doExecValue(ctx, vmi, power.Address, minerInfos[i].maddr, big.Zero(), power.Methods.UpdateClaimedPower, mustEnc(claimParams))
					if err != nil {
						return cid.Undef, xerrors.Errorf("failed to confirm presealed sectors: %w", err)
					}

					_, err = vmi.Flush()
					if err != nil {
						return cid.Undef, xerrors.Errorf("flushing vm: %w", err)
					}

					mact, find, err := vmi.StateTree().GetActor(ctx, minerInfos[i].maddr)
					if err != nil {
						return cid.Undef, xerrors.Errorf("getting miner actor: %w", err)
					}

					if !find {
						return cid.Undef, xerrors.New("actor not found")
					}

					mst, err := miner.Load(adt.WrapStore(ctx, cst), mact)
					if err != nil {
						return cid.Undef, xerrors.Errorf("getting miner state: %w", err)
					}

					if err = mst.EraseAllUnproven(); err != nil {
						return cid.Undef, xerrors.Errorf("failed to erase unproven sectors: %w", err)
					}

					mcid, err := cst.Put(ctx, mst.GetState())
					if err != nil {
						return cid.Undef, xerrors.Errorf("putting miner state: %w", err)
					}

					mact.Head = mcid

					if err = vmi.StateTree().SetActor(ctx, minerInfos[i].maddr, mact); err != nil {
						return cid.Undef, xerrors.Errorf("setting miner state: %w", err)
					}
				}
			}
		}
	}

	// Sanity-check total network power
	_, err = vmi.Flush()
	if err != nil {
		return cid.Undef, xerrors.Errorf("flushing vm: %w", err)
	}

	pact, find, err := vmi.StateTree().GetActor(ctx, power.Address)
	if err != nil {
		return cid.Undef, xerrors.Errorf("getting power actor: %w", err)
	}
	if !find {
		return cid.Undef, xerrors.New("actor not found")
	}

	pst, err := power.Load(adt.WrapStore(ctx, cst), pact)
	if err != nil {
		return cid.Undef, xerrors.Errorf("getting power state: %w", err)
	}

	pc, err := pst.TotalPower()
	if err != nil {
		return cid.Undef, xerrors.Errorf("getting total power: %w", err)
	}

	if !pc.RawBytePower.Equals(rawPow) {
		return cid.Undef, xerrors.Errorf("TotalRawBytePower (%s) doesn't match previously calculated rawPow (%s)", pc.RawBytePower, rawPow)
	}

	if !pc.QualityAdjPower.Equals(qaPow) {
		return cid.Undef, xerrors.Errorf("QualityAdjPower (%s) doesn't match previously calculated qaPow (%s)", pc.QualityAdjPower, qaPow)
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

func (fr *fakeRand) ChainGetRandomnessFromBeacon(ctx context.Context, personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	out := make([]byte, 32)
	_, _ = rand.New(rand.NewSource(int64(randEpoch * 1000))).Read(out) //nolint
	return out, nil
}

func (fr *fakeRand) ChainGetRandomnessFromTickets(ctx context.Context, personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	out := make([]byte, 32)
	_, _ = rand.New(rand.NewSource(int64(randEpoch * 1000))).Read(out) //nolint
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

func dealWeight(ctx context.Context, vmi vm.Interpreter, maddr address.Address, dealIDs []abi.DealID, sectorStart, sectorExpiry abi.ChainEpoch, av specactors.Version) (abi.DealWeight, abi.DealWeight, error) {
	// TODO: This hack should move to market actor wrapper
	if av <= specactors.Version2 {
		params := &market0.VerifyDealsForActivationParams{
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
			return big.Zero(), big.Zero(), err
		}
		if err := dealWeights.UnmarshalCBOR(bytes.NewReader(ret)); err != nil {
			return big.Zero(), big.Zero(), err
		}

		return dealWeights.DealWeight, dealWeights.VerifiedDealWeight, nil
	}
	params := &market4.VerifyDealsForActivationParams{Sectors: []market4.SectorDeals{{
		SectorExpiry: sectorExpiry,
		DealIDs:      dealIDs,
	}}}

	var dealWeights market4.VerifyDealsForActivationReturn
	ret, err := doExecValue(ctx, vmi,
		market.Address,
		maddr,
		abi.NewTokenAmount(0),
		market.Methods.VerifyDealsForActivation,
		mustEnc(params),
	)
	if err != nil {
		return big.Zero(), big.Zero(), err
	}
	if err := dealWeights.UnmarshalCBOR(bytes.NewReader(ret)); err != nil {
		return big.Zero(), big.Zero(), err
	}

	return dealWeights.Sectors[0].DealWeight, dealWeights.Sectors[0].VerifiedDealWeight, nil
}

func currentEpochBlockReward(ctx context.Context, vmi vm.Interpreter, maddr address.Address, av specactors.Version) (abi.StoragePower, builtin.FilterEstimate, error) {
	rwret, err := doExecValue(ctx, vmi, reward.Address, maddr, big.Zero(), reward.Methods.ThisEpochReward, nil)
	if err != nil {
		return big.Zero(), builtin.FilterEstimate{}, err
	}

	// TODO: This hack should move to reward actor wrapper
	if av <= specactors.Version2 {
		var epochReward reward0.ThisEpochRewardReturn

		if err := epochReward.UnmarshalCBOR(bytes.NewReader(rwret)); err != nil {
			return big.Zero(), builtin.FilterEstimate{}, err
		}

		return epochReward.ThisEpochBaselinePower, *epochReward.ThisEpochRewardSmoothed, nil
	}

	var epochReward reward4.ThisEpochRewardReturn

	if err := epochReward.UnmarshalCBOR(bytes.NewReader(rwret)); err != nil {
		return big.Zero(), builtin.FilterEstimate{}, err
	}

	return epochReward.ThisEpochBaselinePower, builtin.FilterEstimate(epochReward.ThisEpochRewardSmoothed), nil
}

// todo what is actually called here is vmi.vmOption.CircSupplyCalculator(context.TODO(), height, st) -> L55, So there is no structure UnsafeVM ???
func circSupply(ctx context.Context, vmi vm.Interpreter, maddr address.Address) abi.TokenAmount {
	//unsafeVM := &vm.UnsafeVM{VM: vmi.}
	//rt := unsafeVM.MakeRuntime(ctx, &types.Message{
	//	GasLimit: 1_000_000_000,
	//	From:     maddr,
	//})
	//
	//return rt.TotalFilCircSupply()

	return big.Zero()
}
