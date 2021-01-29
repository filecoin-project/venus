package miner

import (
	"bytes"
	"errors"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-bitfield"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/dline"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	cbg "github.com/whyrusleeping/cbor-gen"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus/pkg/specactors/adt"

	builtin3 "github.com/filecoin-project/specs-actors/v3/actors/builtin"
	miner3 "github.com/filecoin-project/specs-actors/v3/actors/builtin/miner"
	adt3 "github.com/filecoin-project/specs-actors/v3/actors/util/adt"
)

var _ State = (*state3)(nil)

func load3(store adt.Store, root cid.Cid) (State, error) {
	out := state3{store: store}
	err := store.Get(store.Context(), root, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

type state3 struct {
	miner3.State
	store adt.Store
}

type deadline3 struct {
	miner3.Deadline
	store adt.Store
}

type partition3 struct {
	miner3.Partition
	store adt.Store
}

func (s *state3) AvailableBalance(bal abi.TokenAmount) (available abi.TokenAmount, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = xerrors.Errorf("failed to get available balance: %w", r)
			available = abi.NewTokenAmount(0)
		}
	}()
	// this panics if the miner doesnt have enough funds to cover their locked pledge
	available, err = s.GetAvailableBalance(bal)
	return available, err
}

func (s *state3) VestedFunds(epoch abi.ChainEpoch) (abi.TokenAmount, error) {
	return s.CheckVestedFunds(s.store, epoch)
}

func (s *state3) LockedFunds() (LockedFunds, error) {
	return LockedFunds{
		VestingFunds:             s.State.LockedFunds,
		InitialPledgeRequirement: s.State.InitialPledge,
		PreCommitDeposits:        s.State.PreCommitDeposits,
	}, nil
}

func (s *state3) FeeDebt() (abi.TokenAmount, error) {
	return s.State.FeeDebt, nil
}

func (s *state3) InitialPledge() (abi.TokenAmount, error) {
	return s.State.InitialPledge, nil
}

func (s *state3) PreCommitDeposits() (abi.TokenAmount, error) {
	return s.State.PreCommitDeposits, nil
}

func (s *state3) GetSector(num abi.SectorNumber) (*SectorOnChainInfo, error) {
	info, ok, err := s.State.GetSector(s.store, num)
	if !ok || err != nil {
		return nil, err
	}

	ret := fromV3SectorOnChainInfo(*info)
	return &ret, nil
}

func (s *state3) FindSector(num abi.SectorNumber) (*SectorLocation, error) {
	dlIdx, partIdx, err := s.State.FindSector(s.store, num)
	if err != nil {
		return nil, err
	}
	return &SectorLocation{
		Deadline:  dlIdx,
		Partition: partIdx,
	}, nil
}

func (s *state3) NumLiveSectors() (uint64, error) {
	dls, err := s.State.LoadDeadlines(s.store)
	if err != nil {
		return 0, err
	}
	var total uint64
	if err := dls.ForEach(s.store, func(dlIdx uint64, dl *miner3.Deadline) error {
		total += dl.LiveSectors
		return nil
	}); err != nil {
		return 0, err
	}
	return total, nil
}

// todo review
func (s *state3) FaultsSectors() ([]uint64, error) {
	out := bitfield.New()

	dls, err := s.State.LoadDeadlines(s.store)
	if err != nil {
		return []uint64{}, err
	}

	if err := dls.ForEach(s.store, func(dlIdx uint64, dl *miner3.Deadline) error {
		partitions, err := dl.PartitionsArray(s.store)
		if err != nil {
			return err
		}

		var partition miner3.Partition
		return partitions.ForEach(&partition, func(i int64) error {
			out, err = bitfield.MergeBitFields(out, partition.Faults)
			return err
		})
	}); err != nil {
		return []uint64{}, err
	}

	return out.All(bitfield.MaxEncodedSize)
}

