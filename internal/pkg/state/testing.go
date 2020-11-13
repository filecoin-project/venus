package state

import (
	"context"
	"github.com/filecoin-project/go-bitfield"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/internal/pkg/specactors/builtin"
	"github.com/filecoin-project/venus/internal/pkg/specactors/builtin/miner"
	"github.com/filecoin-project/venus/internal/pkg/util/ffiwrapper"
)

// FakeStateView is a fake state view.
type FakeStateView struct {
	NetworkName string
	Power       *NetworkPower
	Miners      map[address.Address]*FakeMinerState
}

// NewFakeStateView creates a new fake state view.
func NewFakeStateView(rawBytePower, qaPower abi.StoragePower, minerCount, minPowerMinerCount int64) *FakeStateView {
	return &FakeStateView{
		Power: &NetworkPower{
			RawBytePower:         rawBytePower,
			QualityAdjustedPower: qaPower,
			MinerCount:           minerCount,
			MinPowerMinerCount:   minPowerMinerCount,
		},
		Miners: make(map[address.Address]*FakeMinerState),
	}
}

// FakeMinerState is fake state for a single miner.
type FakeMinerState struct {
	SectorConfiguration *MinerSectorConfiguration
	Owner               address.Address
	Worker              address.Address
	PeerID              peer.ID
	ProvingPeriodStart  abi.ChainEpoch
	ProvingPeriodEnd    abi.ChainEpoch
	PoStFailures        int
	Sectors             []miner.SectorOnChainInfo
	Deadlines           []*bitfield.BitField
	ClaimedRawPower     abi.StoragePower
	ClaimedQAPower      abi.StoragePower
	PledgeRequirement   abi.TokenAmount
	PledgeBalance       abi.TokenAmount
}

// FakeSectorInfo fakes a subset of sector onchain info
type FakeSectorInfo struct {
	ID        abi.SectorNumber
	SealedCID cid.Cid
}

func (v *FakeStateView) InitNetworkName(_ context.Context) (string, error) {
	return v.NetworkName, nil
}

// MinerSectorConfiguration reports a miner's sector size.
func (v *FakeStateView) MinerSectorConfiguration(ctx context.Context, maddr address.Address) (*MinerSectorConfiguration, error) {
	m, ok := v.Miners[maddr]
	if !ok {
		return nil, errors.Errorf("no miner %s", maddr)
	}
	return m.SectorConfiguration, nil
}

// MinerSectorCount reports the number of sectors a miner has pledged
func (v *FakeStateView) MinerSectorCount(ctx context.Context, maddr address.Address) (uint64, error) {
	m, ok := v.Miners[maddr]
	if !ok {
		return 0, errors.Errorf("no miner %s", maddr)
	}

	return uint64(len(m.Sectors)), nil
}

func (v *FakeStateView) MinerGetSector(_ context.Context, maddr address.Address, sectorNum abi.SectorNumber) (*miner.SectorOnChainInfo, bool, error) {
	m, ok := v.Miners[maddr]
	if !ok {
		return nil, false, errors.Errorf("no miner %s", maddr)
	}
	for _, s := range m.Sectors {
		if s.SectorNumber == sectorNum {
			return &s, true, nil
		}
	}
	return nil, false, nil
}

// MinerControlAddresses reports a miner's control addresses.
func (v *FakeStateView) MinerControlAddresses(_ context.Context, maddr address.Address) (owner, worker address.Address, err error) {
	m, ok := v.Miners[maddr]
	if !ok {
		return address.Undef, address.Undef, errors.Errorf("no miner %s", maddr)
	}
	return m.Owner, m.Worker, nil
}

func (v *FakeStateView) MinerExists(_ context.Context, _ address.Address) (bool, error) {
	return true, nil
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

func (v *FakeStateView) AccountSignerAddress(ctx context.Context, a address.Address) (address.Address, error) {
	return a, nil
}

func (v *FakeStateView) PowerNetworkTotal(_ context.Context) (*NetworkPower, error) {
	return v.Power, nil
}

func (v *FakeStateView) MinerClaimedPower(ctx context.Context, miner address.Address) (abi.StoragePower, abi.StoragePower, error) {
	m, ok := v.Miners[miner]
	if !ok {
		return big.Zero(), big.Zero(), errors.Errorf("no miner %s", miner)
	}
	return m.ClaimedRawPower, m.ClaimedQAPower, nil
}

func (v *FakeStateView) GetSectorsForWinningPoSt(ctx context.Context, pv ffiwrapper.Verifier, st cid.Cid, maddr address.Address, rand abi.PoStRandomness) ([]builtin.SectorInfo, error) {
	_, ok := v.Miners[maddr]
	if !ok {
		return nil, errors.Errorf("no miner %s", maddr)
	}
	return []builtin.SectorInfo{}, nil
}

func (v *FakeStateView) MinerPledgeCollateral(_ context.Context, maddr address.Address) (locked abi.TokenAmount, total abi.TokenAmount, err error) {
	m, ok := v.Miners[maddr]
	if !ok {
		return big.Zero(), big.Zero(), errors.Errorf("no miner %s", maddr)
	}
	return m.PledgeRequirement, m.PledgeBalance, nil
}

func (v *FakeStateView) MinerInfo(ctx context.Context, maddr address.Address) (*miner.MinerInfo, error) {
	m, ok := v.Miners[maddr]
	if !ok {
		return nil, errors.Errorf("no miner %s", maddr)
	}
	return &miner.MinerInfo{
		Owner:  m.Owner,
		Worker: m.Worker,
		PeerId: &m.PeerID,
	}, nil
}

func NewBitField() *bitfield.BitField {
	bit := bitfield.New()
	return &bit
}
