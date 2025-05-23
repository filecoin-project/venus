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

	builtin15 "github.com/filecoin-project/go-state-types/builtin"
	msig15 "github.com/filecoin-project/go-state-types/builtin/v15/multisig"
	adt15 "github.com/filecoin-project/go-state-types/builtin/v15/util/adt"
)

var _ State = (*state15)(nil)

func load15(store adt.Store, root cid.Cid) (State, error) {
	out := state15{store: store}
	err := store.Get(store.Context(), root, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func make15(store adt.Store, signers []address.Address, threshold uint64, startEpoch abi.ChainEpoch, unlockDuration abi.ChainEpoch, initialBalance abi.TokenAmount) (State, error) {
	out := state15{store: store}
	out.State = msig15.State{}
	out.State.Signers = signers
	out.State.NumApprovalsThreshold = threshold
	out.State.StartEpoch = startEpoch
	out.State.UnlockDuration = unlockDuration
	out.State.InitialBalance = initialBalance

	em, err := adt15.StoreEmptyMap(store, builtin15.DefaultHamtBitwidth)
	if err != nil {
		return nil, err
	}

	out.State.PendingTxns = em

	return &out, nil
}

type state15 struct {
	msig15.State
	store adt.Store
}

func (s *state15) LockedBalance(currEpoch abi.ChainEpoch) (abi.TokenAmount, error) {
	return s.State.AmountLocked(currEpoch - s.State.StartEpoch), nil
}

func (s *state15) StartEpoch() (abi.ChainEpoch, error) {
	return s.State.StartEpoch, nil
}

func (s *state15) UnlockDuration() (abi.ChainEpoch, error) {
	return s.State.UnlockDuration, nil
}

func (s *state15) InitialBalance() (abi.TokenAmount, error) {
	return s.State.InitialBalance, nil
}

func (s *state15) Threshold() (uint64, error) {
	return s.State.NumApprovalsThreshold, nil
}

func (s *state15) Signers() ([]address.Address, error) {
	return s.State.Signers, nil
}

func (s *state15) ForEachPendingTxn(cb func(id int64, txn Transaction) error) error {
	arr, err := adt15.AsMap(s.store, s.State.PendingTxns, builtin15.DefaultHamtBitwidth)
	if err != nil {
		return err
	}
	var out msig15.Transaction
	return arr.ForEach(&out, func(key string) error {
		txid, n := binary.Varint([]byte(key))
		if n <= 0 {
			return fmt.Errorf("invalid pending transaction key: %v", key)
		}
		return cb(txid, (Transaction)(out)) //nolint:unconvert
	})
}

func (s *state15) PendingTxnChanged(other State) (bool, error) {
	other15, ok := other.(*state15)
	if !ok {
		// treat an upgrade as a change, always
		return true, nil
	}
	return !s.State.PendingTxns.Equals(other15.PendingTxns), nil
}

func (s *state15) transactions() (adt.Map, error) {
	return adt15.AsMap(s.store, s.PendingTxns, builtin15.DefaultHamtBitwidth)
}

func (s *state15) decodeTransaction(val *cbg.Deferred) (Transaction, error) {
	var tx msig15.Transaction
	if err := tx.UnmarshalCBOR(bytes.NewReader(val.Raw)); err != nil {
		return Transaction{}, err
	}
	return Transaction(tx), nil
}

func (s *state15) GetState() interface{} {
	return &s.State
}

func (s *state15) ActorKey() string {
	return manifest.MultisigKey
}

func (s *state15) ActorVersion() actorstypes.Version {
	return actorstypes.Version15
}

func (s *state15) Code() cid.Cid {
	code, ok := actors.GetActorCodeID(s.ActorVersion(), s.ActorKey())
	if !ok {
		panic(fmt.Errorf("didn't find actor %v code id for actor version %d", s.ActorKey(), s.ActorVersion()))
	}

	return code
}
