package consensus

import (
	"context"
	"sort"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
	"github.com/ipfs/go-ipfs-blockstore"
	"github.com/pkg/errors"
)

type ProtocolUpgrade struct {
	Version     uint64
	EffectiveAt *types.BlockHeight
}

type ProtocolUpgradeTable struct {
	Network  string
	upgrades []ProtocolUpgrade
}

func NewProtocolUpgradeTable(network string) *ProtocolUpgradeTable {

}

func (put *ProtocolUpgradeTable) upgrade(network string, version uint64, effectiveAt *types.BlockHeight) {
	// ignore upgrade if not part of our
	if network == put.Network {
		upgrade := ProtocolUpgrade{
			Version:     version,
			EffectiveAt: effectiveAt,
		}

		idx := sort.Search(len(put.upgrades), func(i int) bool {
			return effectiveAt.GreaterEqual(put.upgrades[i].EffectiveAt)
		})

		// insert upgrade
		put.upgrades = append(put.upgrades, ProtocolUpgrade{})
		copy(put.upgrades[idx+1:], put.upgrades[idx:])

		put.upgrades = append(put.upgrades[:idx], append([]ProtocolUpgrade{upgrade}, put.upgrades[idx:]...)...)
	}
}

func (put *ProtocolUpgradeTable) VersionAt(height types.BlockHeight) (uint64, error) {
	// find index of first upgrade that is yet active (or len(upgrades) if they are all active.
	idx := sort.Search(len(put.upgrades), func(i int) bool {
		return height.LessThan(put.upgrades[i].EffectiveAt)
	})

	// providing a height less than the first upgrade is an error
	if idx == 0 {
		if len(put.upgrades) == 0 {
			return 0, errors.Errorf("no protocol versions for %s network", put.Network)
		}
		return 0, errors.Errorf("chain height %s is less than effective start of first upgrade %s for network %s",
			height.String(), put.upgrades[0].EffectiveAt.String(), put.Network)
	}

	// return the upgrade just prior to the index to get the last upgrade in effect.
	return put.upgrades[idx-1].Version, nil
}

// NetworkNameFromGenesis retrieves the name of the current network from the genesis block.
// The network name can not change while this node is running. Since the network name determines
// the protocol version, we must retrieve it at genesis where the protocol is known.
func NetworkNameFromGenesis(ctx context.Context, store chain.Store, bs blockstore.Blockstore) (string, error) {
	genesisTipsetKey := types.NewTipSetKey(store.GenesisCid())
	st, err := store.GetTipSetState(ctx, genesisTipsetKey)
	if err != nil {
		return "", err
	}

	vms := vm.NewStorageMap(bs)
	res, _, err := CallQueryMethod(ctx, st, vms, address.InitAddress, "getNetwork", nil, address.Undef, nil)
	if err != nil {
		return "", err
	}

	return string(res[0]), nil
}
