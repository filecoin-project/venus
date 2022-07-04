package vmsupport

import (
	"context"
	"errors"
	"fmt"
	goruntime "runtime"
	"sync"

	"github.com/ipfs/go-cid"
	cbornode "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log/v2"
	"github.com/minio/blake2b-simd"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"

	vmr "github.com/filecoin-project/specs-actors/v7/actors/runtime"
	proof7 "github.com/filecoin-project/specs-actors/v7/actors/runtime/proof"

	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/state"
	"github.com/filecoin-project/venus/pkg/util/ffiwrapper"
	"github.com/filecoin-project/venus/pkg/util/ffiwrapper/impl"
	"github.com/filecoin-project/venus/pkg/vm"
	"github.com/filecoin-project/venus/pkg/vm/vmcontext"
)

var log = logging.Logger("vmsupport")

type faultChecker interface {
	VerifyConsensusFault(ctx context.Context, h1, h2, extra []byte, curEpoch abi.ChainEpoch, msg vm.VmMessage, gasIpld cbornode.IpldStore, view vm.SyscallsStateView, getter vmcontext.LookbackStateGetter) (*vmr.ConsensusFault, error)
}

// Syscalls contains the concrete implementation of LegacyVM system calls, including connection to
// proof verification and blockchain inspection.
// Errors returned by these methods are intended to be returned to the actor code to respond to: they must be
// entirely deterministic and repeatable by other implementations.
// Any non-deterministic error will instead trigger a panic.
// TODO: determine a more robust mechanism for distinguishing transient runtime failures from deterministic errors
// in LegacyVM and supporting code. https://github.com/filecoin-project/venus/issues/3844
type Syscalls struct {
	faultChecker faultChecker
	verifier     ffiwrapper.Verifier
}

func NewSyscalls(faultChecker faultChecker, verifier ffiwrapper.Verifier) *Syscalls {
	return &Syscalls{
		faultChecker: faultChecker,
		verifier:     verifier,
	}
}

func (s *Syscalls) VerifyReplicaUpdate(update proof7.ReplicaUpdateInfo) error {
	ok, err := s.verifier.VerifyReplicaUpdate(update)
	if err != nil {
		return fmt.Errorf("failed to verify replica update: %w", err)
	}

	if !ok {
		return fmt.Errorf("invalid replica update")
	}

	return nil
}

// VerifySignature Verifies that a signature is valid for an address and plaintext.
func (s *Syscalls) VerifySignature(ctx context.Context, view vm.SyscallsStateView, signature crypto.Signature, signer address.Address, plaintext []byte) error {
	return state.NewSignatureValidator(view).ValidateSignature(ctx, plaintext, signer, signature)
}

// HashBlake2b Hashes input data using blake2b with 256 bit output.
func (s *Syscalls) HashBlake2b(data []byte) [32]byte {
	return blake2b.Sum256(data)
}

//ComputeUnsealedSectorCID Computes an unsealed sector CID (CommD) from its constituent piece CIDs (CommPs) and sizes.
func (s *Syscalls) ComputeUnsealedSectorCID(_ context.Context, proof abi.RegisteredSealProof, pieces []abi.PieceInfo) (cid.Cid, error) {
	return impl.GenerateUnsealedCID(proof, pieces)
}

// VerifySeal returns true if the sealing operation from which its inputs were
// derived was valid, and false if not.
func (s *Syscalls) VerifySeal(_ context.Context, info proof7.SealVerifyInfo) error {
	ok, err := s.verifier.VerifySeal(info)
	if err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("seal invalid")
	}
	return nil
}

var BatchSealVerifyParallelism = 2 * goruntime.NumCPU()

//BatchVerifySeals batch verify windows post
func (s *Syscalls) BatchVerifySeals(ctx context.Context, vis map[address.Address][]proof7.SealVerifyInfo) (map[address.Address][]bool, error) {
	out := make(map[address.Address][]bool)

	sema := make(chan struct{}, BatchSealVerifyParallelism)

	var wg sync.WaitGroup
	for addr, seals := range vis {
		results := make([]bool, len(seals))
		out[addr] = results

		for i, seal := range seals {
			wg.Add(1)
			go func(ma address.Address, ix int, svi proof7.SealVerifyInfo, res []bool) {
				defer wg.Done()
				sema <- struct{}{}

				if err := s.VerifySeal(ctx, svi); err != nil {
					log.Warnw("seal verify in batch failed", "miner", ma, "index", ix, "err", err)
					res[ix] = false
				} else {
					res[ix] = true
				}

				<-sema
			}(addr, i, seal, results)
		}
	}
	wg.Wait()

	return out, nil
}

func (s *Syscalls) VerifyAggregateSeals(aggregate proof7.AggregateSealVerifyProofAndInfos) error {
	ok, err := s.verifier.VerifyAggregateSeals(aggregate)
	if err != nil {
		return fmt.Errorf("failed to verify aggregated PoRep: %w", err)
	}
	if !ok {
		return fmt.Errorf("invalid aggregate proof")
	}

	return nil
}

//VerifyPoSt verify windows post
func (s *Syscalls) VerifyPoSt(ctx context.Context, info proof7.WindowPoStVerifyInfo) error {
	ok, err := s.verifier.VerifyWindowPoSt(ctx, info)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("window PoSt verification failed")
	}
	return nil
}

// Verifies that two block headers provide proof of a consensus fault:
// - both headers mined by the same actor
// - headers are different
// - first header is of the same or lower epoch as the second
// - at least one of the headers appears in the current chain at or after epoch `earliest`
// - the headers provide evidence of a fault (see the spec for the different fault types).
// The parameters are all serialized block headers. The third "extra" parameter is consulted only for
// the "parent grinding fault", in which case it must be the sibling of h1 (same parent tipset) and one of the
// blocks in the parent of h2 (i.e. h2's grandparent).
// Returns nil and an error if the headers don't prove a fault.
func (s *Syscalls) VerifyConsensusFault(ctx context.Context, h1, h2, extra []byte, curEpoch abi.ChainEpoch, msg vm.VmMessage, gasIpld cbornode.IpldStore, view vm.SyscallsStateView, getter vmcontext.LookbackStateGetter) (*vmr.ConsensusFault, error) {
	return s.faultChecker.VerifyConsensusFault(ctx, h1, h2, extra, curEpoch, msg, gasIpld, view, getter)
}
