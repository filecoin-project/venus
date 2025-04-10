// FETCHED FROM LOTUS: builtin/multisig/state.go.template

package multisig

import (
	"bytes"
	"encoding/binary"
	"fmt"

	actorstypes "github.com/filecoin-project/go-state-types/actors"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/go-state-types/manifest"
	"github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/actors/adt"

	builtin16 "github.com/filecoin-project/go-state-types/builtin"
	msig16 "github.com/filecoin-project/go-state-types/builtin/v16/multisig"
	adt16 "github.com/filecoin-project/go-state-types/builtin/v16/util/adt"
)

var _ State = (*state16)(nil)

func load16(store adt.Store, root cid.Cid) (State, error) {
	out := state16{store: store}
	err := store.Get(store.Context(), root, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func make16(store adt.Store, signers []address.Address, threshold uint64, startEpoch abi.ChainEpoch, unlockDuration abi.ChainEpoch, initialBalance abi.TokenAmount) (State, error) {
	out := state16{store: store}
	out.State = msig16.State{}
	out.State.Signers = signers
	out.State.NumApprovalsThreshold = threshold
	out.State.StartEpoch = startEpoch
	out.State.UnlockDuration = unlockDuration
	out.State.InitialBalance = initialBalance

	em, err := adt16.StoreEmptyMap(store, builtin16.DefaultHamtBitwidth)
	if err != nil {
		return nil, err
	}

	out.State.PendingTxns = em

	return &out, nil
}

type state16 struct {
	msig16.State
	store adt.Store
}

func (s *state16) LockedBalance(currEpoch abi.ChainEpoch) (abi.TokenAmount, error) {
	return s.State.AmountLocked(currEpoch - s.State.StartEpoch), nil
}

func (s *state16) StartEpoch() (abi.ChainEpoch, error) {
	return s.State.StartEpoch, nil
}

func (s *state16) UnlockDuration() (abi.ChainEpoch, error) {
	return s.State.UnlockDuration, nil
}

func (s *state16) InitialBalance() (abi.TokenAmount, error) {
	return s.State.InitialBalance, nil
}

func (s *state16) Threshold() (uint64, error) {
	return s.State.NumApprovalsThreshold, nil
}

func (s *state16) Signers() ([]address.Address, error) {
	return s.State.Signers, nil
}

func (s *state16) ForEachPendingTxn(cb func(id int64, txn Transaction) error) error {
	arr, err := adt16.AsMap(s.store, s.State.PendingTxns, builtin16.DefaultHamtBitwidth)
	if err != nil {
		return err
	}
	var out msig16.Transaction
	return arr.ForEach(&out, func(key string) error {
		txid, n := binary.Varint([]byte(key))
		if n <= 0 {
			return fmt.Errorf("invalid pending transaction key: %v", key)
		}
		return cb(txid, (Transaction)(out)) //nolint:unconvert
	})
}

func (s *state16) PendingTxnChanged(other State) (bool, error) {
	other16, ok := other.(*state16)
	if !ok {
		// treat an upgrade as a change, always
		return true, nil
	}
	return !s.State.PendingTxns.Equals(other16.PendingTxns), nil
}

func (s *state16) transactions() (adt.Map, error) {
	return adt16.AsMap(s.store, s.PendingTxns, builtin16.DefaultHamtBitwidth)
}

func (s *state16) decodeTransaction(val *cbg.Deferred) (Transaction, error) {
	var tx msig16.Transaction
	if err := tx.UnmarshalCBOR(bytes.NewReader(val.Raw)); err != nil {
		return Transaction{}, err
	}
	return Transaction(tx), nil
}

func (s *state16) GetState() interface{} {
	return &s.State
}

func (s *state16) ActorKey() string {
	return manifest.MultisigKey
}

func (s *state16) ActorVersion() actorstypes.Version {
	return actorstypes.Version16
}

func (s *state16) Code() cid.Cid {
	code, ok := actors.GetActorCodeID(s.ActorVersion(), s.ActorKey())
	if !ok {
		panic(fmt.Errorf("didn't find actor %v code id for actor version %d", s.ActorKey(), s.ActorVersion()))
	}

	return code
}
