package vmcontext

import (
	"context"
	goruntime "runtime"
	"sync"

	cbornode "github.com/ipfs/go-ipld-cbor"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
	proof7 "github.com/filecoin-project/specs-actors/v7/actors/runtime/proof"
	"github.com/ipfs/go-cid"

	/* inline-gen template
	{{range .actorVersions}}
	rt{{.}} "github.com/filecoin-project/specs-actors{{import .}}actors/runtime"{{end}}

	/* inline-gen start */

	rt0 "github.com/filecoin-project/specs-actors/actors/runtime"
	rt2 "github.com/filecoin-project/specs-actors/v2/actors/runtime"
	rt3 "github.com/filecoin-project/specs-actors/v3/actors/runtime"
	rt4 "github.com/filecoin-project/specs-actors/v4/actors/runtime"
	rt5 "github.com/filecoin-project/specs-actors/v5/actors/runtime"
	rt6 "github.com/filecoin-project/specs-actors/v6/actors/runtime"
	rt7 "github.com/filecoin-project/specs-actors/v7/actors/runtime"
	rt8 "github.com/filecoin-project/specs-actors/v8/actors/runtime"

	/* inline-gen end */

	"github.com/filecoin-project/venus/pkg/crypto"
	vmState "github.com/filecoin-project/venus/pkg/state/tree"
	"github.com/filecoin-project/venus/pkg/vm/gas"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/miner"
)

type SyscallsStateView interface {
	ResolveToKeyAddr(ctx context.Context, address address.Address) (address.Address, error)
	MinerInfo(ctx context.Context, maddr address.Address, nv network.Version) (*miner.MinerInfo, error)
	TotalFilCircSupply(height abi.ChainEpoch, st vmState.Tree) (abi.TokenAmount, error)
	GetNetworkVersion(ctx context.Context, ce abi.ChainEpoch) network.Version
}

// Syscall implementation interface.
// These methods take the chain epoch and other context that is implicit in the runtime as explicit parameters.
type SyscallsImpl interface {
	VerifySignature(ctx context.Context, view SyscallsStateView, signature crypto.Signature, signer address.Address, plaintext []byte) error
	HashBlake2b(data []byte) [32]byte
	ComputeUnsealedSectorCID(ctx context.Context, proof7 abi.RegisteredSealProof, pieces []abi.PieceInfo) (cid.Cid, error)
	VerifySeal(ctx context.Context, info proof7.SealVerifyInfo) error
	BatchVerifySeals(ctx context.Context, vis map[address.Address][]proof7.SealVerifyInfo) (map[address.Address][]bool, error)
	VerifyAggregateSeals(aggregate proof7.AggregateSealVerifyProofAndInfos) error
	VerifyReplicaUpdate(update proof7.ReplicaUpdateInfo) error
	VerifyPoSt(ctx context.Context, info proof7.WindowPoStVerifyInfo) error
	VerifyConsensusFault(ctx context.Context, h1, h2, extra []byte, curEpoch abi.ChainEpoch, msg VmMessage, gasIpld cbornode.IpldStore, view SyscallsStateView, getter LookbackStateGetter) (*rt7.ConsensusFault, error)
}

type syscalls struct {
	impl          SyscallsImpl
	vm            *LegacyVM
	gasBlockStore cbornode.IpldStore
	vmMsg         VmMessage
	gasTank       *gas.GasTracker
	pricelist     gas.Pricelist
	stateView     SyscallsStateView
}

/* inline-gen template
{{range .actorVersions}}
var _ rt{{.}}.Syscalls = (*syscalls)(nil){{end}}
/* inline-gen start */

var _ rt0.Syscalls = (*syscalls)(nil)
var _ rt2.Syscalls = (*syscalls)(nil)
var _ rt3.Syscalls = (*syscalls)(nil)
var _ rt4.Syscalls = (*syscalls)(nil)
var _ rt5.Syscalls = (*syscalls)(nil)
var _ rt6.Syscalls = (*syscalls)(nil)
var _ rt7.Syscalls = (*syscalls)(nil)
var _ rt8.Syscalls = (*syscalls)(nil)

/* inline-gen end */

func (sys syscalls) VerifySignature(signature crypto.Signature, signer address.Address, plaintext []byte) error {
	charge, err := sys.pricelist.OnVerifySignature(signature.Type, len(plaintext))
	if err != nil {
		return err
	}
	sys.gasTank.Charge(charge, "VerifySignature")
	return sys.impl.VerifySignature(sys.vm.context, sys.stateView, signature, signer, plaintext)
}

func (sys syscalls) HashBlake2b(data []byte) [32]byte {
	sys.gasTank.Charge(sys.pricelist.OnHashing(len(data)), "HashBlake2b")
	return sys.impl.HashBlake2b(data)
}

func (sys syscalls) ComputeUnsealedSectorCID(proof abi.RegisteredSealProof, pieces []abi.PieceInfo) (cid.Cid, error) {
	sys.gasTank.Charge(sys.pricelist.OnComputeUnsealedSectorCid(proof, pieces), "ComputeUnsealedSectorCID")
	return sys.impl.ComputeUnsealedSectorCID(sys.vm.context, proof, pieces)
}

func (sys syscalls) VerifySeal(info proof7.SealVerifyInfo) error {
	sys.gasTank.Charge(sys.pricelist.OnVerifySeal(info), "VerifySeal")
	return sys.impl.VerifySeal(sys.vm.context, info)
}

func (sys syscalls) VerifyPoSt(info proof7.WindowPoStVerifyInfo) error {
	sys.gasTank.Charge(sys.pricelist.OnVerifyPost(info), "VerifyWindowPoSt")
	return sys.impl.VerifyPoSt(sys.vm.context, info)
}

func (sys syscalls) VerifyConsensusFault(h1, h2, extra []byte) (*rt7.ConsensusFault, error) {
	sys.gasTank.Charge(sys.pricelist.OnVerifyConsensusFault(), "VerifyConsensusFault")
	return sys.impl.VerifyConsensusFault(sys.vm.context, h1, h2, extra, sys.vm.currentEpoch, sys.vmMsg, sys.gasBlockStore, sys.stateView, sys.vm.vmOption.LookbackStateGetter)
}

var BatchSealVerifyParallelism = 2 * goruntime.NumCPU()

func (sys syscalls) BatchVerifySeals(vis map[address.Address][]proof7.SealVerifyInfo) (map[address.Address][]bool, error) {
	out := make(map[address.Address][]bool)

	sema := make(chan struct{}, BatchSealVerifyParallelism)
	vmlog.Info("BatchVerifySeals miners:", len(vis))
	var wg sync.WaitGroup
	for addr, seals := range vis {
		results := make([]bool, len(seals))
		out[addr] = results

		for i, s := range seals {
			wg.Add(1)
			go func(ma address.Address, ix int, svi proof7.SealVerifyInfo, res []bool) {
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
	vmlog.Info("BatchVerifySeals Result miners:", len(out))
	return out, nil
}

func (sys *syscalls) VerifyAggregateSeals(aggregate proof7.AggregateSealVerifyProofAndInfos) error {
	sys.gasTank.Charge(sys.pricelist.OnVerifyAggregateSeals(aggregate), "VerifyAggregateSeals")
	return sys.impl.VerifyAggregateSeals(aggregate)
}

func (sys *syscalls) VerifyReplicaUpdate(update proof7.ReplicaUpdateInfo) error {
	sys.gasTank.Charge(sys.pricelist.OnVerifyReplicaUpdate(update), "OnVerifyReplicaUpdate")
	return sys.impl.VerifyReplicaUpdate(update)
}
