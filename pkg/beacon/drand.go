package beacon

import (
	"bytes"
	"context"
	"time"

	dchain "github.com/drand/drand/chain"
	dclient "github.com/drand/drand/client"
	hclient "github.com/drand/drand/client/http"
	dlog "github.com/drand/drand/log"
	"github.com/drand/kyber"
	kzap "github.com/go-kit/kit/log/zap"
	lru "github.com/hashicorp/golang-lru"
	"go.uber.org/zap/zapcore"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-state-types/abi"
	cfg "github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/types"
)

// DrandBeacon connects Lotus with a drand network in order to provide
// randomness to the system in a way that's aligned with Filecoin rounds/epochs.
//
// We connect to drand peers via their public HTTP endpoints. The peers are
// enumerated in the drandServers variable.
//
// The root trust for the Drand chain is configured from build.DrandChain.
type DrandBeacon struct {
	client dclient.Client

	pubkey kyber.Point

	// seconds
	interval time.Duration

	drandGenTime uint64
	filGenTime   uint64
	filRoundTime uint64

	localCache *lru.Cache
}

// DrandHTTPClient interface overrides the user agent used by drand
type DrandHTTPClient interface {
	SetUserAgent(string)
}

//NewDrandBeacon create new beacon client from config, genesis block time and block delay
func NewDrandBeacon(genTimeStamp, interval uint64, config cfg.DrandConf) (*DrandBeacon, error) {
	drandChain, err := dchain.InfoFromJSON(bytes.NewReader([]byte(config.ChainInfoJSON)))
	if err != nil {
		return nil, xerrors.Errorf("unable to unmarshal drand chain info: %w", err)
	}

	dlogger := dlog.NewKitLoggerFrom(kzap.NewZapSugarLogger(
		log.SugaredLogger.Desugar(), zapcore.InfoLevel))

	var clients []dclient.Client
	for _, url := range config.Servers {
		hc, err := hclient.NewWithInfo(url, drandChain, nil)
		if err != nil {
			return nil, xerrors.Errorf("could not create http drand client: %w", err)
		}
		hc.(DrandHTTPClient).SetUserAgent("drand-client-lotus/" + constants.BuildVersion)
		clients = append(clients, hc)

	}

	opts := []dclient.Option{
		dclient.WithChainInfo(drandChain),
		dclient.WithCacheSize(1024),
		dclient.WithLogger(dlogger),
	}

	log.Info("drand beacon without pubsub")

	client, err := dclient.Wrap(clients, opts...)
	if err != nil {
		return nil, xerrors.Errorf("creating drand client: %v", err)
	}

	lc, err := lru.New(1024)
	if err != nil {
		return nil, err
	}

	db := &DrandBeacon{
		client:     client,
		localCache: lc,
	}

	db.pubkey = drandChain.PublicKey
	db.interval = drandChain.Period
	db.drandGenTime = uint64(drandChain.GenesisTime)
	db.filRoundTime = interval
	db.filGenTime = genTimeStamp

	return db, nil
}

//Entry get a beacon value of specify block height,
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
			br.Err = xerrors.Errorf("drand failed Get request: %w", err)
		} else {
			br.Entry.Round = resp.Round()
			br.Entry.Data = resp.Signature()
		}
		log.Infow("done fetching randomness", "round", round, "took", time.Since(start))
		out <- br
		close(out)
	}()

	return out
}
func (db *DrandBeacon) cacheValue(e types.BeaconEntry) {
	db.localCache.Add(e.Round, e)
}

func (db *DrandBeacon) getCachedValue(round uint64) *types.BeaconEntry {
	v, ok := db.localCache.Get(round)
	if !ok {
		return nil
	}
	e, _ := v.(*types.BeaconEntry)
	return e
}

func (db *DrandBeacon) VerifyEntry(curr types.BeaconEntry, prev types.BeaconEntry) error {
	if prev.Round == 0 {
		// TODO handle genesis better
		return nil
	}
	if be := db.getCachedValue(curr.Round); be != nil {
		// return no error if the value is in the cache already
		return nil
	}
	b := &dchain.Beacon{
		PreviousSig: prev.Data,
		Round:       curr.Round,
		Signature:   curr.Data,
	}
	err := dchain.VerifyBeacon(db.pubkey, b)
	if err == nil {
		db.cacheValue(curr)
	}
	return err
}

// MaxBeaconRoundForEpoch get the turn of beacon chain corresponding to chain height
func (db *DrandBeacon) MaxBeaconRoundForEpoch(filEpoch abi.ChainEpoch) uint64 {
	// TODO: sometimes the genesis time for filecoin is zero and this goes negative
	latestTS := ((uint64(filEpoch) * db.filRoundTime) + db.filGenTime) - db.filRoundTime
	dround := (latestTS - db.drandGenTime) / uint64(db.interval.Seconds())
	return dround
}

var _ RandomBeacon = (*DrandBeacon)(nil)
