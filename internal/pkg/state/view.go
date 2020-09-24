package state

import (
	"context"
	"fmt"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-state-types/dline"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/lotus/build"
	"github.com/filecoin-project/specs-actors/actors/builtin/multisig"
	"github.com/filecoin-project/specs-actors/actors/builtin/reward"
	"github.com/filecoin-project/specs-actors/actors/builtin/verifreg"
	"github.com/filecoin-project/specs-actors/actors/runtime/proof"
	xerrors "github.com/pkg/errors"
	"github.com/prometheus/common/log"
	cbg "github.com/whyrusleeping/cbor-gen"
	"sync"

	"github.com/filecoin-project/go-bitfield"
	"github.com/filecoin-project/go-filecoin/vendors/sector-storage/ffiwrapper"

	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/builtin/account"
	notinit "github.com/filecoin-project/specs-actors/actors/builtin/init"
	"github.com/filecoin-project/specs-actors/actors/builtin/market"
	"github.com/filecoin-project/specs-actors/actors/builtin/miner"
	paychActor "github.com/filecoin-project/specs-actors/actors/builtin/paych"
	"github.com/filecoin-project/specs-actors/actors/builtin/power"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	vmstate "github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

var dealProviderCollateralNum = big.NewInt(110)
var dealProviderCollateralDen = big.NewInt(100)

// Viewer builds state views from state root CIDs.
type Viewer struct {
	ipldStore cbor.IpldStore
}

// NewViewer creates a new state
func NewViewer(store cbor.IpldStore) *Viewer {
	return &Viewer{store}
}

// StateView returns a new state view.
func (c *Viewer) StateView(root cid.Cid) *View {
	return NewView(c.ipldStore, root)
}

type genesisInfo struct {
	genesisMsigs []multisig.State
	// info about the Accounts in the genesis state
	genesisActors      []genesisActor
	genesisPledge      abi.TokenAmount
	genesisMarketFunds abi.TokenAmount
}

type genesisActor struct {
	addr    addr.Address
	initBal abi.TokenAmount
}

// View is a read-only interface to a snapshot of application-level actor state.
// This object interprets the actor state, abstracting the concrete on-chain structures so as to
// hide the complications of protocol versions.
// Exported methods on this type avoid exposing concrete state structures (which may be subject to versioning)
// where possible.
type View struct {
	ipldStore cbor.IpldStore
	root      cid.Cid

	genInfo       *genesisInfo
	genesisMsigLk sync.Mutex
	genesisRoot   cid.Cid
}

// NewView creates a new state view
func NewView(store cbor.IpldStore, root cid.Cid) *View {
	return &View{
		ipldStore: store,
		root:      root,
	}
}

// InitNetworkName Returns the network name from the init actor state.
func (v *View) InitNetworkName(ctx context.Context) (string, error) {
	initState, err := v.loadInitActor(ctx)
	if err != nil {
		return "", err
	}
	return initState.NetworkName, nil
}

// InitResolveAddress Returns ID address if public key address is given.
func (v *View) InitResolveAddress(ctx context.Context, a addr.Address) (addr.Address, error) {
	if a.Protocol() == addr.ID {
		return a, nil
	}

	initState, err := v.loadInitActor(ctx)
	if err != nil {
		return addr.Undef, err
	}

	state := &notinit.State{
		AddressMap: initState.AddressMap,
	}
	rAddr, _, err := state.ResolveAddress(v.adtStore(ctx), a) //todo add by force bool?
	if err != nil {
		return addr.Undef, err
	}
	return rAddr, nil
}

// Returns public key address if id address is given
func (v *View) AccountSignerAddress(ctx context.Context, a addr.Address) (addr.Address, error) {
	if a.Protocol() == addr.SECP256K1 || a.Protocol() == addr.BLS {
		return a, nil
	}

	accountActorState, err := v.loadAccountActor(ctx, a)
	if err != nil {
		return addr.Undef, err
	}

	return accountActorState.Address, nil
}

// MinerControlAddresses returns the owner and worker addresses for a miner actor
func (v *View) MinerControlAddresses(ctx context.Context, maddr addr.Address) (owner, worker addr.Address, err error) {
	minerInfo, err := v.MinerInfo(ctx, maddr)
	if err != nil {
		return addr.Undef, addr.Undef, err
	}
	return minerInfo.Owner, minerInfo.Worker, nil
}

func (v *View) MinerInfo(ctx context.Context, maddr addr.Address) (*miner.MinerInfo, error) {
	minerState, err := v.loadMinerActor(ctx, maddr)
	if err != nil {
		return nil, err
	}
	return minerState.GetInfo(v.adtStore(ctx))
}

// MinerPeerID returns the PeerID for a miner actor
func (v *View) MinerPeerID(ctx context.Context, maddr addr.Address) (peer.ID, error) {
	minerInfo, err := v.MinerInfo(ctx, maddr)
	if err != nil {
		return "", err
	}
	return peer.ID(minerInfo.PeerId), nil
}

type MinerSectorConfiguration struct {
	SealProofType              abi.RegisteredSealProof
	SectorSize                 abi.SectorSize
	WindowPoStPartitionSectors uint64
}

// MinerSectorConfiguration returns the sector size for a miner actor
func (v *View) MinerSectorConfiguration(ctx context.Context, maddr addr.Address) (*MinerSectorConfiguration, error) {
	minerInfo, err := v.MinerInfo(ctx, maddr)
	if err != nil {
		return nil, err
	}
	return &MinerSectorConfiguration{
		SealProofType:              minerInfo.SealProofType,
		SectorSize:                 minerInfo.SectorSize,
		WindowPoStPartitionSectors: minerInfo.WindowPoStPartitionSectors,
	}, nil
}

// MinerSectorCount counts all the on-chain sectors
func (v *View) MinerSectorCount(ctx context.Context, maddr addr.Address) (uint64, error) {
	minerState, err := v.loadMinerActor(ctx, maddr)
	if err != nil {
		return 0, err
	}
	sectors, err := v.asArray(ctx, minerState.Sectors)
	if err != nil {
		return 0, err
	}
	length := sectors.Length()
	return length, nil
}

// Loads sector info from miner state.
func (v *View) MinerGetSector(ctx context.Context, maddr addr.Address, sectorNum abi.SectorNumber) (*miner.SectorOnChainInfo, bool, error) {
	minerState, err := v.loadMinerActor(ctx, maddr)
	if err != nil {
		return nil, false, err
	}
	return minerState.GetSector(v.adtStore(ctx), sectorNum)
}

// MinerDeadlineInfo returns information relevant to the current proving deadline
func (v *View) MinerDeadlineInfo(ctx context.Context, maddr addr.Address, epoch abi.ChainEpoch) (index uint64, open, close, challenge abi.ChainEpoch, _ error) {
	minerState, err := v.loadMinerActor(ctx, maddr)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	deadlineInfo := minerState.DeadlineInfo(epoch)
	return deadlineInfo.Index, deadlineInfo.Open, deadlineInfo.Close, deadlineInfo.Challenge, nil
}

// MinerSuccessfulPoSts counts how many successful window PoSts have been made this proving period so far.
func (v *View) MinerSuccessfulPoSts(ctx context.Context, maddr addr.Address) (uint64, error) {
	minerState, err := v.loadMinerActor(ctx, maddr)
	if err != nil {
		return 0, err
	}

	deadlines, err := minerState.LoadDeadlines(v.adtStore(ctx))
	if err != nil {
		return 0, err
	}

	count := uint64(0)
	err = deadlines.ForEach(v.adtStore(ctx), func(dlIdx uint64, dl *miner.Deadline) error {
		dCount, err := dl.PostSubmissions.Count()
		if err != nil {
			return err
		}
		count += dCount
		return nil
	})
	if err != nil {
		return 0, err
	}

	return count, nil
}

// MinerDeadlines returns a bitfield of sectors in a proving period
// NOTE: exposes on-chain structures directly because it's referenced directly by the storage-fsm module.
// This is in conflict with the general goal of the state view of hiding the chain state representations from
// consumers in order to support versioning that representation through protocol upgrades.
// See https://github.com/filecoin-project/go-filecoin/vendors/storage-sealing/issues/13
func (v *View) MinerDeadlines(ctx context.Context, maddr addr.Address) (*miner.Deadlines, error) {
	minerState, err := v.loadMinerActor(ctx, maddr)
	if err != nil {
		return nil, err
	}

	return minerState.LoadDeadlines(v.adtStore(ctx))
}

func (v *View) MinerProvingPeriodStart(ctx context.Context, maddr addr.Address) (abi.ChainEpoch, error) {
	minerState, err := v.loadMinerActor(ctx, maddr)
	if err != nil {
		return 0, err
	}
	return minerState.ProvingPeriodStart, nil
}

// MinerSectorsForEach Iterates over the sectors in a miner's proving set.
func (v *View) MinerSectorsForEach(ctx context.Context, maddr addr.Address,
	f func(abi.SectorNumber, cid.Cid, abi.RegisteredSealProof, []abi.DealID) error) error {
	minerState, err := v.loadMinerActor(ctx, maddr)
	if err != nil {
		return err
	}

	sectors, err := v.asArray(ctx, minerState.Sectors)
	if err != nil {
		return err
	}

	// This version for the new actors
	var sector miner.SectorOnChainInfo
	return sectors.ForEach(&sector, func(secnum int64) error {
		// Add more fields here as required by new callers.
		return f(sector.SectorNumber, sector.SealedCID, sector.SealProof, sector.DealIDs)
	})
}

// MinerExists Returns true iff the miner exists.
func (v *View) MinerExists(ctx context.Context, maddr addr.Address) (bool, error) {
	_, err := v.loadMinerActor(ctx, maddr)
	if err == nil {
		return true, nil
	}
	if err == types.ErrNotFound {
		return false, nil
	}
	return false, err
}

// MinerFaults Returns all sector ids that are faults
func (v *View) MinerFaults(ctx context.Context, maddr addr.Address) ([]uint64, error) {
	minerState, err := v.loadMinerActor(ctx, maddr)
	if err != nil {
		return nil, err
	}
	out := bitfield.New()

	deallines, err := minerState.LoadDeadlines(v.adtStore(ctx))
	if err != nil {
		return nil, err
	}

	err = deallines.ForEach(v.adtStore(ctx), func(dlIdx uint64, dl *miner.Deadline) error {
		partitions, err := dl.PartitionsArray(v.adtStore(ctx))
		if err != nil {
			return err
		}

		var partition miner.Partition
		return partitions.ForEach(&partition, func(i int64) error {
			out, err = bitfield.MergeBitFields(out, partition.Faults)
			return err
		})
	})

	maxSectorNum, err := out.All(miner.SectorsMax)
	if err != nil {
		return nil, err
	}
	return maxSectorNum, nil
}

// MinerGetPrecommittedSector Looks up info for a miners precommitted sector.
// NOTE: exposes on-chain structures directly for storage FSM API.
func (v *View) MinerGetPrecommittedSector(ctx context.Context, maddr addr.Address, sectorNum abi.SectorNumber) (*miner.SectorPreCommitOnChainInfo, bool, error) {
	minerState, err := v.loadMinerActor(ctx, maddr)
	if err != nil {
		return nil, false, err
	}

	return minerState.GetPrecommittedSector(v.adtStore(ctx), sectorNum)
}

// MarketEscrowBalance looks up a token amount in the escrow table for the given address
func (v *View) MarketEscrowBalance(ctx context.Context, addr addr.Address) (found bool, amount abi.TokenAmount, err error) {
	marketState, err := v.loadMarketActor(ctx)
	if err != nil {
		return false, abi.NewTokenAmount(0), err
	}

	escrow, err := v.asMap(ctx, marketState.EscrowTable)
	if err != nil {
		return false, abi.NewTokenAmount(0), err
	}

	var value abi.TokenAmount
	found, err = escrow.Get(abi.AddrKey(addr), &value)
	return
}

// MarketComputeDataCommitment takes deal ids and uses associated commPs to compute commD for a sector that contains the deals
func (v *View) MarketComputeDataCommitment(ctx context.Context, registeredProof abi.RegisteredSealProof, dealIDs []abi.DealID) (cid.Cid, error) {
	marketState, err := v.loadMarketActor(ctx)
	if err != nil {
		return cid.Undef, err
	}

	deals, err := v.asArray(ctx, marketState.Proposals)
	if err != nil {
		return cid.Undef, err
	}

	// map deals to pieceInfo
	pieceInfos := make([]abi.PieceInfo, len(dealIDs))
	for i, id := range dealIDs {
		var proposal market.DealProposal
		found, err := deals.Get(uint64(id), &proposal)
		if err != nil {
			return cid.Undef, err
		}

		if !found {
			return cid.Undef, fmt.Errorf("Could not find deal id %d", id)
		}

		pieceInfos[i].PieceCID = proposal.PieceCID
		pieceInfos[i].Size = proposal.PieceSize
	}

	return ffiwrapper.GenerateUnsealedCID(registeredProof, pieceInfos)
}

// NOTE: exposes on-chain structures directly for storage FSM interface.
func (v *View) MarketDealProposal(ctx context.Context, dealID abi.DealID) (market.DealProposal, error) {
	marketState, err := v.loadMarketActor(ctx)
	if err != nil {
		return market.DealProposal{}, err
	}

	deals, err := v.asArray(ctx, marketState.Proposals)
	if err != nil {
		return market.DealProposal{}, err
	}

	var proposal market.DealProposal
	found, err := deals.Get(uint64(dealID), &proposal)
	if err != nil {
		return market.DealProposal{}, err
	}
	if !found {
		return market.DealProposal{}, fmt.Errorf("Could not find deal id %d", dealID)
	}

	return proposal, nil
}

// NOTE: exposes on-chain structures directly for storage FSM and market module interfaces.
func (v *View) MarketDealState(ctx context.Context, dealID abi.DealID) (*market.DealState, bool, error) {
	marketState, err := v.loadMarketActor(ctx)
	if err != nil {
		return nil, false, err
	}

	dealStates, err := v.asDealStateArray(ctx, marketState.States)
	if err != nil {
		return nil, false, err
	}
	return dealStates.Get(dealID)
}

// NOTE: exposes on-chain structures directly for market interface.
// The callback receives a pointer to a transient object; take a copy or drop the reference outside the callback.
func (v *View) MarketDealStatesForEach(ctx context.Context, f func(id abi.DealID, state *market.DealState) error) error {
	marketState, err := v.loadMarketActor(ctx)
	if err != nil {
		return err
	}

	dealStates, err := v.asDealStateArray(ctx, marketState.States)
	if err != nil {
		return err
	}

	var ds market.DealState
	return dealStates.ForEach(&ds, func(dealId int64) error {
		return f(abi.DealID(dealId), &ds)
	})
}

// StateDealProviderCollateralBounds returns the min and max collateral a storage provider
// can issue. It takes the deal size and verified status as parameters.
func (v *View) MarketDealProviderCollateralBounds(ctx context.Context, size abi.PaddedPieceSize, verified bool, height abi.ChainEpoch) (DealCollateralBounds, error) {
	powerState, err := v.loadPowerActor(ctx)
	if err != nil {
		return DealCollateralBounds{}, xerrors.Errorf("getting power state: %w", err)
	}
	rewardState, err := v.loadRewardActor(ctx)
	if err != nil {
		return DealCollateralBounds{}, xerrors.Errorf("getting reward state: %w", err)
	}

	tree, err := vmstate.LoadState(ctx, v.ipldStore, v.root)
	if err != nil {
		return DealCollateralBounds{}, xerrors.Errorf("failed to get tree: %w", err)
	}

	circ, err := v.GetCirculatingSupplyDetailed(ctx, height, tree)
	if err != nil {
		return DealCollateralBounds{}, xerrors.Errorf("getting total circulating supply: %w", err)
	}

	min, max := market.DealProviderCollateralBounds(size,
		verified,
		powerState.TotalRawBytePower,
		powerState.ThisEpochQualityAdjPower,
		rewardState.ThisEpochBaselinePower,
		circ.FilCirculating,
		v.GetNtwkVersion(ctx, height))
	return DealCollateralBounds{
		Min: big.Div(big.Mul(min, dealProviderCollateralNum), dealProviderCollateralDen),
		Max: max,
	}, nil
}

func (v *View) StateVerifiedClientStatus(ctx context.Context, addr addr.Address) (*verifreg.DataCap, error) {
	aid, err := v.StateLookupID(ctx, addr)
	if err != nil {
		log.Warnf("lookup failure %v", err)
		return nil, err
	}

	st, err := v.loadVerifyActor(ctx)

	vh, err := adt.AsMap(v.adtStore(ctx), st.VerifiedClients)
	if err != nil {
		return nil, err
	}

	var dcap verifreg.DataCap
	if found, err := vh.Get(abi.AddrKey(aid), &dcap); err != nil {
		return nil, err
	} else if !found {
		return nil, nil
	}

	return &dcap, nil
}

func (v *View) StateMarketStorageDeal(ctx context.Context, dealID abi.DealID) (*MarketDeal, error) {
	state, err := v.loadMarketActor(ctx)
	if err != nil {
		return nil, err
	}
	store := v.adtStore(ctx)

	da, err := adt.AsArray(store, state.Proposals)
	if err != nil {
		return nil, err
	}

	var dp market.DealProposal
	if found, err := da.Get(uint64(dealID), &dp); err != nil {
		return nil, err
	} else if !found {
		return nil, xerrors.Errorf("deal %d not found", dealID)
	}

	sa, err := market.AsDealStateArray(store, state.States)
	if err != nil {
		return nil, err
	}

	st, found, err := sa.Get(dealID)
	if err != nil {
		return nil, err
	}

	if !found {
		st = &market.DealState{
			SectorStartEpoch: -1,
			LastUpdatedEpoch: -1,
			SlashEpoch:       -1,
		}
	}

	return &MarketDeal{
		Proposal: dp,
		State:    *st,
	}, nil
}

func (v *View) GetCirculatingSupply(ctx context.Context, height abi.ChainEpoch, st vmstate.Tree) (abi.TokenAmount, error) {
	csi, err := v.GetCirculatingSupplyDetailed(ctx, height, st)
	if err != nil {
		return big.Zero(), err
	}

	return csi.FilCirculating, nil
}

func (v *View) GetCirculatingSupplyDetailed(ctx context.Context, height abi.ChainEpoch, st vmstate.Tree) (CirculatingSupply, error) {
	v.genesisMsigLk.Lock()
	defer v.genesisMsigLk.Unlock()
	if v.genInfo == nil {
		err := v.setupGenesisActorsTestnet(ctx)
		if err != nil {
			return CirculatingSupply{}, xerrors.Errorf("failed to setup genesis information: %w", err)
		}
	}

	filVested, err := v.GetFilVested(ctx, height, st)
	if err != nil {
		return CirculatingSupply{}, xerrors.Errorf("failed to calculate filVested: %w", err)
	}

	rewardState, err := v.loadRewardActor(ctx)
	if err != nil {
		return CirculatingSupply{}, xerrors.Errorf("failed to calculate filMined: %w", err)
	}

	filBurnt, err := v.loadActor(ctx, builtin.BurntFundsActorAddr)
	if err != nil {
		return CirculatingSupply{}, xerrors.Errorf("failed to calculate filBurnt: %w", err)
	}

	filLocked, err := v.GetFilLocked(ctx, st)
	if err != nil {
		return CirculatingSupply{}, xerrors.Errorf("failed to calculate filLocked: %w", err)
	}

	ret := big.Add(filVested, rewardState.TotalMined)
	ret = big.Sub(ret, filBurnt.Balance)
	ret = big.Sub(ret, filLocked)

	if ret.LessThan(big.Zero()) {
		ret = big.Zero()
	}

	return CirculatingSupply{
		FilVested:      filVested,
		FilMined:       rewardState.TotalMined,
		FilBurnt:       filBurnt.Balance,
		FilLocked:      filLocked,
		FilCirculating: ret,
	}, nil
}

func (v *View) GetFilVested(ctx context.Context, height abi.ChainEpoch, st vmstate.Tree) (abi.TokenAmount, error) {
	vf := big.Zero()
	for _, v := range v.genInfo.genesisMsigs {
		au := big.Sub(v.InitialBalance, v.AmountLocked(height))
		vf = big.Add(vf, au)
	}

	// there should not be any such accounts in testnet (and also none in mainnet?)
	for _, v := range v.genInfo.genesisActors {
		act, found, err := st.GetActor(ctx, v.addr)
		if !found || err != nil {
			return big.Zero(), xerrors.Errorf("failed to get actor: %w", err)
		}

		diff := big.Sub(v.initBal, act.Balance)
		if diff.GreaterThan(big.Zero()) {
			vf = big.Add(vf, diff)
		}
	}

	vf = big.Add(vf, v.genInfo.genesisPledge)
	vf = big.Add(vf, v.genInfo.genesisMarketFunds)

	return vf, nil
}

// sets up information about the actors in the genesis state
// For testnet we use a hardcoded set of multisig states, instead of what's actually in the genesis multisigs
// We also do not consider ANY account actors (including the faucet)
func (v *View) setupGenesisActorsTestnet(ctx context.Context) error {

	gi := genesisInfo{}

	sTree, err := vmstate.LoadState(ctx, v.ipldStore, v.genesisRoot)
	if err != nil {
		return xerrors.Errorf("loading state tree: %w", err)
	}

	gi.genesisMarketFunds, err = getFilMarketLocked(ctx, v.ipldStore, sTree)
	if err != nil {
		return xerrors.Errorf("setting up genesis market funds: %w", err)
	}

	gi.genesisPledge, err = getFilPowerLocked(ctx, v.ipldStore, sTree)
	if err != nil {
		return xerrors.Errorf("setting up genesis pledge: %w", err)
	}

	totalsByEpoch := make(map[abi.ChainEpoch]abi.TokenAmount)

	// 6 months
	sixMonths := abi.ChainEpoch(183 * builtin.EpochsInDay)
	totalsByEpoch[sixMonths] = big.NewInt(49_929_341)
	totalsByEpoch[sixMonths] = big.Add(totalsByEpoch[sixMonths], big.NewInt(32_787_700))

	// 1 year
	oneYear := abi.ChainEpoch(365 * builtin.EpochsInDay)
	totalsByEpoch[oneYear] = big.NewInt(22_421_712)

	// 2 years
	twoYears := abi.ChainEpoch(2 * 365 * builtin.EpochsInDay)
	totalsByEpoch[twoYears] = big.NewInt(7_223_364)

	// 3 years
	threeYears := abi.ChainEpoch(3 * 365 * builtin.EpochsInDay)
	totalsByEpoch[threeYears] = big.NewInt(87_637_883)

	// 6 years
	sixYears := abi.ChainEpoch(6 * 365 * builtin.EpochsInDay)
	totalsByEpoch[sixYears] = big.NewInt(100_000_000)
	totalsByEpoch[sixYears] = big.Add(totalsByEpoch[sixYears], big.NewInt(300_000_000))

	gi.genesisMsigs = make([]multisig.State, 0, len(totalsByEpoch))
	for k, v := range totalsByEpoch {
		ns := multisig.State{
			InitialBalance: v,
			UnlockDuration: k,
			PendingTxns:    cid.Undef,
		}
		gi.genesisMsigs = append(gi.genesisMsigs, ns)
	}

	v.genInfo = &gi

	return nil
}

func (v *View) GetNtwkVersion(ctx context.Context, height abi.ChainEpoch) network.Version {
	if build.UseNewestNetwork() {
		return build.NewestNetworkVersion
	}

	if height <= build.UpgradeBreezeHeight {
		return network.Version0
	}

	if height <= build.UpgradeSmokeHeight {
		return network.Version1
	}

	return build.NewestNetworkVersion
}

// Returns the storage power actor's values for network total power.
func (v *View) PowerNetworkTotal(ctx context.Context) (*NetworkPower, error) {
	st, err := v.loadPowerActor(ctx)
	if err != nil {
		return nil, err
	}

	return &NetworkPower{
		RawBytePower:         st.TotalRawBytePower,
		QualityAdjustedPower: st.TotalQualityAdjPower,
		MinerCount:           st.MinerCount,
		MinPowerMinerCount:   st.MinerAboveMinPowerCount,
	}, nil
}

// Returns the power of a miner's committed sectors.
func (v *View) MinerClaimedPower(ctx context.Context, miner addr.Address) (raw, qa abi.StoragePower, err error) {
	minerResolved, err := v.InitResolveAddress(ctx, miner)
	if err != nil {
		return big.Zero(), big.Zero(), err
	}
	powerState, err := v.loadPowerActor(ctx)
	if err != nil {
		return big.Zero(), big.Zero(), err
	}
	claim, err := v.loadPowerClaim(ctx, powerState, minerResolved)
	if err != nil {
		return big.Zero(), big.Zero(), err
	}
	return claim.RawBytePower, claim.QualityAdjPower, nil
}

// PaychActorParties returns the From and To addresses for the given payment channel
func (v *View) PaychActorParties(ctx context.Context, paychAddr addr.Address) (from, to addr.Address, err error) {
	a, err := v.loadActor(ctx, paychAddr)
	if err != nil {
		return addr.Undef, addr.Undef, err
	}
	var state paychActor.State
	err = v.ipldStore.Get(ctx, a.Head, &state)
	if err != nil {
		return addr.Undef, addr.Undef, err
	}
	return state.From, state.To, nil
}

func (v *View) StateMinerProvingDeadline(ctx context.Context, addr addr.Address, ts block.TipSet) (*dline.Info, error) {
	mas, err := v.loadMinerActor(ctx, addr)
	if err != nil {
		return nil, xerrors.WithMessage(err, "failed to get proving dealline")
	}
	height, _ := ts.Height()
	return mas.DeadlineInfo(height).NextNotElapsed(), nil
}

func (v *View) StateMinerPartitions(ctx context.Context, addr addr.Address, dlIdx uint64, key block.TipSetKey) ([]*miner.Partition, error) {
	mas, err := v.loadMinerActor(ctx, addr)
	if err != nil {
		return nil, xerrors.WithMessage(err, "failed to get proving dealline")
	}

	adtStore := v.adtStore(ctx)
	dealLines, err := mas.LoadDeadlines(adtStore)
	if err != nil {
		return nil, xerrors.WithMessage(err, "failed to get proving dealline")
	}
	dealLine, err := dealLines.LoadDeadline(adtStore, dlIdx)
	if err != nil {
		return nil, xerrors.WithMessage(err, "failed to get proving dealline")
	}
	partitionsArr, err := dealLine.PartitionsArray(adtStore)
	if err != nil {
		return nil, xerrors.WithMessage(err, "failed to get proving dealline")
	}
	var partitions []*miner.Partition
	var partition miner.Partition
	err = partitionsArr.ForEach(&partition, func(i int64) error {
		partitionCopy := partition
		partitions = append(partitions, &partitionCopy)
		return nil
	})
	if err != nil {
		return nil, xerrors.WithMessage(err, "failed to get proving dealline")
	}
	return partitions, nil
}

func (v *View) StateMinerSectors(ctx context.Context, addr addr.Address, filter *bitfield.BitField, filterOut bool, key block.TipSetKey) ([]*ChainSectorInfo, error) {
	mas, err := v.loadMinerActor(ctx, addr)
	if err != nil {
		return nil, xerrors.WithMessage(err, "failed to get proving dealline")
	}
	return loadSectorsFromSet(v.adtStore(ctx), mas.Sectors, filter, filterOut)
}

func (v *View) StateLookupID(ctx context.Context, address addr.Address) (addr.Address, error) {
	tree, err := vmstate.LoadState(ctx, v.ipldStore, v.root)
	if err != nil {
		return addr.Undef, err
	}
	return tree.LookupID(address)
}

func (v *View) GetFilLocked(ctx context.Context, st vmstate.Tree) (abi.TokenAmount, error) {
	filMarketLocked, err := getFilMarketLocked(ctx, v.ipldStore, st)
	if err != nil {
		return big.Zero(), xerrors.Errorf("failed to get filMarketLocked: %w", err)
	}

	pwrState, err := v.loadPowerActor(ctx)
	if err != nil {
		return big.Zero(), xerrors.Errorf("failed to get filPowerLocked: %w", err)
	}

	return big.Add(filMarketLocked, pwrState.TotalPledgeCollateral), nil
}

///// utils

func (v *View) loadPowerClaim(ctx context.Context, powerState *power.State, miner addr.Address) (*power.Claim, error) {
	claims, err := v.asMap(ctx, powerState.Claims)
	if err != nil {
		return nil, err
	}

	var claim power.Claim
	found, err := claims.Get(abi.AddrKey(miner), &claim)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, types.ErrNotFound
	}
	return &claim, nil
}

