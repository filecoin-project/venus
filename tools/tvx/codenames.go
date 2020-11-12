package main

import (
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/internal/pkg/fork"
)

// ProtocolCodenames is a table that summarises the protocol codenames that
// will be set on extracted vectors, depending on the original execution height.
//
// Implementers rely on these names to filter the vectors they can run through
// their implementations, based on their support level
var ProtocolCodenames = []struct {
	firstEpoch abi.ChainEpoch
	name       string
}{
	{0, "genesis"},
	{fork.UpgradeBreezeHeight + 1, "breeze"},
	{fork.UpgradeSmokeHeight + 1, "smoke"},
	{fork.UpgradeIgnitionHeight + 1, "ignition"},
	{fork.UpgradeRefuelHeight + 1, "refuel"},
	{fork.UpgradeActorsV2Height + 1, "actorsv2"},
	{fork.UpgradeTapeHeight + 1, "tape"},
	{fork.UpgradeLiftoffHeight + 1, "liftoff"},
	{fork.UpgradeKumquatHeight + 1, "postliftoff"},
}

// GetProtocolCodename gets the protocol codename associated with a height.
func GetProtocolCodename(height abi.ChainEpoch) string {
	for i, v := range ProtocolCodenames {
		if height < v.firstEpoch {
			// found the cutoff, return previous.
			return ProtocolCodenames[i-1].name
		}
	}
	return ProtocolCodenames[len(ProtocolCodenames)-1].name
}
