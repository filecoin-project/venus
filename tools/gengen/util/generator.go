package gengen

import (
	"bytes"
	"context"
	"fmt"
	xerrors "github.com/pkg/errors"
	"io"
	mrand "math/rand"

	address "github.com/filecoin-project/go-address"
	amt "github.com/filecoin-project/go-amt-ipld/v2"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/builtin/account"
	"github.com/filecoin-project/specs-actors/actors/builtin/cron"
	init_ "github.com/filecoin-project/specs-actors/actors/builtin/init"
	"github.com/filecoin-project/specs-actors/actors/builtin/market"
	"github.com/filecoin-project/specs-actors/actors/builtin/miner"
	"github.com/filecoin-project/specs-actors/actors/builtin/power"
	"github.com/filecoin-project/specs-actors/actors/builtin/reward"
	"github.com/filecoin-project/specs-actors/actors/builtin/system"
	"github.com/filecoin-project/specs-actors/actors/builtin/verifreg"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	cid "github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/libp2p/go-libp2p-core/peer"
	mh "github.com/multiformats/go-multihash"

	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/crypto"
	"github.com/filecoin-project/venus/internal/pkg/enccid"
	"github.com/filecoin-project/venus/internal/pkg/encoding"
	"github.com/filecoin-project/venus/internal/pkg/fork"
	"github.com/filecoin-project/venus/internal/pkg/genesis"
	"github.com/filecoin-project/venus/internal/pkg/params"
	"github.com/filecoin-project/venus/internal/pkg/proofs"
	gfcstate "github.com/filecoin-project/venus/internal/pkg/state"
	"github.com/filecoin-project/venus/internal/pkg/types"
	"github.com/filecoin-project/venus/internal/pkg/vm"
	"github.com/filecoin-project/venus/internal/pkg/vm/state"
	"github.com/filecoin-project/venus/internal/pkg/vmsupport"
)

const InitialBaseFee = 100e6

// TODO: review add bu force
// TODO: make a list/schedule of these.
var GenesisNetworkVersion = func() network.Version {
	// returns the version _before_ the first upgrade.
	if fork.UpgradeBreezeHeight >= 0 {
		return network.Version0
	}
	if fork.UpgradeSmokeHeight >= 0 {
		return network.Version1
	}
	if fork.UpgradeIgnitionHeight >= 0 {
		return network.Version2
	}
	if fork.UpgradeActorsV2Height >= 0 {
		return network.Version3
	}
	if fork.UpgradeLiftoffHeight >= 0 {
		return network.Version3
	}
	return params.ActorUpgradeNetworkVersion - 1 // genesis requires actors v0.
}()

func genesisNetworkVersion(context.Context, abi.ChainEpoch) network.Version {
	return GenesisNetworkVersion
}

type cstore struct {
	ctx context.Context
	cbor.IpldStore
}

func (s *cstore) Context() context.Context {
	return s.ctx
}

var (
	rewardActorInitialBalance = types.NewAttoFILFromFIL(1.4e9)
)

type GenesisGenerator struct {
	// actor state
	stateTree state.Tree
	store     *vm.Storage
	cst       cbor.IpldStore
	vm        genesis.VM

	keys      []*crypto.KeyInfo // Keys for pre-alloc accounts
	vrkey     *crypto.KeyInfo   // Key for verified registry root
	pnrg      *mrand.Rand
	chainRand crypto.ChainRandomnessSource
	cfg       *GenesisCfg
}

func NewGenesisGenerator(vmStorage *vm.Storage) *GenesisGenerator {
	csc := func(context.Context, abi.ChainEpoch, state.Tree) (abi.TokenAmount, error) {
		return big.Zero(), nil
	}

	g := GenesisGenerator{}
	var err error
	g.stateTree, err = state.NewState(vmStorage, state.StateTreeVersion1)
	if err != nil {
		panic(xerrors.Errorf("create state error, should never come here"))
	}
	g.store = vmStorage
	g.cst = vmStorage

	g.chainRand = crypto.ChainRandomnessSource{Sampler: &crypto.GenesisSampler{VRFProof: genesis.Ticket.VRFProof}}
	vmOption := vm.VmOption{
		CircSupplyCalculator: csc,
		NtwkVersionGetter:    genesisNetworkVersion,
		Rnd:                  &crypto.ChainRandomnessSource{Sampler: &crypto.GenesisSampler{VRFProof: genesis.Ticket.VRFProof}},
		BaseFee:              abi.NewTokenAmount(InitialBaseFee),
		Epoch:                0,
	}
	g.vm = vm.NewVM(g.stateTree, vmStorage, vmsupport.NewSyscalls(&vmsupport.NilFaultChecker{}, &proofs.FakeVerifier{}), vmOption).(genesis.VM)

	return &g
}