func (v *View) loadInitActor(ctx context.Context) (*notinit.State, error) {
	actr, err := v.loadActor(ctx, builtin.InitActorAddr)
	if err != nil {
		return nil, err
	}
	var state notinit.State
	err = v.ipldStore.Get(ctx, actr.Head, &state)
	return &state, err
}

func (v *View) loadMinerActor(ctx context.Context, address addr.Address) (*miner.State, error) {
	resolvedAddr, err := v.InitResolveAddress(ctx, address)
	if err != nil {
		return nil, err
	}
	actr, err := v.loadActor(ctx, resolvedAddr)
	if err != nil {
		return nil, err
	}
	var state miner.State
	err = v.ipldStore.Get(ctx, actr.Head, &state)
	return &state, err
}

func (v *View) loadPowerActor(ctx context.Context) (*power.State, error) {
	actr, err := v.loadActor(ctx, builtin.StoragePowerActorAddr)
	if err != nil {
		return nil, err
	}
	var state power.State
	err = v.ipldStore.Get(ctx, actr.Head, &state)
	return &state, err
}

func (v *View) loadVerifyActor(ctx context.Context) (*verifreg.State, error) {
	actr, err := v.loadActor(ctx, builtin.VerifiedRegistryActorAddr)
	if err != nil {
		return nil, err
	}
	var state verifreg.State
	err = v.ipldStore.Get(ctx, actr.Head, &state)
	return &state, err
}

