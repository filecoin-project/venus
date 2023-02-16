package peermgr

import (
	"context"
	"fmt"
	"sync"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/event"
	host "github.com/libp2p/go-libp2p/core/host"
	net "github.com/libp2p/go-libp2p/core/network"
	peer "github.com/libp2p/go-libp2p/core/peer"
	swarm "github.com/libp2p/go-libp2p/p2p/net/swarm"
	ma "github.com/multiformats/go-multiaddr"

	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("peermgr")

const (
	MaxFilPeers = 4200
	MinFilPeers = 4000
)

type IPeerMgr interface {
	AddFilecoinPeer(p peer.ID)
	GetPeerLatency(p peer.ID) (time.Duration, bool)
	SetPeerLatency(p peer.ID, latency time.Duration)
	Disconnect(p peer.ID)
	Stop(ctx context.Context) error
	Run(ctx context.Context)
}

var (
	_ IPeerMgr = &PeerMgr{}
	_ IPeerMgr = &MockPeerMgr{}
)

type PeerMgr struct {
	bootstrappers []peer.AddrInfo

	// peerLeads is a set of peers we hear about through the network
	// and who may be good peers to connect to for expanding our peer set
	// peerLeads map[peer.ID]time.Time // TODO: unused

	peersLk sync.Mutex
	peers   map[peer.ID]time.Duration

	maxFilPeers int
	minFilPeers int

	expanding chan struct{}

	h   host.Host
	dht *dht.IpfsDHT

	notifee        *net.NotifyBundle
	filPeerEmitter event.Emitter

	period time.Duration
	done   chan struct{}

	sender chan *PeerInfo
}

type FilPeerEvt struct {
	Type FilPeerEvtType
	ID   peer.ID
}

type FilPeerEvtType int

const (
	AddFilPeerEvt FilPeerEvtType = iota
	RemoveFilPeerEvt
)

func NewPeerMgr(h host.Host, dht *dht.IpfsDHT, period time.Duration, bootstrap []peer.AddrInfo) (*PeerMgr, error) {
	pm := &PeerMgr{
		h:             h,
		dht:           dht,
		bootstrappers: bootstrap,

		peers:     make(map[peer.ID]time.Duration),
		expanding: make(chan struct{}, 1),

		maxFilPeers: MaxFilPeers,
		minFilPeers: MinFilPeers,

		done:   make(chan struct{}),
		period: period,

		sender: make(chan *PeerInfo, 100),
	}
	emitter, err := h.EventBus().Emitter(new(FilPeerEvt))
	if err != nil {
		return nil, fmt.Errorf("creating NewFilPeer emitter: %w", err)
	}
	pm.filPeerEmitter = emitter

	s := newStat(pm.sender, func(id peer.ID) {
		pm.connect(id)
	})
	s.start(context.Background())

	go func() {
		ticker := time.NewTicker(3 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			l := len(h.Network().Conns())
			seen := make(map[peer.ID]struct{}, l)

			for _, conn := range h.Network().Conns() {
				if _, ok := seen[conn.RemotePeer()]; ok {
					continue
				}
				seen[conn.RemotePeer()] = struct{}{}
				pm.send(conn.RemotePeer(), conn.RemoteMultiaddr().String())
			}

			log.Infof("connected peers len %d")
		}
	}()

	pm.notifee = &net.NotifyBundle{
		DisconnectedF: func(_ net.Network, c net.Conn) {
			pm.Disconnect(c.RemotePeer())
		},
		ConnectedF: func(n net.Network, c net.Conn) {
			pm.send(c.RemotePeer(), c.RemoteMultiaddr().String())
		},
	}

	h.Network().Notify(pm.notifee)

	return pm, nil
}

func (pmgr *PeerMgr) AddFilecoinPeer(p peer.ID) {
	_ = pmgr.filPeerEmitter.Emit(FilPeerEvt{Type: AddFilPeerEvt, ID: p}) //nolint:errcheck

	pi := pmgr.h.Peerstore().PeerInfo(p)
	pmgr.send(p, firstAddr(pi.Addrs))

	pmgr.peersLk.Lock()
	defer pmgr.peersLk.Unlock()
	pmgr.peers[p] = time.Duration(0)
}

func (pmgr *PeerMgr) GetPeerLatency(p peer.ID) (time.Duration, bool) {
	pmgr.peersLk.Lock()
	defer pmgr.peersLk.Unlock()
	dur, ok := pmgr.peers[p]
	return dur, ok
}

func (pmgr *PeerMgr) SetPeerLatency(p peer.ID, latency time.Duration) {
	pmgr.peersLk.Lock()
	defer pmgr.peersLk.Unlock()
	if _, ok := pmgr.peers[p]; ok {
		pmgr.peers[p] = latency
	}
}

func (pmgr *PeerMgr) Disconnect(p peer.ID) {
	disconnected := false

	if pmgr.h.Network().Connectedness(p) == net.NotConnected {
		pmgr.peersLk.Lock()
		_, disconnected = pmgr.peers[p]
		if disconnected {
			delete(pmgr.peers, p)
		}
		pmgr.peersLk.Unlock()
	}

	if disconnected {
		_ = pmgr.filPeerEmitter.Emit(FilPeerEvt{Type: RemoveFilPeerEvt, ID: p}) //nolint:errcheck
	}
}

func (pmgr *PeerMgr) Stop(ctx context.Context) error {
	log.Warn("closing peermgr done")
	_ = pmgr.filPeerEmitter.Close()
	close(pmgr.done)
	return nil
}

func (pmgr *PeerMgr) Run(ctx context.Context) {
	tick := time.NewTicker(pmgr.period)
	defer tick.Stop()

	for {
		pCount := pmgr.getPeerCount()
		if pCount < pmgr.minFilPeers {
			pmgr.expandPeers()
		} else if pCount > pmgr.maxFilPeers {
			log.Debugf("peer count about threshold: %d > %d", pCount, pmgr.maxFilPeers)
		}

		select {
		case <-tick.C:
			continue
		case <-pmgr.done:
			log.Warn("exiting peermgr run")
			return
		}
	}
}

func (pmgr *PeerMgr) getPeerCount() int {
	pmgr.peersLk.Lock()
	defer pmgr.peersLk.Unlock()
	return len(pmgr.peers)
}

func (pmgr *PeerMgr) expandPeers() {
	select {
	case pmgr.expanding <- struct{}{}:
	default:
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.TODO(), time.Second*30)
		defer cancel()

		pmgr.doExpand(ctx)

		<-pmgr.expanding
	}()
}

