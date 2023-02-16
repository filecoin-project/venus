package peermgr

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

func newStat(in chan *PeerInfo, connect func(id peer.ID)) *stat {
	s := &stat{
		in:      in,
		connect: connect,

		PeerInfos: &PeerInfos{
			Venus: make(map[string]*PeerInfo),
			Lotus: make(map[string]*PeerInfo),
			Other: make(map[string]*PeerInfo),
		},
	}

	return s
}

type stat struct {
	in      chan *PeerInfo
	connect func(peer.ID)

	*PeerInfos
}

type PeerInfos struct {
	Venus map[string]*PeerInfo
	Lotus map[string]*PeerInfo
	Other map[string]*PeerInfo
}

type PeerInfo struct {
	ID    peer.ID
	Addr  string
	Agent string
}

func (s *stat) start(ctx context.Context) {
	go func() {
		printInterval := time.NewTicker(3 * time.Minute)
		defer printInterval.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case pi := <-s.in:
				s.addPeerInfo(pi)
			case <-printInterval.C:
				s.unique()
				// for id, pi := range s.Other {
				// 	if len(pi.Agent) == 0 {
				// 		peerID, err := peer.Decode(id)
				// 		if err != nil {
				// 			log.Warnf("decode peer %v failed %v", id, err)
				// 			continue
				// 		}
				// 		s.connect(peerID)
				// 	}
				// }
				s.print()
			}
		}
	}()
}

func (s *stat) addPeerInfo(pi *PeerInfo) {
	typ := parsePeerTyp(pi.Agent)
	switch typ {
	case 0:
		s.Lotus[pi.ID.Pretty()] = pi
	case 1:
		s.Venus[pi.ID.Pretty()] = pi
	case 2:
		s.Other[pi.ID.Pretty()] = pi
	}
}

// parsePeerTyp return peet type, 0 lotus peer, 1 venus peer, 2 other peer
func parsePeerTyp(agent string) int {
	if strings.Contains(agent, "lotus") {
		return 0
	} else if strings.Contains(agent, "venus") {
		return 1
	} else {
		return 2
	}
}

func (s *stat) unique() {
	for id, pi := range s.Venus {
		if _, ok := s.Other[id]; ok && len(pi.Agent) > 0 {
			delete(s.Other, id)
		}
	}

	for id, pi := range s.Lotus {
		if _, ok := s.Other[id]; ok && len(pi.Agent) > 0 {
			delete(s.Other, id)
		}
	}
}

var outputFile = fmt.Sprintf("%s-peerinfo.log", time.Now().Format("2006-01-02T15"))

type output struct {
	*PeerInfos

	VenusCount int
	LotusCount int
	OtherCount int

	UpdateTime string
}

func (s *stat) print() {
	o := output{
		PeerInfos:  s.PeerInfos,
		VenusCount: len(s.Venus),
		LotusCount: len(s.Lotus),
		OtherCount: len(s.Other),
		UpdateTime: time.Now().Format("2006-01-02 15:04:05"),
	}

	data, err := json.MarshalIndent(o, "", "\t")
	if err != nil {
		log.Infof("marshal peer infos failed %v", err)
	} else {
		_ = os.WriteFile(outputFile, data, 0755)
	}
}
