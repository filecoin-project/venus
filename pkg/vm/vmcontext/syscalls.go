package vmcontext

import (
	"context"
	goruntime "runtime"
	"sync"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
	specsruntime "github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/filecoin-project/specs-actors/actors/runtime/proof"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/miner"
	"github.com/filecoin-project/venus/pkg/state"
	"github.com/filecoin-project/venus/pkg/vm/gas"
	vmState "github.com/filecoin-project/venus/pkg/vm/state"
)

type SyscallsStateView interface {
	state.AccountStateView
	MinerInfo(ctx context.Context, maddr address.Address, nv network.Version) (*miner.MinerInfo, error)
	TotalFilCircSupply(height abi.ChainEpoch, st vmState.Tree) (abi.TokenAmount, error)
	GetNtwkVersion(ctx context.Context, ce abi.ChainEpoch) network.Version
}

// Syscall implementation interface.
// These methods take the chain epoch and other context that is implicit in the runtime as explicit parameters.
type SyscallsImpl interface {
	VerifySignature(ctx context.Context, view SyscallsStateView, signature crypto.Signature, signer address.Address, plaintext []byte) error
	HashBlake2b(data []byte) [32]byte
	ComputeUnsealedSectorCID(ctx context.Context, proof abi.RegisteredSealProof, pieces []abi.PieceInfo) (cid.Cid, error)
	VerifySeal(ctx context.Context, info proof.SealVerifyInfo) error
	BatchVerifySeals(ctx context.Context, vis map[address.Address][]proof.SealVerifyInfo) (map[address.Address][]bool, error)
	VerifyPoSt(ctx context.Context, info proof.WindowPoStVerifyInfo) error
	VerifyConsensusFault(ctx context.Context, h1, h2, extra []byte, view SyscallsStateView) (*specsruntime.ConsensusFault, error)
}

type syscalls struct {
	impl      SyscallsImpl
	ctx       context.Context
	gasTank   *gas.GasTracker
	pricelist gas.Pricelist
	stateView SyscallsStateView
}

var _ specsruntime.Syscalls = (*syscalls)(nil)

func (sys syscalls) VerifySignature(signature crypto.Signature, signer address.Address, plaintext []byte) error {
	charge, err := sys.pricelist.OnVerifySignature(signature.Type, len(plaintext))
	if err != nil {
		return err
	}
	sys.gasTank.Charge(charge, "VerifySignature")
	return sys.impl.VerifySignature(sys.ctx, sys.stateView, signature, signer, plaintext)
}

func (sys syscalls) HashBlake2b(data []byte) [32]byte {
	sys.gasTank.Charge(sys.pricelist.OnHashing(len(data)), "HashBlake2b")
	return sys.impl.HashBlake2b(data)
}

func (sys syscalls) ComputeUnsealedSectorCID(proof abi.RegisteredSealProof, pieces []abi.PieceInfo) (cid.Cid, error) {
	sys.gasTank.Charge(sys.pricelist.OnComputeUnsealedSectorCid(proof, pieces), "ComputeUnsealedSectorCID")
	return sys.impl.ComputeUnsealedSectorCID(sys.ctx, proof, pieces)
}

func (sys syscalls) VerifySeal(info proof.SealVerifyInfo) error {
	sys.gasTank.Charge(sys.pricelist.OnVerifySeal(info), "VerifySeal")
	return sys.impl.VerifySeal(sys.ctx, info)
}

func (sys syscalls) VerifyPoSt(info proof.WindowPoStVerifyInfo) error {
	sys.gasTank.Charge(sys.pricelist.OnVerifyPost(info), "VerifyWindowPoSt")
	return sys.impl.VerifyPoSt(sys.ctx, info)
}

func (sys syscalls) VerifyConsensusFault(h1, h2, extra []byte) (*specsruntime.ConsensusFault, error) {
	sys.gasTank.Charge(sys.pricelist.OnVerifyConsensusFault(), "VerifyConsensusFault")
	return sys.impl.VerifyConsensusFault(sys.ctx, h1, h2, extra, sys.stateView)
}

var BatchSealVerifyParallelism = 2 * goruntime.NumCPU()

func (sys syscalls) BatchVerifySeals(vis map[address.Address][]proof.SealVerifyInfo) (map[address.Address][]bool, error) {
	out := make(map[address.Address][]bool)

	sema := make(chan struct{}, BatchSealVerifyParallelism)

	var wg sync.WaitGroup
	for addr, seals := range vis {
		results := make([]bool, len(seals))
		out[addr] = results

		for i, s := range seals {
			wg.Add(1)
			go func(ma address.Address, ix int, svi proof.SealVerifyInfo, res []bool) {
				defer wg.Done()
				sema <- struct{}{}

				if err := sys.VerifySeal(svi); err != nil {
					vmlog.Warnw("seal verify in batch failed", "miner", ma, "index", ix, "err", err)
					res[ix] = false
				} else {
					res[ix] = true
				}

				<-sema
			}(addr, i, s, results)
		}
	}
	wg.Wait()

	return out, nil
}
