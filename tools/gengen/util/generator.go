package gengen

import (
	"context"
	"fmt"
	"io"
	mrand "math/rand"
	"sort"
	"strconv"

	address "github.com/filecoin-project/go-address"
	amt "github.com/filecoin-project/go-amt-ipld/v2"
	"github.com/filecoin-project/specs-actors/actors/abi"
	specsbig "github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/builtin/cron"
	init_ "github.com/filecoin-project/specs-actors/actors/builtin/init"
	"github.com/filecoin-project/specs-actors/actors/builtin/market"
	"github.com/filecoin-project/specs-actors/actors/builtin/miner"
	"github.com/filecoin-project/specs-actors/actors/builtin/power"
	"github.com/filecoin-project/specs-actors/actors/builtin/reward"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	cid "github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/libp2p/go-libp2p-core/peer"
	mh "github.com/multiformats/go-multihash"

	bls "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/cborutil"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	e "github.com/filecoin-project/go-filecoin/internal/pkg/enccid"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
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
	defaultAccounts map[address.Address]abi.TokenAmount
)

func init() {
	defaultAccounts = map[address.Address]abi.TokenAmount{
		builtin.BurntFundsActorAddr: abi.NewTokenAmount(0),
	}
}

type GenesisGenerator struct {
	// actor state
	stateTree state.Tree
	store     vm.Storage
	cst       cbor.IpldStore
	vm        consensus.GenesisVM

	keys      []*crypto.KeyInfo
	pnrg      *mrand.Rand
	chainRand crypto.ChainRandomnessSource
	cfg       *GenesisCfg
}

