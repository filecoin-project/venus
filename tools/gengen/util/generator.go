package gengen

import (
	"context"
	"fmt"
	"io"
	mrand "math/rand"

	address "github.com/filecoin-project/go-address"
	amt "github.com/filecoin-project/go-amt-ipld/v2"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
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
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/libp2p/go-libp2p-core/peer"
	mh "github.com/multiformats/go-multihash"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/cborutil"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	e "github.com/filecoin-project/go-filecoin/internal/pkg/enccid"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/genesis"
	"github.com/filecoin-project/go-filecoin/internal/pkg/proofs"
	gfcstate "github.com/filecoin-project/go-filecoin/internal/pkg/state"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vmsupport"
)

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
	store     vm.Storage
	cst       cbor.IpldStore
	vm        genesis.VM

	keys      []*crypto.KeyInfo // Keys for pre-alloc accounts
	vrkey     *crypto.KeyInfo   // Key for verified registry root
	pnrg      *mrand.Rand
	chainRand crypto.ChainRandomnessSource
	cfg       *GenesisCfg
}

func NewGenesisGenerator(bs blockstore.Blockstore) *GenesisGenerator {
	cst := cborutil.NewIpldStore(bs)
	g := GenesisGenerator{}
	g.stateTree = state.NewState(cst)
	g.store = vm.NewStorage(bs)
	g.vm = vm.NewVM(g.stateTree, &g.store, vmsupport.NewSyscalls(&vmsupport.NilFaultChecker{}, &proofs.FakeVerifier{})).(genesis.VM)
	g.cst = cst

	g.chainRand = crypto.ChainRandomnessSource{Sampler: &crypto.GenesisSampler{VRFProof: genesis.Ticket.VRFProof}}
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

	g.cfg = cfg
	return nil
}

func (g *GenesisGenerator) flush(ctx context.Context) (cid.Cid, error) {
	err := g.store.Flush()
	if err != nil {
		return cid.Undef, err
	}
	return g.stateTree.Commit(ctx)
}

