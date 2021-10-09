package paych

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"

	builtin6 "github.com/filecoin-project/specs-actors/v6/actors/builtin"
	init6 "github.com/filecoin-project/specs-actors/v6/actors/builtin/init"
	paych6 "github.com/filecoin-project/specs-actors/v6/actors/builtin/paych"

	"github.com/filecoin-project/venus/pkg/types/internal"
	"github.com/filecoin-project/venus/pkg/types/specactors"
	init_ "github.com/filecoin-project/venus/pkg/types/specactors/builtin/init"
)

type message6 struct{ from address.Address }

func (m message6) Create(to address.Address, initialAmount abi.TokenAmount) (*internal.Message, error) {
	params, aerr := specactors.SerializeParams(&paych6.ConstructorParams{From: m.from, To: to})
	if aerr != nil {
		return nil, aerr
	}
	enc, aerr := specactors.SerializeParams(&init6.ExecParams{
		CodeCID:           builtin6.PaymentChannelActorCodeID,
		ConstructorParams: params,
	})
	if aerr != nil {
		return nil, aerr
	}

	return &internal.Message{
		To:     init_.Address,
		From:   m.from,
		Value:  initialAmount,
		Method: builtin6.MethodsInit.Exec,
		Params: enc,
	}, nil
}

func (m message6) Update(paych address.Address, sv *SignedVoucher, secret []byte) (*internal.Message, error) {
	params, aerr := specactors.SerializeParams(&paych6.UpdateChannelStateParams{
		Sv:     *sv,
		Secret: secret,
	})
	if aerr != nil {
		return nil, aerr
	}

	return &internal.Message{
		To:     paych,
		From:   m.from,
		Value:  abi.NewTokenAmount(0),
		Method: builtin6.MethodsPaych.UpdateChannelState,
		Params: params,
	}, nil
}

func (m message6) Settle(paych address.Address) (*internal.Message, error) {
	return &internal.Message{
		To:     paych,
		From:   m.from,
		Value:  abi.NewTokenAmount(0),
		Method: builtin6.MethodsPaych.Settle,
	}, nil
}

func (m message6) Collect(paych address.Address) (*internal.Message, error) {
	return &internal.Message{
		To:     paych,
		From:   m.from,
		Value:  abi.NewTokenAmount(0),
		Method: builtin6.MethodsPaych.Collect,
	}, nil
}