// GetSectorExpiration returns the effective expiration of the given sector.
//
// If the sector does not expire early, the Early expiration field is 0.
func (s *state3) GetSectorExpiration(num abi.SectorNumber) (*SectorExpiration, error) {
	dls, err := s.State.LoadDeadlines(s.store)
	if err != nil {
		return nil, err
	}
	// NOTE: this can be optimized significantly.
	// 1. If the sector is non-faulty, it will either expire on-time (can be
	// learned from the sector info), or in the next quantized expiration
	// epoch (i.e., the first element in the partition's expiration queue.
	// 2. If it's faulty, it will expire early within the first 14 entries
	// of the expiration queue.
	stopErr := errors.New("stop")
	out := SectorExpiration{}
	err = dls.ForEach(s.store, func(dlIdx uint64, dl *miner3.Deadline) error {
		partitions, err := dl.PartitionsArray(s.store)
		if err != nil {
			return err
		}
		quant := s.State.QuantSpecForDeadline(dlIdx)
		var part miner3.Partition
		return partitions.ForEach(&part, func(partIdx int64) error {
			if found, err := part.Sectors.IsSet(uint64(num)); err != nil {
				return err
			} else if !found {
				return nil
			}
			if found, err := part.Terminated.IsSet(uint64(num)); err != nil {
				return err
			} else if found {
				// already terminated
				return stopErr
			}

			q, err := miner3.LoadExpirationQueue(s.store, part.ExpirationsEpochs, quant, miner3.PartitionExpirationAmtBitwidth)
			if err != nil {
				return err
			}
			var exp miner3.ExpirationSet
			return q.ForEach(&exp, func(epoch int64) error {
				if early, err := exp.EarlySectors.IsSet(uint64(num)); err != nil {
					return err
				} else if early {
					out.Early = abi.ChainEpoch(epoch)
					return nil
				}
				if onTime, err := exp.OnTimeSectors.IsSet(uint64(num)); err != nil {
					return err
				} else if onTime {
					out.OnTime = abi.ChainEpoch(epoch)
					return stopErr
				}
				return nil
			})
		})
	})
	if err == stopErr {
		err = nil
	}
	if err != nil {
		return nil, err
	}
	if out.Early == 0 && out.OnTime == 0 {
		return nil, xerrors.Errorf("failed to find sector %d", num)
	}
	return &out, nil
}

func (s *state3) GetPrecommittedSector(num abi.SectorNumber) (*SectorPreCommitOnChainInfo, error) {
	info, ok, err := s.State.GetPrecommittedSector(s.store, num)
	if !ok || err != nil {
		return nil, err
	}

	ret := fromV3SectorPreCommitOnChainInfo(*info)

	return &ret, nil
}

func (s *state3) LoadSectors(snos *bitfield.BitField) ([]*SectorOnChainInfo, error) {
	sectors, err := miner3.LoadSectors(s.store, s.State.Sectors)
	if err != nil {
		return nil, err
	}

	// If no sector numbers are specified, load all.
	if snos == nil {
		infos := make([]*SectorOnChainInfo, 0, sectors.Length())
		var info2 miner3.SectorOnChainInfo
		if err := sectors.ForEach(&info2, func(_ int64) error {
			info := fromV3SectorOnChainInfo(info2)
			infos = append(infos, &info)
			return nil
		}); err != nil {
			return nil, err
		}
		return infos, nil
	}

	// Otherwise, load selected.
	infos2, err := sectors.Load(*snos)
	if err != nil {
		return nil, err
	}
	infos := make([]*SectorOnChainInfo, len(infos2))
	for i, info2 := range infos2 {
		info := fromV3SectorOnChainInfo(*info2)
		infos[i] = &info
	}
	return infos, nil
}

func (s *state3) IsAllocated(num abi.SectorNumber) (bool, error) {
	var allocatedSectors bitfield.BitField
	if err := s.store.Get(s.store.Context(), s.State.AllocatedSectors, &allocatedSectors); err != nil {
		return false, err
	}

	return allocatedSectors.IsSet(uint64(num))
}

func (s *state3) LoadDeadline(idx uint64) (Deadline, error) {
	dls, err := s.State.LoadDeadlines(s.store)
	if err != nil {
		return nil, err
	}
	dl, err := dls.LoadDeadline(s.store, idx)
	if err != nil {
		return nil, err
	}
	return &deadline3{*dl, s.store}, nil
}

func (s *state3) ForEachDeadline(cb func(uint64, Deadline) error) error {
	dls, err := s.State.LoadDeadlines(s.store)
	if err != nil {
		return err
	}
	return dls.ForEach(s.store, func(i uint64, dl *miner3.Deadline) error {
		return cb(i, &deadline3{*dl, s.store})
	})
}

func (s *state3) NumDeadlines() (uint64, error) {
	return miner3.WPoStPeriodDeadlines, nil
}

func (s *state3) DeadlinesChanged(other State) (bool, error) {
	other2, ok := other.(*state3)
	if !ok {
		// treat an upgrade as a change, always
		return true, nil
	}

	return !s.State.Deadlines.Equals(other2.Deadlines), nil
}

func (s *state3) MinerInfoChanged(other State) (bool, error) {
	other0, ok := other.(*state3)
	if !ok {
		// treat an upgrade as a change, always
		return true, nil
	}
	return !s.State.Info.Equals(other0.State.Info), nil
}

