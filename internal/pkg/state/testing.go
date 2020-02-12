package state

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/ipfs/go-cid"
)

// FakeStateView is a fake state view.
type FakeStateView struct {
	NetworkPower abi.StoragePower
	Miners       map[address.Address]*FakeMinerState
}

// NewFakeStateView creates a new fake state view.
func NewFakeStateView(networkPower abi.StoragePower) *FakeStateView {
	return &FakeStateView{
		NetworkPower: networkPower,
		Miners:       make(map[address.Address]*FakeMinerState),
	}
}

// FakeMinerState is fake state for a single miner.
type FakeMinerState struct {
	SectorSize   abi.SectorSize
	Owner        address.Address
	Worker       address.Address
	ProvingSet   []FakeSectorInfo
	ClaimedPower abi.StoragePower
}

// FakeSectorInfo fakes a subset of sector onchain info
type FakeSectorInfo struct {
	ID        abi.SectorNumber
	SealedCID cid.Cid
}

// AddMiner adds a new miner to the state.
func (v *FakeStateView) AddMiner(sectorSize abi.SectorSize, addr, owner, worker address.Address, provingSet []FakeSectorInfo, power abi.StoragePower) {
	v.Miners[addr] = &FakeMinerState{
		SectorSize:   sectorSize,
		Owner:        owner,
		Worker:       worker,
		ProvingSet:   provingSet,
		ClaimedPower: power,
	}
}

// MinerSectorSize reports a miner's sector size.
func (v *FakeStateView) MinerSectorSize(_ context.Context, maddr address.Address) (abi.SectorSize, error) {
	return v.Miners[maddr].SectorSize, nil
}

// MinerControlAddresses reports a miner's control addresses.
func (v *FakeStateView) MinerControlAddresses(_ context.Context, maddr address.Address) (owner, worker address.Address, err error) {
	m := v.Miners[maddr]
	return m.Owner, m.Worker, nil
}

// MinerProvingSetForEach iterates sectors in a miner's proving set.
func (v *FakeStateView) MinerProvingSetForEach(_ context.Context, maddr address.Address, f func(id abi.SectorNumber, sealedCID cid.Cid) error) error {
	for _, si := range v.Miners[maddr].ProvingSet {
		err := f(si.ID, si.SealedCID)
		if err != nil {
			return err
		}
	}
	return nil
}

// NetworkTotalPower reports a network's total power.
func (v *FakeStateView) NetworkTotalPower(_ context.Context) (abi.StoragePower, error) {
	return v.NetworkPower, nil
}

// MinerClaimedPower reports a miner's claimed power.
func (v *FakeStateView) MinerClaimedPower(_ context.Context, maddr address.Address) (abi.StoragePower, error) {
	return v.Miners[maddr].ClaimedPower, nil
}