func NewGenesisGenerator(bs blockstore.Blockstore) *GenesisGenerator {
	cst := cborutil.NewIpldStore(bs)
	g := GenesisGenerator{}
	g.stateTree = state.NewState(cst)
	g.store = vm.NewStorage(bs)
	g.vm = vm.NewVM(g.stateTree, &g.store, vmsupport.NewSyscalls(&vmsupport.NilFaultChecker{}, &proofs.FakeVerifier{})).(consensus.GenesisVM)
	g.cst = cst

	g.chainRand = crypto.ChainRandomnessSource{Sampler: &crypto.GenesisSampler{VRFProof: consensus.GenesisTicket.VRFProof}}
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

func (g *GenesisGenerator) createActor(addr address.Address, codeCid cid.Cid, balance abi.TokenAmount, stateFn func() (interface{}, error)) (*actor.Actor, error) {
	a := actor.Actor{
		Code:       e.NewCid(codeCid),
		CallSeqNum: 0,
		Balance:    balance,
	}
	if stateFn != nil {
		state, err := stateFn()
		if err != nil {
			return nil, fmt.Errorf("failed to create state")
		}
		headCid, _, err := g.store.Put(context.Background(), state)
		if err != nil {
			return nil, fmt.Errorf("failed to store state")
		}
		a.Head = e.NewCid(headCid)
	}
	if err := g.stateTree.SetActor(context.Background(), addr, &a); err != nil {
		return nil, fmt.Errorf("failed to create actor during genesis block creation")
	}

	return &a, nil
}

func (g *GenesisGenerator) setupDefaultActors(ctx context.Context) error {

	_, err := g.createActor(builtin.SystemActorAddr, builtin.SystemActorCodeID, specsbig.Zero(), func() (interface{}, error) {
		return &adt.EmptyValue{}, nil
	})
	if err != nil {
		return err
	}

	_, err = g.createActor(builtin.CronActorAddr, builtin.CronActorCodeID, specsbig.Zero(), func() (interface{}, error) {
		return &cron.State{Entries: []cron.Entry{{
			Receiver:  builtin.StoragePowerActorAddr,
			MethodNum: builtin.MethodsPower.OnEpochTickEnd,
		}}}, nil
	})
	if err != nil {
		return err
	}

	_, err = g.createActor(builtin.InitActorAddr, builtin.InitActorCodeID, specsbig.Zero(), func() (interface{}, error) {
		emptyMap, err := adt.MakeEmptyMap(g.vm.ContextStore())
		if err != nil {
			return nil, err
		}
		return init_.ConstructState(emptyMap.Root(), g.cfg.Network), nil
	})
	if err != nil {
		return err
	}

	rewardActor, err := g.createActor(builtin.RewardActorAddr, builtin.RewardActorCodeID, specsbig.Zero(), func() (interface{}, error) {
		emptyMap, err := adt.MakeEmptyMap(g.vm.ContextStore())
		if err != nil {
			return nil, err
		}
		return reward.ConstructState(emptyMap.Root()), nil
	})
	if err != nil {
		return err
	}
	rewardActor.Balance = abi.NewTokenAmount(100_000_000_000_000_000)
	if err := g.stateTree.SetActor(ctx, builtin.RewardActorAddr, rewardActor); err != nil {
		return err
	}

	_, err = g.createActor(builtin.StoragePowerActorAddr, builtin.StoragePowerActorCodeID, specsbig.Zero(), func() (interface{}, error) {
		emptyMap, err := adt.MakeEmptyMap(g.vm.ContextStore())
		if err != nil {
			return nil, err
		}
		return power.ConstructState(emptyMap.Root()), nil
	})
	if err != nil {
		return err
	}

	_, err = g.createActor(builtin.StorageMarketActorAddr, builtin.StorageMarketActorCodeID, specsbig.Zero(), func() (interface{}, error) {
		emptyArray, err := adt.MakeEmptyArray(g.vm.ContextStore())
		if err != nil {
			return nil, err
		}

		emptyMap, err := adt.MakeEmptyMap(g.vm.ContextStore())
		if err != nil {
			return nil, err
		}

		emptyMSet, err := market.MakeEmptySetMultimap(g.vm.ContextStore())
		if err != nil {
			return nil, err
		}
		return market.ConstructState(emptyArray.Root(), emptyMap.Root(), emptyMSet.Root()), nil
	})
	if err != nil {
		return err
	}

	// sort addresses so genesis generation will be stable
	sortedAddresses := []string{}
	for addr := range defaultAccounts {
		sortedAddresses = append(sortedAddresses, string(addr.Bytes()))
	}
	sort.Strings(sortedAddresses)
	// BurntFunds
	for _, addrBytes := range sortedAddresses {
		addr, err := address.NewFromBytes([]byte(addrBytes))
		if err != nil {
			return err
		}
		val := defaultAccounts[addr]
		if addr.Protocol() == address.ID {
			a := actor.NewActor(builtin.AccountActorCodeID, val)

			if err := g.stateTree.SetActor(ctx, addr, a); err != nil {
				return err
			}
		} else {
			_, err = g.vm.ApplyGenesisMessage(builtin.RewardActorAddr, addr, builtin.MethodSend, val, nil, &g.chainRand)
			if err != nil {
				return err
			}
		}

		if err != nil {
			return err
		}
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

		valint, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return err
		}

		_, err = g.vm.ApplyGenesisMessage(builtin.RewardActorAddr, addr,
			builtin.MethodSend, abi.NewTokenAmount(int64(valint)), nil, &g.chainRand)
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
		Miner:  builtin.SystemActorAddr,
		Ticket: consensus.GenesisTicket,
		EPoStInfo: block.EPoStInfo{
			PoStProofs: []block.EPoStProof{},
			VRFProof:   abi.PoStRandomness(make([]byte, 32)),
			Winners:    []block.EPoStCandidate{},
		},
		Parents:         block.NewTipSetKey(),
		ParentWeight:    specsbig.Zero(),
		Height:          0,
		StateRoot:       e.NewCid(stateRoot),
		MessageReceipts: e.NewCid(emptyAMTCid),
		Messages:        e.NewCid(metaCid),
		BLSAggregateSig: crypto.Signature{
			Type: crypto.SigTypeBLS,
			Data: bls.Aggregate(nil)[:],
		},
		Timestamp:     g.cfg.Time,
		BlockSig:      &crypto.Signature{Type: crypto.SigTypeBLS, Data: []byte{}},
		ForkSignaling: 0,
	}

	return g.cst.Put(ctx, geneblk)
}

func genKeys(cfgkeys int, pnrg io.Reader) ([]*crypto.KeyInfo, error) {
	keys := make([]*crypto.KeyInfo, cfgkeys)
	for i := 0; i < cfgkeys; i++ {
		ki := crypto.NewBLSKeyRandom() // TODO: use seed, https://github.com/filecoin-project/go-filecoin/issues/3781
		keys[i] = &ki
	}

	return keys, nil
}