func (v *View) loadRewardActor(ctx context.Context) (*reward.State, error) {
	actr, err := v.loadActor(ctx, builtin.RewardActorAddr)
	if err != nil {
		return nil, err
	}
	var state reward.State
	err = v.ipldStore.Get(ctx, actr.Head, &state)
	return &state, err
}

func (v *View) loadMarketActor(ctx context.Context) (*market.State, error) {
	actr, err := v.loadActor(ctx, builtin.StorageMarketActorAddr)
	if err != nil {
		return nil, err
	}
	var state market.State
	err = v.ipldStore.Get(ctx, actr.Head, &state)
	return &state, err
}

func (v *View) loadAccountActor(ctx context.Context, a addr.Address) (*account.State, error) {
	resolvedAddr, err := v.InitResolveAddress(ctx, a)
	if err != nil {
		return nil, err
	}
	actr, err := v.loadActor(ctx, resolvedAddr)
	if err != nil {
		return nil, err
	}
	var state account.State
	err = v.ipldStore.Get(ctx, actr.Head, &state)
	return &state, err
}

func (v *View) loadActor(ctx context.Context, address addr.Address) (*actor.Actor, error) {
	tree, err := v.asMap(ctx, v.root)
	if err != nil {
		return nil, err
	}

	var actr actor.Actor
	found, err := tree.Get(abi.AddrKey(address), &actr)
	if !found {
		return nil, types.ErrNotFound
	}

	return &actr, err
}

