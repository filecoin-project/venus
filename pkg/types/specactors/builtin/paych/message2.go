package paych

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"

	builtin2 "github.com/filecoin-project/specs-actors/v2/actors/builtin"
	init2 "github.com/filecoin-project/specs-actors/v2/actors/builtin/init"
	paych2 "github.com/filecoin-project/specs-actors/v2/actors/builtin/paych"

	"github.com/filecoin-project/venus/pkg/types/internal"
	"github.com/filecoin-project/venus/pkg/types/specactors"
	init_ "github.com/filecoin-project/venus/pkg/types/specactors/builtin/init"
)

type message2 struct{ from address.Address }

func (m message2) Create(to address.Address, initialAmount abi.TokenAmount) (*internal.Message, error) {
	params, aerr := specactors.SerializeParams(&paych2.ConstructorParams{From: m.from, To: to})
	if aerr != nil {
		return nil, aerr
	}
	enc, aerr := specactors.SerializeParams(&init2.ExecParams{
		CodeCID:           builtin2.PaymentChannelActorCodeID,
		ConstructorParams: params,
	})
	if aerr != nil {
		return nil, aerr
	}

	return &internal.Message{
		To:     init_.Address,
		From:   m.from,
		Value:  initialAmount,
		Method: builtin2.MethodsInit.Exec,
		Params: enc,
	}, nil
}

func (m message2) Update(paych address.Address, sv *SignedVoucher, secret []byte) (*internal.Message, error) {
	params, aerr := specactors.SerializeParams(&paych2.UpdateChannelStateParams{
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
		Method: builtin2.MethodsPaych.UpdateChannelState,
		Params: params,
	}, nil
}

func (m message2) Settle(paych address.Address) (*internal.Message, error) {
	return &internal.Message{
		To:     paych,
		From:   m.from,
		Value:  abi.NewTokenAmount(0),
		Method: builtin2.MethodsPaych.Settle,
	}, nil
}

func (m message2) Collect(paych address.Address) (*internal.Message, error) {
	return &internal.Message{
		To:     paych,
		From:   m.from,
		Value:  abi.NewTokenAmount(0),
		Method: builtin2.MethodsPaych.Collect,
	}, nil
}