func (pmgr *PeerMgr) doExpand(ctx context.Context) {
	pcount := pmgr.getPeerCount()
	if pcount == 0 {
		if len(pmgr.bootstrappers) == 0 {
			log.Warn("no peers connected, and no bootstrappers configured")
			return
		}

		log.Info("connecting to bootstrap peers")
		for _, bsp := range pmgr.bootstrappers {
			if err := pmgr.h.Connect(ctx, bsp); err != nil {
				log.Warnf("failed to connect to bootstrap peer: %s", err)
			}
		}
		return
	}

	// if we already have some peers and need more, the dht is really good at connecting to most peers. Use that for now until something better comes along.
	if err := pmgr.dht.Bootstrap(ctx); err != nil {
		log.Warnf("dht bootstrapping failed: %s", err)
	}
}

func (pmgr *PeerMgr) send(id peer.ID, addr string) {
	agent, _ := pmgr.getAgent(id)

	pi := &PeerInfo{
		ID:    id,
		Addr:  addr,
		Agent: agent,
	}

	if len(pi.Agent) == 0 {
		pmgr.connect(id)
	}

	go func() {
		pmgr.sender <- pi
	}()
}

func (pmgr *PeerMgr) connect(id peer.ID) {
	pi := pmgr.h.Peerstore().PeerInfo(id)
	if swarm, ok := pmgr.h.Network().(*swarm.Swarm); ok {
		swarm.Backoff().Clear(pi.ID)
	}
	if err := pmgr.h.Connect(context.Background(), pi); err != nil {
		log.Warnf("connect %v failed %v", pi.ID, err)
	} else {
		log.Infof("connect %v success", pi.ID)
	}
}

func (pmgr *PeerMgr) getAgent(id peer.ID) (string, error) {
	var agent string
	agentI, err := pmgr.h.Peerstore().Get(id, "AgentVersion")
	if err != nil {
		log.Infof("get agent version failed %v %v", id, err)
	} else {
		agent, _ = agentI.(string)
	}

	return agent, err
}

func firstAddr(addrs []ma.Multiaddr) string {
	var addr string
	if len(addrs) > 0 {
		addr = addrs[0].String()
	}

	return addr
}

type MockPeerMgr struct{}

func (m MockPeerMgr) AddFilecoinPeer(p peer.ID) {}

func (m MockPeerMgr) GetPeerLatency(p peer.ID) (time.Duration, bool) {
	return time.Duration(0), true
}

func (m MockPeerMgr) SetPeerLatency(p peer.ID, latency time.Duration) {}

func (m MockPeerMgr) Disconnect(p peer.ID) {}

func (m MockPeerMgr) Stop(ctx context.Context) error {
	return nil
}

func (m MockPeerMgr) Run(ctx context.Context) {}