func (g *GenesisGenerator) Init(cfg *GenesisCfg) error {
	g.pnrg = mrand.New(mrand.NewSource(cfg.Seed))

	keys, err := genKeys(cfg.KeysToGen, g.pnrg)
	if err != nil {
		return err
	}
	keys = append(keys, cfg.ImportKeys...)
	g.keys = keys

	vrKey, err := crypto.NewSecpKeyFromSeed(g.pnrg)
	if err != nil {
		return err
	}
	g.vrkey = &vrKey

	// Monkey patch all proof types into the specs-actors package variable
	newSupportedTypes := make(map[abi.RegisteredSealProof]struct{})
	for _, mCfg := range cfg.Miners {
		newSupportedTypes[mCfg.SealProofType] = struct{}{}
	}
	// Switch reference rather than mutate in place to avoid concurrent map mutation (in tests).
	miner.SupportedProofTypes = newSupportedTypes

	g.cfg = cfg
	return nil
}

func (g *GenesisGenerator) flush(ctx context.Context) (cid.Cid, error) {
	err := g.store.Flush()
	if err != nil {
		return cid.Undef, err
	}
	return g.stateTree.Flush(ctx)
}

func (g *GenesisGenerator) createSingletonActor(ctx context.Context, addr address.Address, codeCid cid.Cid, balance abi.TokenAmount, stateFn func() (interface{}, error)) (*types.Actor, error) {
	if addr.Protocol() != address.ID {
		return nil, fmt.Errorf("non-singleton actor would be missing from Init actor's address table")
	}
	state, err := stateFn()
	if err != nil {
		return nil, fmt.Errorf("failed to create state")
	}
	headCid, err := g.store.Put(context.Background(), state)
	if err != nil {
		return nil, fmt.Errorf("failed to store state")
	}

	a := types.Actor{
		Code:       enccid.NewCid(codeCid),
		CallSeqNum: 0,
		Balance:    balance,
		Head:       enccid.NewCid(headCid),
	}
	if err := g.stateTree.SetActor(ctx, addr, &a); err != nil {
		return nil, fmt.Errorf("failed to create actor during genesis block creation")
	}

	return &a, nil
}

func (g *GenesisGenerator) updateSingletonActor(ctx context.Context, addr address.Address, stateFn func(actor2 *types.Actor) (interface{}, error)) (*types.Actor, error) {
	if addr.Protocol() != address.ID {
		return nil, fmt.Errorf("non-singleton actor would be missing from Init actor's address table")
	}
	oldActor, found, err := g.stateTree.GetActor(ctx, addr)
	if !found || err != nil {
		return nil, fmt.Errorf("failed to create state")
	}

	state, err := stateFn(oldActor)
	if err != nil {
		return nil, fmt.Errorf("failed to create state")
	}
	headCid, err := g.store.Put(context.Background(), state)
	if err != nil {
		return nil, fmt.Errorf("failed to store state")
	}

	a := types.Actor{
		Code:       oldActor.Code,
		CallSeqNum: 0,
		Balance:    oldActor.Balance,
		Head:       enccid.NewCid(headCid),
	}
	if err := g.stateTree.SetActor(ctx, addr, &a); err != nil {
		return nil, fmt.Errorf("failed to create actor during genesis block creation")
	}

	return &a, nil
}

