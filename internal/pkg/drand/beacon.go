package drand

import (
	"sort"
	"time"

	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/filecoin-project/go-state-types/abi"
)

type DrandSchedule []DrandPoint

type DrandPoint struct {
	Start  abi.ChainEpoch
	Config DrandConfig
}

type DrandConfig struct {
	Servers       []string
	Relays        []string
	ChainInfoJSON string
}

type DrandEnum int

func DrandConfigSchedule() DrandSchedule {
	out := DrandSchedule{}
	for start, config := range DrandSchedules {
		out = append(out, DrandPoint{Start: start, Config: DrandConfigs[config]})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Start < out[j].Start
	})

	return out
}

func RandomSchedule(tg time.Time) (Schedule, error) {
	cfg := DrandConfigSchedule()

	shd := Schedule{}
	for _, dc := range cfg {
		bc, err := NewGRPC(tg, clock.DefaultEpochDuration, dc.Config)
		if err != nil {
			return nil, xerrors.Errorf("creating drand beacon: %w", err)
		}
		shd = append(shd, BeaconPoint{Start: dc.Start, Beacon: bc})
	}

	return shd, nil
}