func (v *View) adtStore(ctx context.Context) adt.Store {
	return StoreFromCbor(ctx, v.ipldStore)
}

func (v *View) asArray(ctx context.Context, root cid.Cid) (*adt.Array, error) {
	return adt.AsArray(v.adtStore(ctx), root)
}

func (v *View) asMap(ctx context.Context, root cid.Cid) (*adt.Map, error) {
	return adt.AsMap(v.adtStore(ctx), root)
}

func (v *View) asDealStateArray(ctx context.Context, root cid.Cid) (*market.DealMetaArray, error) {
	return market.AsDealStateArray(v.adtStore(ctx), root)
}

func (v *View) asBalanceTable(ctx context.Context, root cid.Cid) (*adt.BalanceTable, error) {
	return adt.AsBalanceTable(v.adtStore(ctx), root)
}

func getFilPowerLocked(ctx context.Context, ipldStore cbor.IpldStore, st vmstate.Tree) (abi.TokenAmount, error) {
	pactor, found, err := st.GetActor(ctx, builtin.StoragePowerActorAddr)
	if !found || err != nil {
		return big.Zero(), xerrors.Errorf("failed to load power actor: %w", err)
	}

	var pst power.State
	if err := ipldStore.Get(ctx, pactor.Head, &pst); err != nil {
		return big.Zero(), xerrors.Errorf("failed to load power state: %w", err)
	}
	return pst.TotalPledgeCollateral, nil
}

