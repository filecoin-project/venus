package conformance

import (
	"context"
	"fmt"
	"sync"

	"github.com/filecoin-project/venus/pkg/vm/vmcontext"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/crypto"

	"github.com/filecoin-project/test-vectors/schema"
)

type RecordingRand struct {
	reporter Reporter
	api      v1api.FullNode
	// once guards the loading of the head tipset.
	// can be removed when https://github.com/filecoin-project/lotus/issues/4223
	// is fixed.
	once     sync.Once
	head     types.TipSetKey
	lk       sync.Mutex
	recorded schema.Randomness
}

func (r *RecordingRand) ChainGetRandomnessFromBeacon(ctx context.Context, pers crypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	return r.getChainRandomness(ctx, pers, round, entropy)
}

func (r *RecordingRand) ChainGetRandomnessFromTickets(ctx context.Context, pers crypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	return r.getChainRandomness(ctx, pers, round, entropy)
}

var _ vmcontext.HeadChainRandomness = (*RecordingRand)(nil)

// NewRecordingRand returns a vm.Rand implementation that proxies calls to a
// full Lotus node via JSON-RPC, and records matching rules and responses so
// they can later be embedded in test vectors.
func NewRecordingRand(reporter Reporter, api v1api.FullNode) *RecordingRand {
	return &RecordingRand{reporter: reporter, api: api}
}

func (r *RecordingRand) loadHead() {
	head, err := r.api.ChainHead(context.TODO())
	if err != nil {
		panic(fmt.Sprintf("could not fetch chain head while fetching randomness: %s", err))
	}
	r.head = head.Key()
}

func (r *RecordingRand) GetChainRandomnessV2(ctx context.Context, pers crypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error) {
	return r.getChainRandomness(ctx, pers, round, entropy)
}

func (r *RecordingRand) GetChainRandomnessV1(ctx context.Context, pers crypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error) {
	return r.getChainRandomness(ctx, pers, round, entropy)
}

func (r *RecordingRand) getChainRandomness(ctx context.Context, pers crypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error) {
	r.once.Do(r.loadHead)
	// FullNode's StateGetRandomnessFromTickets handles whether we should be looking forward or back
	ret, err := r.api.StateGetRandomnessFromTickets(ctx, pers, round, entropy, r.head)
	if err != nil {
		return ret, err
	}

	r.reporter.Logf("fetched and recorded chain randomness for: dst=%d, epoch=%d, entropy=%x, result=%x", pers, round, entropy, ret)

	match := schema.RandomnessMatch{
		On: schema.RandomnessRule{
			Kind:                schema.RandomnessChain,
			DomainSeparationTag: int64(pers),
			Epoch:               int64(round),
			Entropy:             entropy,
		},
		Return: []byte(ret),
	}
	r.lk.Lock()
	r.recorded = append(r.recorded, match)
	r.lk.Unlock()

	return ret, err
}

func (r *RecordingRand) GetBeaconRandomnessV3(ctx context.Context, pers crypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error) {
	return r.getBeaconRandomness(ctx, pers, round, entropy)
}

func (r *RecordingRand) GetBeaconRandomnessV1(ctx context.Context, pers crypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error) {
	return r.getBeaconRandomness(ctx, pers, round, entropy)
}

func (r *RecordingRand) GetBeaconRandomnessV2(ctx context.Context, pers crypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error) {
	return r.getBeaconRandomness(ctx, pers, round, entropy)
}

func (r *RecordingRand) getBeaconRandomness(ctx context.Context, pers crypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error) {
	r.once.Do(r.loadHead)
	ret, err := r.api.StateGetRandomnessFromBeacon(ctx, pers, round, entropy, r.head)
	if err != nil {
		return ret, err
	}

	r.reporter.Logf("fetched and recorded beacon randomness for: dst=%d, epoch=%d, entropy=%x, result=%x", pers, round, entropy, ret)

	match := schema.RandomnessMatch{
		On: schema.RandomnessRule{
			Kind:                schema.RandomnessBeacon,
			DomainSeparationTag: int64(pers),
			Epoch:               int64(round),
			Entropy:             entropy,
		},
		Return: []byte(ret),
	}
	r.lk.Lock()
	r.recorded = append(r.recorded, match)
	r.lk.Unlock()

	return ret, err
}

func (r *RecordingRand) Recorded() schema.Randomness {
	r.lk.Lock()
	defer r.lk.Unlock()

	return r.recorded
}