func (s *state3) Info() (MinerInfo, error) {
	info, err := s.State.GetInfo(s.store)
	if err != nil {
		return MinerInfo{}, err
	}

	var pid *peer.ID
	if peerID, err := peer.IDFromBytes(info.PeerId); err == nil {
		pid = &peerID
	}

	mi := MinerInfo{
		Owner:            info.Owner,
		Worker:           info.Worker,
		ControlAddresses: info.ControlAddresses,

		NewWorker:         address.Undef,
		WorkerChangeEpoch: -1,

		PeerId:                     pid,
		Multiaddrs:                 info.Multiaddrs,
		WindowPoStProofType:        info.WindowPoStProofType,
		SectorSize:                 info.SectorSize,
		WindowPoStPartitionSectors: info.WindowPoStPartitionSectors,
		ConsensusFaultElapsed:      info.ConsensusFaultElapsed,
	}

	if info.PendingWorkerKey != nil {
		mi.NewWorker = info.PendingWorkerKey.NewWorker
		mi.WorkerChangeEpoch = info.PendingWorkerKey.EffectiveAt
	}

	return mi, nil
}

func (s *state3) DeadlineInfo(epoch abi.ChainEpoch) (*dline.Info, error) {
	return s.State.DeadlineInfo(epoch), nil
}

func (s *state3) SectorArray() (adt.Array, error) {
	return adt3.AsArray(s.store, s.Sectors, miner3.SectorsAmtBitwidth)
}

func (s *state3) sectors() (adt.Array, error) {
	return adt3.AsArray(s.store, s.Sectors, miner3.SectorsAmtBitwidth)
}

func (s *state3) decodeSectorOnChainInfo(val *cbg.Deferred) (SectorOnChainInfo, error) {
	var si miner3.SectorOnChainInfo
	err := si.UnmarshalCBOR(bytes.NewReader(val.Raw))
	if err != nil {
		return SectorOnChainInfo{}, err
	}

	return fromV3SectorOnChainInfo(si), nil
}

func (s *state3) precommits() (adt.Map, error) {
	return adt3.AsMap(s.store, s.PreCommittedSectors, builtin3.DefaultHamtBitwidth)
}

func (s *state3) decodeSectorPreCommitOnChainInfo(val *cbg.Deferred) (SectorPreCommitOnChainInfo, error) {
	var sp miner3.SectorPreCommitOnChainInfo
	err := sp.UnmarshalCBOR(bytes.NewReader(val.Raw))
	if err != nil {
		return SectorPreCommitOnChainInfo{}, err
	}

	return fromV3SectorPreCommitOnChainInfo(sp), nil
}

func (d *deadline3) LoadPartition(idx uint64) (Partition, error) {
	p, err := d.Deadline.LoadPartition(d.store, idx)
	if err != nil {
		return nil, err
	}
	return &partition3{*p, d.store}, nil
}

func (d *deadline3) ForEachPartition(cb func(uint64, Partition) error) error {
	ps, err := d.Deadline.PartitionsArray(d.store)
	if err != nil {
		return err
	}
	var part miner3.Partition
	return ps.ForEach(&part, func(i int64) error {
		return cb(uint64(i), &partition3{part, d.store})
	})
}

func (d *deadline3) PartitionsChanged(other Deadline) (bool, error) {
	other2, ok := other.(*deadline3)
	if !ok {
		// treat an upgrade as a change, always
		return true, nil
	}

	return !d.Deadline.Partitions.Equals(other2.Deadline.Partitions), nil
}

func (d *deadline3) PartitionsPoSted() (bitfield.BitField, error) {
	return d.Deadline.PartitionsPoSted, nil
}

func (p *partition3) AllSectors() (bitfield.BitField, error) {
	return p.Partition.Sectors, nil
}

func (p *partition3) FaultySectors() (bitfield.BitField, error) {
	return p.Partition.Faults, nil
}

func (p *partition3) RecoveringSectors() (bitfield.BitField, error) {
	return p.Partition.Recoveries, nil
}

func fromV3SectorOnChainInfo(v3 miner3.SectorOnChainInfo) SectorOnChainInfo {
	return SectorOnChainInfo{
		SectorNumber:          v3.SectorNumber,
		SealProof:             v3.SealProof,
		SealedCID:             v3.SealedCID,
		DealIDs:               v3.DealIDs,
		Activation:            v3.Activation,
		Expiration:            v3.Expiration,
		DealWeight:            v3.DealWeight,
		VerifiedDealWeight:    v3.VerifiedDealWeight,
		InitialPledge:         v3.InitialPledge,
		ExpectedDayReward:     v3.ExpectedDayReward,
		ExpectedStoragePledge: v3.ExpectedStoragePledge,
	}
}

func fromV3SectorPreCommitOnChainInfo(v3 miner3.SectorPreCommitOnChainInfo) SectorPreCommitOnChainInfo {
	return SectorPreCommitOnChainInfo{
		Info:               (SectorPreCommitInfo)(v3.Info),
		PreCommitDeposit:   v3.PreCommitDeposit,
		PreCommitEpoch:     v3.PreCommitEpoch,
		DealWeight:         v3.DealWeight,
		VerifiedDealWeight: v3.VerifiedDealWeight,
	}
}