func loadSectorsFromSet(adtStore adt.Store, ssc cid.Cid, filter *bitfield.BitField, filterOut bool) ([]*ChainSectorInfo, error) {
	a, err := adt.AsArray(adtStore, ssc)
	if err != nil {
		return nil, err
	}

	var sset []*ChainSectorInfo
	var v cbg.Deferred
	if err := a.ForEach(&v, func(i int64) error {
		if filter != nil {
			set, err := filter.IsSet(uint64(i))
			if err != nil {
				return xerrors.Errorf("filter check error: %w", err)
			}
			if set == filterOut {
				return nil
			}
		}

		var oci miner.SectorOnChainInfo
		if err := cbor.DecodeInto(v.Raw, &oci); err != nil {
			return err
		}
		sset = append(sset, &ChainSectorInfo{
			Info: oci,
			ID:   abi.SectorNumber(i),
		})
		return nil
	}); err != nil {
		return nil, err
	}

	return sset, nil
}

func getFilMarketLocked(ctx context.Context, ipldStore cbor.IpldStore, st vmstate.Tree) (abi.TokenAmount, error) {
	mactor, found, err := st.GetActor(ctx, builtin.StorageMarketActorAddr)
	if !found || err != nil {
		return big.Zero(), xerrors.Errorf("failed to load market actor: %w", err)
	}

	var mst market.State
	if err := ipldStore.Get(ctx, mactor.Head, &mst); err != nil {
		return big.Zero(), xerrors.Errorf("failed to load market state: %w", err)
	}

	fml := big.Add(mst.TotalClientLockedCollateral, mst.TotalProviderLockedCollateral)
	fml = big.Add(fml, mst.TotalClientStorageFee)
	return fml, nil
}

// StoreFromCbor wraps a cbor ipldStore for ADT access.
func StoreFromCbor(ctx context.Context, ipldStore cbor.IpldStore) adt.Store {
	return &cstore{ctx, ipldStore}
}

type cstore struct {
	ctx context.Context
	cbor.IpldStore
}

func (s *cstore) Context() context.Context {
	return s.ctx
}
