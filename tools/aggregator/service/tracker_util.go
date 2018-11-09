package aggregator

import (
	"sort"
)

type tipsetRank struct {
	Tipset string
	Rank   int
}

// nodesInConsensus calculates the number of nodes in consensus and the heaviesttipset
func nodesInConsensus(tipsetCount map[string]int) (int, string) {
	var out []tipsetRank
	for t, r := range tipsetCount {
		tr := tipsetRank{
			Tipset: t,
			Rank:   r,
		}
		out = append(out, tr)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Rank > out[j].Rank })
	if len(out) > 1 && out[0].Rank == out[1].Rank {
		return 0, ""
	}
	return out[0].Rank, out[0].Tipset
}
