package paych

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"

	builtin4 "github.com/filecoin-project/specs-actors/v4/actors/builtin"
	init4 "github.com/filecoin-project/specs-actors/v4/actors/builtin/init"
	paych4 "github.com/filecoin-project/specs-actors/v4/actors/builtin/paych"

	"github.com/filecoin-project/venus/pkg/types/internal"
	"github.com/filecoin-project/venus/pkg/types/specactors"
	init_ "github.com/filecoin-project/venus/pkg/types/specactors/builtin/init"
)

type message4 struct{ from address.Address }

func (m message4) Create(to address.Address, initialAmount abi.TokenAmount) (*internal.Message, error) {
	params, aerr := specactors.SerializeParams(&paych4.ConstructorParams{From: m.from, To: to})
	if aerr != nil {
		return nil, aerr
	}
	enc, aerr := specactors.SerializeParams(&init4.ExecParams{
		CodeCID:           builtin4.PaymentChannelActorCodeID,
		ConstructorParams: params,
	})
	if aerr != nil {
		return nil, aerr
	}

	return &internal.Message{
		To:     init_.Address,
		From:   m.from,
		Value:  initialAmount,
		Method: builtin4.MethodsInit.Exec,
		Params: enc,
	}, nil
}

func (m message4) Update(paych address.Address, sv *SignedVoucher, secret []byte) (*internal.Message, error) {
	params, aerr := specactors.SerializeParams(&paych4.UpdateChannelStateParams{
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
		Method: builtin4.MethodsPaych.UpdateChannelState,
		Params: params,
	}, nil
}

func (m message4) Settle(paych address.Address) (*internal.Message, error) {
	return &internal.Message{
		To:     paych,
		From:   m.from,
		Value:  abi.NewTokenAmount(0),
		Method: builtin4.MethodsPaych.Settle,
	}, nil
}

func (m message4) Collect(paych address.Address) (*internal.Message, error) {
	return &internal.Message{
		To:     paych,
		From:   m.from,
		Value:  abi.NewTokenAmount(0),
		Method: builtin4.MethodsPaych.Collect,
	}, nil
}