func (g *GenesisGenerator) setupMiners(ctx context.Context) ([]*RenderedMinerInfo, error) {
	var minfos []*RenderedMinerInfo

	// Setup total network power
	err := g.initPowerTable(ctx)
	if err != nil {
		return nil, err
	}

	for _, m := range g.cfg.Miners {
		// Create miner actor
		wAddr, mIDAddr, err := g.createMiner(ctx, m)
		if err != nil {
			return nil, err
		}

		// Add configured deals to the market actor with miner as provider and worker as client
		dealIDs, err := g.commitDeals(ctx, wAddr, mIDAddr, m)
		if err != nil {
			return nil, err
		}

		// Setup windowed post including cron registration
		err = g.setupPost(ctx, wAddr, mIDAddr, m.ProvingPeriodStart)
		if err != nil {
			return nil, err
		}

		for i, comm := range m.CommittedSectors {
			// Acquire deal weight value
			// call deal verify market actor to do calculation
			dealWeight, err := g.getDealWeight(dealIDs[i], abi.ChainEpoch(comm.DealCfg.EndEpoch), mIDAddr)
			if err != nil {
				return nil, err
			}

			// Update Power
			// directly set power actor state to new power accounting for deal weight duration and sector size
			pledge, err := g.updatePower(ctx, dealWeight, m.SectorSize, abi.ChainEpoch(comm.DealCfg.EndEpoch), mIDAddr)
			if err != nil {
				return nil, err
			}

			// Put sectors in miner sector sets
			// add sector info for given sector to proving set and sector set and register sector expiry on cron
			err = g.putSectors(ctx, comm, mIDAddr, dealIDs[i], dealWeight, pledge)
			if err != nil {
				return nil, err
			}
		}
		power, err := g.getPower(ctx, mIDAddr)
		if err != nil {
			return nil, err
		}
		minfo := &RenderedMinerInfo{
			Address: mIDAddr,
			Owner:   m.Owner,
			Power:   power,
		}
		minfos = append(minfos, minfo)
	}
	return minfos, nil
}

func (g *GenesisGenerator) initPowerTable(ctx context.Context) error {
	networkPower := specsbig.Zero()
	for _, m := range g.cfg.Miners {
		networkPower = specsbig.Add(networkPower, specsbig.NewInt(int64(m.SectorSize)*int64(len(m.CommittedSectors))))
	}
	powAct, found, err := g.stateTree.GetActor(ctx, builtin.StoragePowerActorAddr)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("state tree could not find power actor")
	}
	var powerActorState power.State
	_, err = g.store.Get(ctx, powAct.Head.Cid, &powerActorState)
	if err != nil {
		return err
	}
	powerActorState.TotalNetworkPower = networkPower
	newPowCid, _, err := g.store.Put(ctx, &powerActorState)
	if err != nil {
		return err
	}
	powAct.Head = e.NewCid(newPowCid)
	return g.stateTree.SetActor(ctx, builtin.StoragePowerActorAddr, powAct)
}

func (g *GenesisGenerator) createMiner(ctx context.Context, m *CreateStorageMinerConfig) (address.Address, address.Address, error) {
	wAddr, err := g.keys[m.Owner].Address()
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
		h, err := mh.Sum(wAddr.Bytes(), mh.SHA2_256, -1)
		if err != nil {
			return address.Undef, address.Undef, err
		}
		pid = peer.ID(h)
	}

	// give collateral to account actor
	_, err = g.vm.ApplyGenesisMessage(builtin.RewardActorAddr, wAddr, builtin.MethodSend, abi.NewTokenAmount(100000), nil, &g.chainRand)
	if err != nil {
		return address.Undef, address.Undef, err
	}

	out, err := g.vm.ApplyGenesisMessage(wAddr, builtin.StoragePowerActorAddr, builtin.MethodsPower.CreateMiner, abi.NewTokenAmount(100000), &power.CreateMinerParams{
		Owner:      wAddr,
		Worker:     wAddr,
		Peer:       pid,
		SectorSize: m.SectorSize,
	}, &g.chainRand)
	if err != nil {
		return address.Undef, address.Undef, err
	}

	// get miner ID address
	ret := out.(*power.CreateMinerReturn)
	return wAddr, ret.IDAddress, nil
}

