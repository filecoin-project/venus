package paych

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	builtin0 "github.com/filecoin-project/specs-actors/actors/builtin"
	init0 "github.com/filecoin-project/specs-actors/actors/builtin/init"
	paych0 "github.com/filecoin-project/specs-actors/actors/builtin/paych"

	"github.com/filecoin-project/venus/internal/pkg/specactors"
	init_ "github.com/filecoin-project/venus/internal/pkg/specactors/builtin/init"
	"github.com/filecoin-project/venus/internal/pkg/types"
)

type message0 struct{ from address.Address }

func (m message0) Create(to address.Address, initialAmount abi.TokenAmount) (*types.UnsignedMessage, error) {
	params, aerr := specactors.SerializeParams(&paych0.ConstructorParams{From: m.from, To: to})
	if aerr != nil {
		return nil, aerr
	}
	enc, aerr := specactors.SerializeParams(&init0.ExecParams{
		CodeCID:           builtin0.PaymentChannelActorCodeID,
		ConstructorParams: params,
	})
	if aerr != nil {
		return nil, aerr
	}

	return &types.UnsignedMessage{
		To:     init_.Address,
		From:   m.from,
		Value:  initialAmount,
		Method: builtin0.MethodsInit.Exec,
		Params: enc,
	}, nil
}

func (m message0) Update(paych address.Address, sv *SignedVoucher, secret []byte) (*types.UnsignedMessage, error) {
	params, aerr := specactors.SerializeParams(&paych0.UpdateChannelStateParams{
		Sv:     *sv,
		Secret: secret,
	})
	if aerr != nil {
		return nil, aerr
	}

	return &types.UnsignedMessage{
		To:     paych,
		From:   m.from,
		Value:  abi.NewTokenAmount(0),
		Method: builtin0.MethodsPaych.UpdateChannelState,
		Params: params,
	}, nil
}

func (m message0) Settle(paych address.Address) (*types.UnsignedMessage, error) {
	return &types.UnsignedMessage{
		To:     paych,
		From:   m.from,
		Value:  abi.NewTokenAmount(0),
		Method: builtin0.MethodsPaych.Settle,
	}, nil
}

func (m message0) Collect(paych address.Address) (*types.UnsignedMessage, error) {
	return &types.UnsignedMessage{
		To:     paych,
		From:   m.from,
		Value:  abi.NewTokenAmount(0),
		Method: builtin0.MethodsPaych.Collect,
	}, nil
}
