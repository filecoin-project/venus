package beacon

import (
	"fmt"
	"sort"

	"github.com/filecoin-project/go-state-types/abi"

	cfg "github.com/filecoin-project/venus/pkg/config"
)

type Schedule []BeaconPoint

//BeaconForEpoch select beacon at specify epoch
func (bs Schedule) BeaconForEpoch(e abi.ChainEpoch) RandomBeacon {
	for i := len(bs) - 1; i >= 0; i-- {
		bp := bs[i]
		if e >= bp.Start {
			return bp.Beacon
		}
	}
	return bs[0].Beacon
}

//DrandConfigSchedule create new beacon schedule , used to select beacon server at specify chain height
func DrandConfigSchedule(genTimeStamp uint64, blockDelay uint64, drandSchedule map[abi.ChainEpoch]cfg.DrandEnum) (Schedule, error) {
	shd := Schedule{}

	for start, config := range drandSchedule {
		bc, err := NewDrandBeacon(genTimeStamp, blockDelay, cfg.DrandConfigs[config])
		if err != nil {
			return nil, fmt.Errorf("creating drand beacon: %v", err)
		}
		shd = append(shd, BeaconPoint{Start: start, Beacon: bc})
	}

	sort.Slice(shd, func(i, j int) bool {
		return shd[i].Start < shd[j].Start
	})

	log.Infof("Schedule: %v", shd)
	return shd, nil
}