func (g *GenesisGenerator) createSingletonActor(ctx context.Context, addr address.Address, codeCid cid.Cid, balance abi.TokenAmount, stateFn func() (interface{}, error)) (*actor.Actor, error) {
	if addr.Protocol() != address.ID {
		return nil, fmt.Errorf("non-singleton actor would be missing from Init actor's address table")
	}
	state, err := stateFn()
	if err != nil {
		return nil, fmt.Errorf("failed to create state")
	}
	headCid, _, err := g.store.Put(context.Background(), state)
	if err != nil {
		return nil, fmt.Errorf("failed to store state")
	}

	a := actor.Actor{
		Code:       e.NewCid(codeCid),
		CallSeqNum: 0,
		Balance:    balance,
		Head:       e.NewCid(headCid),
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
		return &cron.State{Entries: []cron.Entry{{
			Receiver:  builtin.StoragePowerActorAddr,
			MethodNum: builtin.MethodsPower.OnEpochTickEnd,
		}}}, nil
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
		return reward.ConstructState(emptyMap), nil
	})
	if err != nil {
		return err
	}

	_, err = g.createSingletonActor(ctx, builtin.StoragePowerActorAddr, builtin.StoragePowerActorCodeID, big.Zero(), func() (interface{}, error) {
		emptyMap, err := adt.MakeEmptyMap(g.vm.ContextStore()).Root()
		if err != nil {
			return nil, err
		}
		return power.ConstructState(emptyMap), nil
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

		_, err = g.vm.ApplyGenesisMessage(builtin.RewardActorAddr, addr, builtin.MethodSend, value, nil, &g.chainRand)
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

	meta := types.TxMeta{SecpRoot: e.NewCid(emptyAMTCid), BLSRoot: e.NewCid(emptyAMTCid)}
	metaCid, err := g.cst.Put(ctx, meta)
	if err != nil {
		return cid.Undef, err
	}

	geneblk := &block.Block{
		Miner:           builtin.SystemActorAddr,
		Ticket:          genesis.Ticket,
		Parents:         block.NewTipSetKey(),
		ParentWeight:    big.Zero(),
		Height:          0,
		StateRoot:       e.NewCid(stateRoot),
		MessageReceipts: e.NewCid(emptyAMTCid),
		Messages:        e.NewCid(metaCid),
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

	// Estimate the first epoch's block reward as a linear release over 6 years.
	// The actual release will be initially faster, with exponential decay.
	// Replace this code with calls to the reward actor when it's fixed.
	// See https://github.com/filecoin-project/specs-actors/issues/317
	sixYearEpochs := 6 * 365 * 86400 / miner.EpochDurationSeconds
	initialBlockReward := big.Div(rewardActorInitialBalance, big.NewInt(int64(sixYearEpochs)))

	// First iterate all miners and sectors to compute sector info, and accumulate the total network power that
	// will be present (which determines the necessary pledge amounts).
	// One reason that this state can't be computed purely by applying messages is that we wish to compute the
	// initial pledge for the sectors based on the total genesis power, regardless of the order in which
	// sectors are inserted here.
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
			dealIDs, err = g.publishDeals(actorAddr, ownerAddr, ownerKey, m.CommittedSectors)
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

			rawPower, qaPower := computeSectorPower(m.SectorSize, sectorExpiration, dealWeight, verifiedWeight)

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
	}

	// Now commit the sectors and power updates.
	for _, sector := range sectorsToCommit {
		// Update power, setting state directly
		sectorPledge, err := g.updatePower(ctx, sector.miner, sector.rawPower, sector.qaPower, networkQAPower, initialBlockReward)
		if err != nil {
			return nil, err
		}

		// Put sector info in miner sector set.
		err = g.putSector(ctx, sector, sectorPledge)
		if err != nil {
			return nil, err
		}

		// Transfer the pledge amount from the owner to the miner actor
		_, err = g.vm.ApplyGenesisMessage(sector.owner, sector.miner, builtin.MethodSend, sectorPledge, nil, &g.chainRand)
		if err != nil {
			return nil, err
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
	_, err = g.store.Get(ctx, mAct.Head.Cid, &mState)
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
		Owner:      ownerAddr,
		Worker:     ownerAddr,
		Peer:       pid,
		SectorSize: m.SectorSize,
	}, &g.chainRand)
	if err != nil {
		return address.Undef, address.Undef, err
	}

	// get miner ID address
	ret := out.(*power.CreateMinerReturn)
	return ownerAddr, ret.IDAddress, nil
}

func (g *GenesisGenerator) publishDeals(actorAddr, clientAddr address.Address, clientkey *crypto.KeyInfo, comms []*CommitConfig) ([]abi.DealID, error) {
	// Add 0 balance to escrow and locked table
	_, err := g.vm.ApplyGenesisMessage(clientAddr, builtin.StorageMarketActorAddr, builtin.MethodsMarket.AddBalance, big.Zero(), &clientAddr, &g.chainRand)
	if err != nil {
		return nil, err
	}
	_, err = g.vm.ApplyGenesisMessage(clientAddr, builtin.StorageMarketActorAddr, builtin.MethodsMarket.AddBalance, big.Zero(), &actorAddr, &g.chainRand)
	if err != nil {
		return nil, err
	}

	// Add all deals to chain in one message
	params := &market.PublishStorageDealsParams{}
	for _, comm := range comms {
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
	out, err := g.vm.ApplyGenesisMessage(clientAddr, builtin.StorageMarketActorAddr, builtin.MethodsMarket.PublishStorageDeals, big.Zero(), params, &g.chainRand)
	if err != nil {
		return nil, err
	}

	ret := out.(*market.PublishStorageDealsReturn)
	return ret.IDs, nil
}

func (g *GenesisGenerator) getDealWeight(dealID abi.DealID, sectorExpiry abi.ChainEpoch, minerIDAddr address.Address) (dealWeight, verifiedWeight abi.DealWeight, err error) {
	weightParams := &market.VerifyDealsOnSectorProveCommitParams{
		DealIDs:      []abi.DealID{dealID},
		SectorExpiry: sectorExpiry,
	}

	weightOut, err := g.vm.ApplyGenesisMessage(minerIDAddr, builtin.StorageMarketActorAddr, builtin.MethodsMarket.VerifyDealsOnSectorProveCommit, big.Zero(), weightParams, &g.chainRand)
	if err != nil {
		return big.Zero(), big.Zero(), err
	}
	ret := weightOut.(*market.VerifyDealsOnSectorProveCommitReturn)
	return ret.DealWeight, ret.VerifiedDealWeight, nil
}

func (g *GenesisGenerator) updatePower(ctx context.Context, miner address.Address, rawPower, qaPower, networkPower abi.StoragePower, epochBlockReward big.Int) (abi.TokenAmount, error) {
	// NOTE: it would be much better to use OnSectorProveCommit, which would then calculate the initial pledge amount.
	powAct, found, err := g.stateTree.GetActor(ctx, builtin.StoragePowerActorAddr)
	if err != nil {
		return big.Zero(), err
	}
	if !found {
		return big.Zero(), fmt.Errorf("state tree could not find power actor")
	}
	var powerState power.State
	_, err = g.store.Get(ctx, powAct.Head.Cid, &powerState)
	if err != nil {
		return big.Zero(), err
	}

	err = powerState.AddToClaim(&cstore{ctx, g.cst}, miner, rawPower, qaPower)
	if err != nil {
		return big.Zero(), err
	}
	// Adjusting the total power here is technically wrong and unnecessary (it happens in AddToClaim),
	// but needed due to gain non-zero power in small networks when no miner meets the consensus minimum.
	// At present, both impls ignore the consensus minimum and rely on this incorrect value.
	// See https://github.com/filecoin-project/specs-actors/issues/266
	//     https://github.com/filecoin-project/go-filecoin/issues/3958
	powerState.TotalRawBytePower = big.Add(powerState.TotalRawBytePower, rawPower)
	powerState.TotalQualityAdjPower = big.Add(powerState.TotalQualityAdjPower, qaPower)

	// Persist new state.
	newPowCid, _, err := g.store.Put(ctx, &powerState)
	if err != nil {
		return big.Zero(), err
	}
	powAct.Head = e.NewCid(newPowCid)
	err = g.stateTree.SetActor(ctx, builtin.StoragePowerActorAddr, powAct)
	if err != nil {
		return big.Zero(), err
	}

	initialPledge := big.Div(big.Mul(qaPower, epochBlockReward), networkPower)
	return initialPledge, nil
}

func (g *GenesisGenerator) putSector(ctx context.Context, sector *sectorCommitInfo, pledge abi.TokenAmount) error {
	mAct, found, err := g.stateTree.GetActor(ctx, sector.miner)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("mState tree could not find miner actor %s", sector.miner)
	}
	var mState miner.State
	_, err = g.store.Get(ctx, mAct.Head.Cid, &mState)
	if err != nil {
		return err
	}

	newSectorInfo := &miner.SectorOnChainInfo{
		Info: miner.SectorPreCommitInfo{
			RegisteredProof: sector.comm.ProofType,
			SectorNumber:    sector.comm.SectorNum,
			SealedCID:       sector.comm.CommR,
			SealRandEpoch:   0,
			DealIDs:         sector.dealIDs,
			Expiration:      sector.expiration,
		},
		ActivationEpoch:    0,
		DealWeight:         sector.dealWeight,
		VerifiedDealWeight: sector.verifiedWeight,
	}
	err = mState.PutSector(&cstore{ctx, g.cst}, newSectorInfo)
	if err != nil {
		return err
	}

	err = mState.AddNewSectors(sector.comm.SectorNum)
	if err != nil {
		return err
	}

	// Persist new state.
	newMinerCid, _, err := g.store.Put(ctx, &mState)
	if err != nil {
		return err
	}
	mAct.Head = e.NewCid(newMinerCid)
	err = g.stateTree.SetActor(ctx, sector.miner, mAct)
	return err
}

func computeSectorPower(size abi.SectorSize, duration abi.ChainEpoch, dealWeight, verifiedDealWeight abi.DealWeight) (abi.StoragePower, abi.StoragePower) {
	weight := &power.SectorStorageWeightDesc{
		SectorSize:         size,
		Duration:           duration,
		DealWeight:         dealWeight,
		VerifiedDealWeight: verifiedDealWeight,
	}
	spower := big.NewIntUnsigned(uint64(size))
	qapower := power.QAPowerForWeight(weight)
	return spower, qapower
}
