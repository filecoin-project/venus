package state

import (
	"context"

	"github.com/filecoin-project/go-filecoin/internal/pkg/constants"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin/miner"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
)

// FakeStateView is a fake state view.
type FakeStateView struct {
	NetworkName  string
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
	SectorSize         abi.SectorSize
	Owner              address.Address
	Worker             address.Address
	PeerID             peer.ID
	ProvingPeriodStart abi.ChainEpoch
	ProvingPeriodEnd   abi.ChainEpoch
	PoStFailures       int
	Sectors            []miner.SectorOnChainInfo
	ProvingSet         []FakeSectorInfo
	ClaimedPower       abi.StoragePower
	PledgeRequirement  abi.TokenAmount
	PledgeBalance      abi.TokenAmount
}

// FakeSectorInfo fakes a subset of sector onchain info
type FakeSectorInfo struct {
	ID        abi.SectorNumber
	SealedCID cid.Cid
}

func (v *FakeStateView) InitNetworkName(_ context.Context) (string, error) {
	return v.NetworkName, nil
}

// MinerSectorSize reports a miner's sector size.
func (v *FakeStateView) MinerSectorSize(_ context.Context, maddr address.Address) (abi.SectorSize, error) {
	m, ok := v.Miners[maddr]
	if !ok {
		return 0, errors.Errorf("no miner %s", maddr)
	}
	return m.SectorSize, nil
}

// MinerSectorCount reports the number of sectors a miner has pledged
func (v *FakeStateView) MinerSectorCount(ctx context.Context, maddr address.Address) (int, error) {
	m, ok := v.Miners[maddr]
	if !ok {
		return 0, errors.Errorf("no miner %s", maddr)
	}

	return len(m.Sectors), nil
}

// MinerControlAddresses reports a miner's control addresses.
func (v *FakeStateView) MinerControlAddresses(_ context.Context, maddr address.Address) (owner, worker address.Address, err error) {
	m, ok := v.Miners[maddr]
	if !ok {
		return address.Undef, address.Undef, errors.Errorf("no miner %s", maddr)
	}
	return m.Owner, m.Worker, nil
}

func (v *FakeStateView) MinerPeerID(ctx context.Context, maddr address.Address) (peer.ID, error) {
	m, ok := v.Miners[maddr]
	if !ok {
		return "", errors.Errorf("no miner %s", maddr)
	}
	return m.PeerID, nil
}

func (v *FakeStateView) MinerProvingPeriod(ctx context.Context, maddr address.Address) (start abi.ChainEpoch, end abi.ChainEpoch, failureCount int, err error) {
	m, ok := v.Miners[maddr]
	if !ok {
		return 0, 0, 0, errors.Errorf("no miner %s", maddr)
	}
	return m.ProvingPeriodStart, m.ProvingPeriodEnd, m.PoStFailures, nil
}

// MinerProvingSetForEach iterates sectors in a miner's proving set.
func (v *FakeStateView) MinerProvingSetForEach(_ context.Context, maddr address.Address, f func(id abi.SectorNumber, sealedCID cid.Cid, rpp abi.RegisteredProof) error) error {
	m, ok := v.Miners[maddr]
	if !ok {
		return errors.Errorf("no miner %s", maddr)
	}

	for _, si := range m.ProvingSet {
		err := f(si.ID, si.SealedCID, constants.DevRegisteredSealProof)
		if err != nil {
			return err
		}
	}
	return nil
}

func (v *FakeStateView) AccountSignerAddress(ctx context.Context, a address.Address) (address.Address, error) {
	return a, nil
}

// NetworkTotalPower reports a network's total power.
func (v *FakeStateView) NetworkTotalPower(_ context.Context) (abi.StoragePower, error) {
	return v.NetworkPower, nil
}

// MinerClaimedPower reports a miner's claimed power.
func (v *FakeStateView) MinerClaimedPower(_ context.Context, maddr address.Address) (abi.StoragePower, error) {
	m, ok := v.Miners[maddr]
	if !ok {
		return big.Zero(), errors.Errorf("no miner %s", maddr)
	}
	return m.ClaimedPower, nil
}

func (v *FakeStateView) MinerPledgeCollateral(_ context.Context, maddr address.Address) (locked abi.TokenAmount, total abi.TokenAmount, err error) {
	m, ok := v.Miners[maddr]
	if !ok {
		return big.Zero(), big.Zero(), errors.Errorf("no miner %s", maddr)
	}
	return m.PledgeRequirement, m.PledgeBalance, nil
}