func (g *GenesisGenerator) setupBuiltInActors(ctx context.Context) error {
	emptyMap, err := adt.MakeEmptyMap(g.vm.ContextStore()).Root()
	if err != nil {
		return err
	}
	emptyArray, err := adt.MakeEmptyArray(g.vm.ContextStore()).Root()
	if err != nil {
		return err
	}

	_, err = g.createSingletonActor(ctx, builtin.SystemActorAddr, builtin.SystemActorCodeID, big.Zero(), func() (interface{}, error) {
		return &system.State{}, nil
	})
	if err != nil {
		return err
	}

	_, err = g.createSingletonActor(ctx, builtin.CronActorAddr, builtin.CronActorCodeID, big.Zero(), func() (interface{}, error) {
		return &cron.State{Entries: cron.BuiltInEntries()}, nil
	})
	if err != nil {
		return err
	}

	_, err = g.createSingletonActor(ctx, builtin.InitActorAddr, builtin.InitActorCodeID, big.Zero(), func() (interface{}, error) {
		emptyMap, err := adt.MakeEmptyMap(g.vm.ContextStore()).Root()
		if err != nil {
			return nil, err
		}
		return init_.ConstructState(emptyMap, g.cfg.Network), nil
	})
	if err != nil {
		return err
	}

	_, err = g.createSingletonActor(ctx, builtin.RewardActorAddr, builtin.RewardActorCodeID, rewardActorInitialBalance, func() (interface{}, error) {
		return reward.ConstructState(big.Zero()), nil
	})
	if err != nil {
		return err
	}

	_, err = g.createSingletonActor(ctx, builtin.StoragePowerActorAddr, builtin.StoragePowerActorCodeID, big.Zero(), func() (interface{}, error) {
		emptyMap, err := adt.MakeEmptyMap(g.vm.ContextStore()).Root()
		if err != nil {
			return nil, err
		}

		multiMap, err := adt.AsMultimap(g.vm.ContextStore(), emptyMap)
		if err != nil {
			return nil, err
		}

		emptyMultiMap, err := multiMap.Root()
		if err != nil {
			return nil, err
		}
		return power.ConstructState(emptyMap, emptyMultiMap), nil
	})
	if err != nil {
		return err
	}

	_, err = g.createSingletonActor(ctx, builtin.StorageMarketActorAddr, builtin.StorageMarketActorCodeID, big.Zero(), func() (interface{}, error) {
		emptyMSet, err := market.MakeEmptySetMultimap(g.vm.ContextStore()).Root()
		if err != nil {
			return nil, err
		}
		return market.ConstructState(emptyArray, emptyMap, emptyMSet), nil
	})
	if err != nil {
		return err
	}

	_, err = g.createSingletonActor(ctx, builtin.VerifiedRegistryActorAddr, builtin.VerifiedRegistryActorCodeID, big.Zero(), func() (interface{}, error) {
		rootAddr, err := g.vrkey.Address()
		if err != nil {
			return nil, err
		}
		return verifreg.ConstructState(emptyMap, rootAddr), nil
	})
	if err != nil {
		return err
	}

	_, err = g.createSingletonActor(ctx, builtin.BurntFundsActorAddr, builtin.AccountActorCodeID, big.Zero(), func() (interface{}, error) {
		pkAddr, err := address.NewSecp256k1Address([]byte{})
		if err != nil {
			return nil, err
		}
		return &account.State{Address: pkAddr}, nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (g *GenesisGenerator) setupPrealloc() error {
	if len(g.keys) < len(g.cfg.PreallocatedFunds) {
		return fmt.Errorf("keys do not match prealloc")
	}

	for i, v := range g.cfg.PreallocatedFunds {
		ki := g.keys[i]
		addr, err := ki.Address()
		if err != nil {
			return err
		}

		value, ok := types.NewAttoFILFromFILString(v)
		if !ok {
			return fmt.Errorf("failed to parse FIL value '%s'", v)
		}

		_, err = g.vm.ApplyGenesisMessage(builtin.RewardActorAddr, addr, builtin.MethodSend, value, nil)
		if err != nil {
			return err
		}
	}
	return nil
}

func (g *GenesisGenerator) genBlock(ctx context.Context) (cid.Cid, error) {
	stateRoot, err := g.flush(ctx)
	if err != nil {
		return cid.Undef, err
	}
	// define empty cid and ensure empty components exist in blockstore
	emptyAMTCid, err := amt.FromArray(ctx, g.cst, nil)
	if err != nil {
		return cid.Undef, err
	}

	meta := types.TxMeta{SecpRoot: enccid.NewCid(emptyAMTCid), BLSRoot: enccid.NewCid(emptyAMTCid)}
	metaCid, err := g.cst.Put(ctx, meta)
	if err != nil {
		return cid.Undef, err
	}

	geneblk := &block.Block{
		Miner:           builtin.SystemActorAddr,
		Ticket:          genesis.Ticket,
		BeaconEntries:   []*block.BeaconEntry{{Data: []byte{0xca, 0xfe, 0xfa, 0xce}}},
		ElectionProof:   new(crypto.ElectionProof),
		Parents:         block.NewTipSetKey(),
		ParentWeight:    big.Zero(),
		Height:          0,
		StateRoot:       enccid.NewCid(stateRoot),
		MessageReceipts: enccid.NewCid(emptyAMTCid),
		Messages:        enccid.NewCid(metaCid),
		Timestamp:       g.cfg.Time,
		ForkSignaling:   0,
	}

	return g.cst.Put(ctx, geneblk)
}

func genKeys(cfgkeys int, pnrg io.Reader) ([]*crypto.KeyInfo, error) {
	keys := make([]*crypto.KeyInfo, cfgkeys)
	for i := 0; i < cfgkeys; i++ {
		ki, err := crypto.NewBLSKeyFromSeed(pnrg)
		if err != nil {
			return nil, err
		}
		keys[i] = &ki
	}
	return keys, nil
}

type sectorCommitInfo struct {
	miner          address.Address
	owner          address.Address
	comm           *CommitConfig
	dealIDs        []abi.DealID
	dealWeight     abi.DealWeight
	verifiedWeight abi.DealWeight
	rawPower       abi.StoragePower
	qaPower        abi.StoragePower
	expiration     abi.ChainEpoch
}

func (g *GenesisGenerator) setupMiners(ctx context.Context) ([]*RenderedMinerInfo, error) {
	var minfos []*RenderedMinerInfo

	var sectorsToCommit []*sectorCommitInfo
	networkQAPower := big.Zero()

	// First iterate all miners and sectors to compute sector info, and accumulate the total network power that
	// will be present (which determines the necessary pledge amounts).
	// One reason that this state can't be computed purely by applying messages is that we wish to compute the
	// initial pledge for the sectors based on the total genesis power, regardless of the order in which
	// sectors are inserted here.
	totalRawPow, totalQaPow := big.NewInt(0), big.NewInt(0)
	for _, m := range g.cfg.Miners {
		// Create miner actor
		ownerAddr, actorAddr, err := g.createMiner(ctx, m)
		if err != nil {
			return nil, err
		}

		mState, err := g.loadMinerState(ctx, actorAddr)
		if err != nil {
			return nil, err
		}

		// Add configured deals to the market actor with miner as provider and worker as client
		dealIDs := []abi.DealID{}
		if len(m.CommittedSectors) > 0 {
			ownerKey := g.keys[m.Owner]
			dealIDs, err = g.publishDeals(actorAddr, ownerAddr, ownerKey, m.CommittedSectors, m.MarketBalance)
			if err != nil {
				return nil, err
			}
		}

		minerQAPower := big.Zero()
		minerRawPower := big.Zero()
		for i, comm := range m.CommittedSectors {
			// Adjust sector expiration up to the epoch before the subsequent proving period starts.
			periodOffset := mState.ProvingPeriodStart % miner.WPoStProvingPeriod
			expiryOffset := abi.ChainEpoch(comm.DealCfg.EndEpoch+1) % miner.WPoStProvingPeriod
			sectorExpiration := abi.ChainEpoch(comm.DealCfg.EndEpoch) + miner.WPoStProvingPeriod + (periodOffset - expiryOffset)

			// Acquire deal weight value
			// call deal verify market actor to do calculation
			dealWeight, verifiedWeight, err := g.getDealWeight(dealIDs[i], sectorExpiration, actorAddr)
			if err != nil {
				return nil, err
			}

			sectorSize, err := m.SealProofType.SectorSize()
			if err != nil {
				return nil, err
			}
			rawPower, qaPower := computeSectorPower(sectorSize, sectorExpiration, dealWeight, verifiedWeight)

			sectorsToCommit = append(sectorsToCommit, &sectorCommitInfo{
				miner:          actorAddr,
				owner:          ownerAddr,
				comm:           comm,
				dealIDs:        []abi.DealID{dealIDs[i]},
				dealWeight:     dealWeight,
				verifiedWeight: verifiedWeight,
				rawPower:       rawPower,
				qaPower:        qaPower,
				expiration:     sectorExpiration,
			})
			minerQAPower = big.Add(minerQAPower, qaPower)
			minerRawPower = big.Add(minerRawPower, rawPower)
			networkQAPower = big.Add(networkQAPower, qaPower)
		}

		minfo := &RenderedMinerInfo{
			Address:  actorAddr,
			Owner:    m.Owner,
			RawPower: minerRawPower,
			QAPower:  minerQAPower,
		}
		minfos = append(minfos, minfo)
		totalRawPow = big.Add(totalRawPow, minerRawPower)
		totalQaPow = big.Add(totalQaPow, minerQAPower)
	}

	g.updateSingletonActor(ctx, builtin.StoragePowerActorAddr, func(actor *types.Actor) (interface{}, error) {
		var mState power.State
		err := g.store.Get(ctx, actor.Head.Cid, &mState)
		if err != nil {
			return nil, err
		}
		mState.TotalQualityAdjPower = totalQaPow
		mState.TotalRawBytePower = totalRawPow

		mState.ThisEpochQualityAdjPower = totalQaPow
		mState.ThisEpochRawBytePower = totalRawPow
		return &mState, nil
	})

	g.updateSingletonActor(ctx, builtin.RewardActorAddr, func(actor *types.Actor) (interface{}, error) {
		return reward.ConstructState(networkQAPower), nil
	})

	// Now commit the sectors and power updates.
	for _, sector := range sectorsToCommit {
		params := &miner.SectorPreCommitInfo{
			SealProof:     sector.comm.ProofType,
			SectorNumber:  sector.comm.SectorNum,
			SealedCID:     sector.comm.CommR,
			SealRandEpoch: -1,
			DealIDs:       sector.dealIDs,
			Expiration:    sector.expiration, // TODO: Allow setting externally!
		}

		dweight, err := g.dealWeight(ctx, sector.miner, params.DealIDs, 0, sector.expiration)
		if err != nil {
			return nil, xerrors.Errorf("getting deal weight: %w", err)
		}

		size, err := sector.comm.ProofType.SectorSize()
		if err != nil {
			return nil, xerrors.Errorf("failed to get sector size: %w", err)
		}
		sectorWeight := miner.QAPowerForWeight(size, sector.expiration, dweight.DealWeight, dweight.VerifiedDealWeight)

		// we've added fake power for this sector above, remove it now
		_, err = g.updateSingletonActor(ctx, builtin.StoragePowerActorAddr, func(actor *types.Actor) (interface{}, error) {
			var mState power.State
			err = g.store.Get(ctx, actor.Head.Cid, &mState)
			if err != nil {
				return nil, err
			}

			mState.TotalQualityAdjPower = big.Sub(mState.TotalQualityAdjPower, sectorWeight) //nolint:scopelint
			size, _ := sector.comm.ProofType.SectorSize()
			if err != nil {
				return nil, err
			}
			mState.TotalRawBytePower = big.Sub(mState.TotalRawBytePower, big.NewIntUnsigned(uint64(size)))
			return &mState, nil
		})

		if err != nil {
			return nil, xerrors.Errorf("removing fake power: %w", err)
		}

		epochReward, err := g.currentEpochBlockReward(ctx, sector.miner)
		if err != nil {
			return nil, xerrors.Errorf("getting current epoch reward: %w", err)
		}

		tpow, err := g.currentTotalPower(ctx, sector.miner)
		if err != nil {
			return nil, xerrors.Errorf("getting current total power: %w", err)
		}

		pcd := miner.PreCommitDepositForPower(epochReward.ThisEpochRewardSmoothed, tpow.QualityAdjPowerSmoothed, sectorWeight)

		pledge := miner.InitialPledgeForPower(
			sectorWeight,
			epochReward.ThisEpochBaselinePower,
			tpow.PledgeCollateral,
			epochReward.ThisEpochRewardSmoothed,
			tpow.QualityAdjPowerSmoothed,
			g.circSupply(ctx, sector.miner),
		)

		pledge = big.Add(pcd, pledge)

		encodeParams, _ := encoding.Encode(params)
		_, err = g.doExecValue(ctx, sector.miner, sector.owner, pledge, builtin.MethodsMiner.PreCommitSector, encodeParams)
		if err != nil {
			return nil, xerrors.Errorf("failed to confirm presealed sectors: %w", err)
		}

		// Commit one-by-one, otherwise pledge math tends to explode
		confirmParams := &builtin.ConfirmSectorProofsParams{
			Sectors: []abi.SectorNumber{sector.comm.SectorNum},
		}
		encodeParams, _ = encoding.Encode(confirmParams)
		_, err = g.doExecValue(ctx, sector.miner, builtin.StoragePowerActorAddr, big.Zero(), builtin.MethodsMiner.ConfirmSectorProofsValid, encodeParams)
		if err != nil {
			return nil, xerrors.Errorf("failed to confirm presealed sectors: %w", err)
		}
	}
	return minfos, nil
}

func (g *GenesisGenerator) loadMinerState(ctx context.Context, actorAddr address.Address) (*miner.State, error) {
	mAct, found, err := g.stateTree.GetActor(ctx, actorAddr)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("no such miner actor %s", actorAddr)
	}
	var mState miner.State
	err = g.store.Get(ctx, mAct.Head.Cid, &mState)
	if err != nil {
		return nil, err
	}
	return &mState, nil
}

func (g *GenesisGenerator) createMiner(ctx context.Context, m *CreateStorageMinerConfig) (address.Address, address.Address, error) {
	pkAddr, err := g.keys[m.Owner].Address()
	if err != nil {
		return address.Undef, address.Undef, err
	}

	// Resolve worker account's ID address.
	stateRoot, err := g.flush(ctx)
	if err != nil {
		return address.Undef, address.Undef, err
	}
	view := gfcstate.NewView(g.cst, stateRoot)
	ownerAddr, err := view.InitResolveAddress(ctx, pkAddr)
	if err != nil {
		return address.Undef, address.Undef, err
	}

	var pid peer.ID
	if m.PeerID != "" {
		p, err := peer.Decode(m.PeerID)
		if err != nil {
			return address.Undef, address.Undef, err
		}
		pid = p
	} else {
		// this is just deterministically deriving from the owner
		h, err := mh.Sum(ownerAddr.Bytes(), mh.SHA2_256, -1)
		if err != nil {
			return address.Undef, address.Undef, err
		}
		pid = peer.ID(h)
	}

	out, err := g.vm.ApplyGenesisMessage(ownerAddr, builtin.StoragePowerActorAddr, builtin.MethodsPower.CreateMiner, big.Zero(), &power.CreateMinerParams{
		Owner:         ownerAddr,
		Worker:        ownerAddr,
		Peer:          abi.PeerID(pid),
		SealProofType: m.SealProofType,
	})
	if err != nil {
		return address.Undef, address.Undef, err
	}

	if out.Receipt.ExitCode != 0 {
		return address.Undef, address.Undef, xerrors.Errorf("execute genesis msg error")
	}
	// get miner ID address
	createMinerReturn := power.CreateMinerReturn{}
	err = createMinerReturn.UnmarshalCBOR(bytes.NewReader(out.Receipt.ReturnValue))
	if err != nil {
		return address.Undef, address.Undef, err
	}
	return ownerAddr, createMinerReturn.IDAddress, nil
}

func (g *GenesisGenerator) publishDeals(actorAddr, clientAddr address.Address, clientkey *crypto.KeyInfo, comms []*CommitConfig, marketBalance abi.TokenAmount) ([]abi.DealID, error) {
	// Add 0 balance to escrow and locked table
	if marketBalance.GreaterThan(big.Zero()) {
		_, err := g.vm.ApplyGenesisMessage(clientAddr, builtin.StorageMarketActorAddr, builtin.MethodsMarket.AddBalance, marketBalance, &clientAddr)
		if err != nil {
			return nil, err
		}

		_, err = g.vm.ApplyGenesisMessage(clientAddr, builtin.StorageMarketActorAddr, builtin.MethodsMarket.AddBalance, marketBalance, &actorAddr)
		if err != nil {
			return nil, err
		}
	}

	// Add all deals to chain in one message
	params := &market.PublishStorageDealsParams{}
	for _, comm := range comms {
		mm := comm.DealCfg.CommP.Prefix()
		fmt.Println(mm)
		proposal := market.DealProposal{
			PieceCID:             comm.DealCfg.CommP,
			PieceSize:            abi.PaddedPieceSize(comm.DealCfg.PieceSize),
			VerifiedDeal:         comm.DealCfg.Verified,
			Client:               clientAddr,
			Provider:             actorAddr,
			StartEpoch:           0,
			EndEpoch:             abi.ChainEpoch(comm.DealCfg.EndEpoch),
			StoragePricePerEpoch: big.Zero(),
			ProviderCollateral:   big.Zero(), // collateral should actually be good
			ClientCollateral:     big.Zero(),
		}
		proposalBytes, err := encoding.Encode(&proposal)
		if err != nil {
			return nil, err
		}
		sig, err := crypto.Sign(proposalBytes, clientkey.PrivateKey, crypto.SigTypeBLS)
		if err != nil {
			return nil, err
		}

		params.Deals = append(params.Deals, market.ClientDealProposal{
			Proposal:        proposal,
			ClientSignature: sig,
		})
	}

	// apply deal builtin.MethodsMarket.PublishStorageDeals
	out, err := g.vm.ApplyGenesisMessage(clientAddr, builtin.StorageMarketActorAddr, builtin.MethodsMarket.PublishStorageDeals, big.Zero(), params)
	if err != nil {
		return nil, err
	}
	if out.Receipt.ExitCode != 0 {
		return nil, xerrors.Errorf("execute genesis msg error")
	}
	publishStoreageDealsReturn := market.PublishStorageDealsReturn{}
	err = publishStoreageDealsReturn.UnmarshalCBOR(bytes.NewReader(out.Receipt.ReturnValue))
	if err != nil {
		return nil, err
	}
	return publishStoreageDealsReturn.IDs, nil
}

func (g *GenesisGenerator) getDealWeight(dealID abi.DealID, sectorExpiry abi.ChainEpoch, minerIDAddr address.Address) (dealWeight, verifiedWeight abi.DealWeight, err error) {
	weightParams := &market.VerifyDealsForActivationParams{
		DealIDs:      []abi.DealID{dealID},
		SectorExpiry: sectorExpiry,
	}

	weightOut, err := g.vm.ApplyGenesisMessage(minerIDAddr, builtin.StorageMarketActorAddr, builtin.MethodsMarket.VerifyDealsForActivation, big.Zero(), weightParams)
	if err != nil {
		return big.Zero(), big.Zero(), err
	}
	if weightOut.Receipt.ExitCode != 0 {
		return big.Zero(), big.Zero(), xerrors.Errorf("execute genesis msg error")
	}
	verifyDealsReturn := market.VerifyDealsForActivationReturn{}
	err = verifyDealsReturn.UnmarshalCBOR(bytes.NewReader(weightOut.Receipt.ReturnValue))
	if err != nil {
		return big.Zero(), big.Zero(), err
	}
	return verifyDealsReturn.DealWeight, verifyDealsReturn.VerifiedDealWeight, nil
}

func (g *GenesisGenerator) updatePower(ctx context.Context, minerAddr address.Address, rawPower, qaPower, networkPower abi.StoragePower, epochBlockReward big.Int) (abi.TokenAmount, error) {
	// NOTE: it would be much better to use OnSectorProveCommit, which would then calculate the initial pledge amount.
	powAct, found, err := g.stateTree.GetActor(ctx, builtin.StoragePowerActorAddr)
	if err != nil {
		return big.Zero(), err
	}
	if !found {
		return big.Zero(), fmt.Errorf("state tree could not find power actor")
	}
	var powerState power.State
	err = g.store.Get(ctx, powAct.Head.Cid, &powerState)
	if err != nil {
		return big.Zero(), err
	}

	err = powerState.AddToClaim(&cstore{ctx, g.cst}, minerAddr, rawPower, qaPower)
	if err != nil {
		return big.Zero(), err
	}
	// Adjusting the total power here is technically wrong and unnecessary (it happens in AddToClaim),
	// but needed due to gain non-zero power in small networks when no minerAddr meets the consensus minimum.
	// At present, both impls ignore the consensus minimum and rely on this incorrect value.
	// See https://github.com/filecoin-project/specs-actors/issues/266
	//     https://github.com/filecoin-project/go-filecoin/issues/3958
	powerState.TotalRawBytePower = big.Add(powerState.TotalRawBytePower, rawPower)
	powerState.TotalQualityAdjPower = big.Add(powerState.TotalQualityAdjPower, qaPower)

	// Persist new state.
	newPowCid, err := g.store.Put(ctx, &powerState)
	if err != nil {
		return big.Zero(), err
	}
	powAct.Head = enccid.NewCid(newPowCid)
	err = g.stateTree.SetActor(ctx, builtin.StoragePowerActorAddr, powAct)
	if err != nil {
		return big.Zero(), err
	}

	initialPledge := big.Div(big.Mul(qaPower, epochBlockReward), networkPower)

	//minerAddr.ExpectedRewardForPower()
	return initialPledge, nil
}

func (g *GenesisGenerator) putSector(ctx context.Context, sector *sectorCommitInfo, pledge, storagePledge, dayReward abi.TokenAmount) error {
	mAct, found, err := g.stateTree.GetActor(ctx, sector.miner)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("mState tree could not find miner actor %s", sector.miner)
	}
	var mState miner.State
	err = g.store.Get(ctx, mAct.Head.Cid, &mState)
	if err != nil {
		return err
	}

	newSectorInfo := &miner.SectorOnChainInfo{
		SectorNumber:          sector.comm.SectorNum,
		SealProof:             sector.comm.ProofType,
		SealedCID:             sector.comm.CommR,
		DealIDs:               sector.dealIDs,
		Expiration:            sector.expiration,
		Activation:            0,
		DealWeight:            sector.dealWeight,
		VerifiedDealWeight:    sector.verifiedWeight,
		InitialPledge:         pledge,
		ExpectedDayReward:     dayReward,
		ExpectedStoragePledge: storagePledge,
	}
	err = mState.PutSectors(&cstore{ctx, g.cst}, newSectorInfo)
	if err != nil {
		return err
	}

	// Persist new state.
	newMinerCid, err := g.store.Put(ctx, &mState)
	if err != nil {
		return err
	}
	mAct.Head = enccid.NewCid(newMinerCid)
	err = g.stateTree.SetActor(ctx, sector.miner, mAct)
	return err
}

func (g *GenesisGenerator) doExecValue(ctx context.Context, to, from address.Address, value big.Int, method abi.MethodNum, params []byte) ([]byte, error) {
	_, found, err := g.stateTree.GetActor(ctx, from)
	if !found || err != nil {
		return nil, xerrors.Errorf("doExec failed to get from actor (%s): %w", from, err)
	}

	ret, err := g.vm.ApplyGenesisMessage(from, to, method, value, params)
	if err != nil {
		return nil, xerrors.Errorf("doExec apply message failed: %w", err)
	}
	if ret.Receipt.ExitCode != 0 {
		return nil, xerrors.Errorf("execute genesis msg error")
	}
	return ret.Receipt.ReturnValue, nil
}

func (g *GenesisGenerator) currentTotalPower(ctx context.Context, maddr address.Address) (*power.CurrentTotalPowerReturn, error) {
	pwret, err := g.doExecValue(ctx, builtin.StoragePowerActorAddr, maddr, big.Zero(), builtin.MethodsPower.CurrentTotalPower, nil)
	if err != nil {
		return nil, err
	}
	currentTotalReturn := &power.CurrentTotalPowerReturn{}
	err = currentTotalReturn.UnmarshalCBOR(bytes.NewReader(pwret))
	if err != nil {
		return nil, err
	}
	return currentTotalReturn, nil
}

func (g *GenesisGenerator) dealWeight(ctx context.Context, maddr address.Address, dealIDs []abi.DealID, sectorStart, sectorExpiry abi.ChainEpoch) (market.VerifyDealsForActivationReturn, error) {
	params := &market.VerifyDealsForActivationParams{
		DealIDs:      dealIDs,
		SectorStart:  sectorStart,
		SectorExpiry: sectorExpiry,
	}

	paramsBytes, err := encoding.Encode(params)
	if err != nil {
		return market.VerifyDealsForActivationReturn{}, err
	}
	ret, err := g.doExecValue(ctx,
		builtin.StorageMarketActorAddr,
		maddr,
		abi.NewTokenAmount(0),
		builtin.MethodsMarket.VerifyDealsForActivation,
		paramsBytes,
	)
	if err != nil {
		return market.VerifyDealsForActivationReturn{}, err
	}

	vdaReturn := market.VerifyDealsForActivationReturn{}
	err = vdaReturn.UnmarshalCBOR(bytes.NewReader(ret))
	if err != nil {
		return market.VerifyDealsForActivationReturn{}, err
	}
	return vdaReturn, nil
}

func (g *GenesisGenerator) currentEpochBlockReward(ctx context.Context, maddr address.Address) (*reward.ThisEpochRewardReturn, error) {
	rwret, err := g.doExecValue(ctx, builtin.RewardActorAddr, maddr, big.Zero(), builtin.MethodsReward.ThisEpochReward, nil)
	if err != nil {
		return nil, err
	}

	epochRewardReturn := &reward.ThisEpochRewardReturn{}
	err = epochRewardReturn.UnmarshalCBOR(bytes.NewReader(rwret))
	if err != nil {
		return nil, err
	}
	return epochRewardReturn, nil
}

func (g *GenesisGenerator) circSupply(ctx context.Context, maddr address.Address) abi.TokenAmount {
	supply, _ := g.vm.TotalFilCircSupply(0, g.stateTree)
	return supply
}

func computeSectorPower(size abi.SectorSize, duration abi.ChainEpoch, dealWeight, verifiedDealWeight abi.DealWeight) (abi.StoragePower, abi.StoragePower) {
	spower := big.NewIntUnsigned(uint64(size))
	qapower := miner.QAPowerForWeight(size, duration, dealWeight, verifiedDealWeight)
	return spower, qapower
}