func (g *GenesisGenerator) commitDeals(ctx context.Context, addr, mIDAddr address.Address, m *CreateStorageMinerConfig) ([]abi.DealID, error) {

	comms := m.CommittedSectors
	// Look up worker address's id address
	stateRoot, err := g.flush(ctx)
	if err != nil {
		return nil, err
	}
	view := gfcstate.NewView(g.cst, stateRoot)
	workerID, err := view.InitResolveAddress(ctx, addr)
	if err != nil {
		return nil, err
	}

	// Add 0 balance to escrow and locked table
	_, err = g.vm.ApplyGenesisMessage(addr, builtin.StorageMarketActorAddr, builtin.MethodsMarket.AddBalance, specsbig.Zero(), &workerID, &g.chainRand)
	if err != nil {
		return nil, err
	}
	_, err = g.vm.ApplyGenesisMessage(addr, builtin.StorageMarketActorAddr, builtin.MethodsMarket.AddBalance, specsbig.Zero(), &mIDAddr, &g.chainRand)
	if err != nil {
		return nil, err
	}

	// Add all deals to chain in one message
	params := &market.PublishStorageDealsParams{}
	for _, comm := range comms {
		proposal := market.DealProposal{
			PieceCID:             comm.DealCfg.CommP,
			PieceSize:            abi.PaddedPieceSize(comm.DealCfg.PieceSize),
			Client:               workerID,
			Provider:             mIDAddr,
			StartEpoch:           0,
			EndEpoch:             abi.ChainEpoch(comm.DealCfg.EndEpoch), // TODO min and max epoch bounds
			StoragePricePerEpoch: specsbig.Zero(),
			ProviderCollateral:   specsbig.Zero(), // collateral should actually be good
			ClientCollateral:     specsbig.Zero(),
		}
		proposalBytes, err := encoding.Encode(&proposal)
		if err != nil {
			return nil, err
		}
		sig, err := crypto.Sign(proposalBytes, g.keys[m.Owner].PrivateKey, crypto.SigTypeBLS)
		if err != nil {
			return nil, err
		}

		params.Deals = append(params.Deals, market.ClientDealProposal{
			Proposal:        proposal,
			ClientSignature: sig,
		})
	}

	// apply deal builtin.MethodsMarket.PublishStorageDeals
	out, err := g.vm.ApplyGenesisMessage(addr, builtin.StorageMarketActorAddr, builtin.MethodsMarket.PublishStorageDeals, specsbig.Zero(), params, &g.chainRand)
	if err != nil {
		return nil, err
	}

	ret := out.(*market.PublishStorageDealsReturn)

	return ret.IDs, nil
}

func (g *GenesisGenerator) setupPost(ctx context.Context, addr, mIDAddr address.Address, provingPeriodStart *abi.ChainEpoch) error {
	mAct, found, err := g.stateTree.GetActor(ctx, mIDAddr)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("state tree could not find power actor")
	}
	var minerActorState miner.State
	_, err = g.store.Get(ctx, mAct.Head.Cid, &minerActorState)
	if err != nil {
		return err
	}

	// set provingperiod start state field
	ppStart := abi.ChainEpoch(miner.ProvingPeriod)
	if provingPeriodStart != nil {
		ppStart = *provingPeriodStart
	}
	minerActorState.PoStState.ProvingPeriodStart = ppStart

	// register event on cron actor by sending from miner actor address
	// Dragons need to include empty val to prevent cbor gen from panicing
	provingPeriodEvent := &miner.CronEventPayload{
		EventType: miner.CronEventWindowedPoStExpiration,
	}
	provingPeriodEventPayload, err := encoding.Encode(provingPeriodEvent)
	if err != nil {
		return err
	}
	params := &power.EnrollCronEventParams{
		EventEpoch: miner.ProvingPeriod,
		Payload:    provingPeriodEventPayload,
	}
	_, err = g.vm.ApplyGenesisMessage(mIDAddr, builtin.StoragePowerActorAddr, builtin.MethodsPower.EnrollCronEvent, specsbig.Zero(), params, &g.chainRand)
	if err != nil {
		return err
	}
	// Write miner actor
	newMinerCid, _, err := g.store.Put(ctx, &minerActorState)
	if err != nil {
		return err
	}
	mAct.Head = e.NewCid(newMinerCid)
	err = g.stateTree.SetActor(ctx, mIDAddr, mAct)
	if err != nil {
		return err
	}
	return nil
}

func (g *GenesisGenerator) getDealWeight(dealID abi.DealID, endEpoch abi.ChainEpoch, minerIDAddr address.Address) (specsbig.Int, error) {
	weightParams := &market.VerifyDealsOnSectorProveCommitParams{
		DealIDs:      []abi.DealID{dealID},
		SectorExpiry: endEpoch,
	}

	weightOut, err := g.vm.ApplyGenesisMessage(minerIDAddr, builtin.StorageMarketActorAddr, builtin.MethodsMarket.VerifyDealsOnSectorProveCommit, specsbig.Zero(), weightParams, &g.chainRand)
	if err != nil {
		return specsbig.Zero(), err
	}
	return *weightOut.(*specsbig.Int), nil
}

