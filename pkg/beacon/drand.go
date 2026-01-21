package beacon

import (
	"bytes"
	"context"
	"fmt"
	"time"

	dcommon "github.com/drand/drand/v2/common"
	dchain "github.com/drand/drand/v2/common/chain"
	dlog "github.com/drand/drand/v2/common/log"
	dcrypto "github.com/drand/drand/v2/crypto"
	dclient "github.com/drand/go-clients/client"
	hclient "github.com/drand/go-clients/client/http"
	drand "github.com/drand/go-clients/drand"
	"github.com/drand/kyber"
	lru "github.com/hashicorp/golang-lru/v2"
	"go.uber.org/zap"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"

	cfg "github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/venus-shared/types"
)

// DrandBeacon connects Lotus with a drand network in order to provide
// randomness to the system in a way that's aligned with Filecoin rounds/epochs.
//
// We connect to drand peers via their public HTTP endpoints. The peers are
// enumerated in the drandServers variable.
//
// The root trust for the Drand chain is configured from build.DrandChain.
type DrandBeacon struct {
	isChained bool
	client    drand.Client

	pubkey kyber.Point

	// seconds
	interval time.Duration

	drandGenTime uint64
	filGenTime   uint64
	filRoundTime uint64
	scheme       *dcrypto.Scheme

	localCache *lru.Cache[uint64, *types.BeaconEntry]
}

func (db *DrandBeacon) IsChained() bool {
	return db.isChained
}

// DrandHTTPClient interface overrides the user agent used by drand
type DrandHTTPClient interface {
	SetUserAgent(string)
}

type logger struct {
	*zap.SugaredLogger
}

func (l *logger) With(args ...interface{}) dlog.Logger {
	return &logger{l.SugaredLogger.With(args...)}
}

func (l *logger) Named(s string) dlog.Logger {
	return &logger{l.SugaredLogger.Named(s)}
}

func (l *logger) AddCallerSkip(skip int) dlog.Logger {
	return &logger{l.SugaredLogger.With(zap.AddCallerSkip(skip))}
}

// NewDrandBeacon creates new beacon client from config, genesis block time and block delay
func NewDrandBeacon(genTimeStamp, interval uint64, config cfg.DrandConf) (*DrandBeacon, error) {
	drandChain, err := dchain.InfoFromJSON(bytes.NewReader([]byte(config.ChainInfoJSON)))
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal drand chain info: %w", err)
	}

	var clients []drand.Client
	for _, url := range config.Servers {
		hc, err := hclient.NewWithInfo(&logger{&log.SugaredLogger}, url, drandChain, nil)
		if err != nil {
			return nil, fmt.Errorf("could not create http drand client: %w", err)
		}
		hc.SetUserAgent("drand-client-lotus/" + constants.UserVersion())
		clients = append(clients, hc)
	}

	opts := []dclient.Option{
		dclient.WithChainInfo(drandChain),
		dclient.WithCacheSize(1024),
		dclient.WithLogger(&logger{&log.SugaredLogger}),
	}

	if len(clients) == 0 {
		// This is necessary to convince a drand beacon to start without any clients. For historical
		// beacons we need them to be able to verify old entries but we don't need to fetch new ones.
		clients = append(clients, dclient.EmptyClientWithInfo(drandChain))
	}

	client, err := dclient.Wrap(clients, opts...)
	if err != nil {
		return nil, fmt.Errorf("creating drand client: %v", err)
	}

	lc, err := lru.New[uint64, *types.BeaconEntry](1024)
	if err != nil {
		return nil, err
	}

	db := &DrandBeacon{
		isChained:  config.IsChained,
		client:     client,
		localCache: lc,
	}

	sch, err := dcrypto.GetSchemeByID(drandChain.Scheme)
	if err != nil {
		return nil, err
	}
	db.scheme = sch
	db.pubkey = drandChain.PublicKey
	db.interval = drandChain.Period
	db.drandGenTime = uint64(drandChain.GenesisTime)
	db.filRoundTime = interval
	db.filGenTime = genTimeStamp

	return db, nil
}

// Entry gets a beacon value of specified block height
func (db *DrandBeacon) Entry(ctx context.Context, round uint64) <-chan Response {
	out := make(chan Response, 1)
	if round != 0 {
		be := db.getCachedValue(round)
		if be != nil {
			out <- Response{Entry: *be}
			close(out)
			return out
		}
	}

	go func() {
		start := time.Now()
		log.Infow("start fetching randomness", "round", round)
		resp, err := db.client.Get(ctx, round)

		var br Response
		if err != nil {
			br.Err = fmt.Errorf("drand failed Get request: %w", err)
		} else {
			br.Entry.Round = resp.GetRound()
			br.Entry.Data = resp.GetSignature()
		}
		log.Infow("done fetching randomness", "round", round, "took", time.Since(start))
		out <- br
		close(out)
	}()

	return out
}

func (db *DrandBeacon) cacheValue(e types.BeaconEntry) {
	db.localCache.Add(e.Round, &e)
}

func (db *DrandBeacon) getCachedValue(round uint64) *types.BeaconEntry {
	v, _ := db.localCache.Get(round)
	return v
}

func (db *DrandBeacon) VerifyEntry(entry types.BeaconEntry, prevEntrySig []byte) error {
	if be := db.getCachedValue(entry.Round); be != nil {
		if !bytes.Equal(entry.Data, be.Data) {
			return fmt.Errorf("invalid beacon value, does not match cached good value")
		}
		// return no error if the value is in the cache already
		return nil
	}
	b := &dcommon.Beacon{
		PreviousSig: prevEntrySig,
		Round:       entry.Round,
		Signature:   entry.Data,
	}

	err := db.scheme.VerifyBeacon(b, db.pubkey)
	if err != nil {
		return fmt.Errorf("failed to verify beacon: %w", err)
	}

	db.cacheValue(entry)

	return nil
}

// MaxBeaconRoundForEpoch gets the turn of beacon chain corresponding to chain height
func (db *DrandBeacon) MaxBeaconRoundForEpoch(nv network.Version, filEpoch abi.ChainEpoch) uint64 {
	// TODO: sometimes the genesis time for filecoin is zero and this goes negative
	latestTS := ((uint64(filEpoch) * db.filRoundTime) + db.filGenTime) - db.filRoundTime

	if nv <= network.Version15 {
		return db.maxBeaconRoundV1(latestTS)
	}

	return db.maxBeaconRoundV2(latestTS)
}

func (db *DrandBeacon) maxBeaconRoundV1(latestTS uint64) uint64 {
	dround := (latestTS - db.drandGenTime) / uint64(db.interval.Seconds())
	return dround
}

func (db *DrandBeacon) maxBeaconRoundV2(latestTS uint64) uint64 {
	if latestTS < db.drandGenTime {
		return 1
	}

	fromGenesis := latestTS - db.drandGenTime
	// we take the time from genesis divided by the periods in seconds, that
	// gives us the number of periods since genesis.  We also add +1 because
	// round 1 starts at genesis time.
	return fromGenesis/uint64(db.interval.Seconds()) + 1
}

var _ RandomBeacon = (*DrandBeacon)(nil)