func (g *GenesisGenerator) updatePower(ctx context.Context, dealWeight specsbig.Int, sectorSize abi.SectorSize, endEpoch abi.ChainEpoch, minerIDAddr address.Address) (specsbig.Int, error) {
	powAct, found, err := g.stateTree.GetActor(ctx, builtin.StoragePowerActorAddr)
	if err != nil {
		return specsbig.Zero(), err
	}
	if !found {
		return specsbig.Zero(), fmt.Errorf("state tree could not find power actor")
	}
	var powerActorState power.State
	_, err = g.store.Get(ctx, powAct.Head.Cid, &powerActorState)
	if err != nil {
		return specsbig.Zero(), err
	}
	weight := &power.SectorStorageWeightDesc{
		SectorSize: sectorSize,
		Duration:   endEpoch,
		DealWeight: dealWeight,
	}
	spower := power.ConsensusPowerForWeight(weight)
	if !spower.Equals(powerActorState.TotalNetworkPower) { // avoid div by 0 with 1 miner and 1 sector
		spower = specsbig.Sub(powerActorState.TotalNetworkPower, spower)
	}
	pledge := power.PledgeForWeight(weight, spower)
	err = powerActorState.AddToClaim(&cstore{ctx, g.cst}, minerIDAddr, spower, pledge)
	if err != nil {
		return specsbig.Zero(), err
	}
	newPowCid, _, err := g.store.Put(ctx, &powerActorState)
	if err != nil {
		return specsbig.Zero(), err
	}
	powAct.Head = e.NewCid(newPowCid)
	err = g.stateTree.SetActor(ctx, builtin.StoragePowerActorAddr, powAct)
	if err != nil {
		return specsbig.Zero(), err
	}

	return pledge, nil
}

func (g *GenesisGenerator) putSectors(ctx context.Context, comm *CommitConfig, mIDAddr address.Address, dealID abi.DealID, dealWeight, pledge specsbig.Int) error {
	mAct, found, err := g.stateTree.GetActor(ctx, mIDAddr)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("state tree could not find power actor")
	}
	var minerActorState miner.State
	_, err = g.store.Get(ctx, mAct.Head.Cid, &minerActorState)
	if err != nil {
		return err
	}

	newSectorInfo := &miner.SectorOnChainInfo{
		Info: miner.SectorPreCommitInfo{
			RegisteredProof: abi.RegisteredProof_StackedDRG2KiBSeal, // default to 2kib, TODO set based on sector size
			SectorNumber:    abi.SectorNumber(comm.SectorNum),
			SealedCID:       comm.CommR,
			SealRandEpoch:   0,
			DealIDs:         []abi.DealID{dealID},
			Expiration:      abi.ChainEpoch(comm.DealCfg.EndEpoch),
		},
		ActivationEpoch:       0,
		DealWeight:            dealWeight,
		PledgeRequirement:     pledge,
		DeclaredFaultEpoch:    -1,
		DeclaredFaultDuration: -1,
	}
	err = minerActorState.PutSector(&cstore{ctx, g.cst}, newSectorInfo)
	if err != nil {
		return err
	}
	minerActorState.ProvingSet = minerActorState.Sectors
	// Write miner actor
	newMinerCid, _, err := g.store.Put(ctx, &minerActorState)
	if err != nil {
		return err
	}
	mAct.Head = e.NewCid(newMinerCid)
	err = g.stateTree.SetActor(ctx, mIDAddr, mAct)
	if err != nil {
		return err
	}

	// register sector expiry on cron
	sectorBf := abi.NewBitField()
	sectorBf.Set(comm.SectorNum)

	sectorExpiryEvent := &miner.CronEventPayload{
		EventType: miner.CronEventSectorExpiry,
		Sectors:   &sectorBf,
	}
	sectorExpiryEventPayload, err := encoding.Encode(sectorExpiryEvent)
	if err != nil {
		return err
	}
	params := &power.EnrollCronEventParams{
		EventEpoch: abi.ChainEpoch(comm.DealCfg.EndEpoch),
		Payload:    sectorExpiryEventPayload,
	}
	_, err = g.vm.ApplyGenesisMessage(mIDAddr, builtin.StoragePowerActorAddr, builtin.MethodsPower.EnrollCronEvent, specsbig.Zero(), params, &g.chainRand)
	return err
}

func (g *GenesisGenerator) getPower(ctx context.Context, mIDAddr address.Address) (abi.StoragePower, error) {
	stateRoot, err := g.flush(ctx)
	if err != nil {
		return specsbig.Zero(), err
	}
	view := gfcstate.NewView(g.cst, stateRoot)
	return view.MinerClaimedPower(ctx, mIDAddr)
}
