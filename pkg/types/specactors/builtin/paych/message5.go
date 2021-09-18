package paych

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"

	builtin5 "github.com/filecoin-project/specs-actors/v5/actors/builtin"
	init5 "github.com/filecoin-project/specs-actors/v5/actors/builtin/init"
	paych5 "github.com/filecoin-project/specs-actors/v5/actors/builtin/paych"

	"github.com/filecoin-project/venus/pkg/types/internal"
	"github.com/filecoin-project/venus/pkg/types/specactors"
	init_ "github.com/filecoin-project/venus/pkg/types/specactors/builtin/init"
)

type message5 struct{ from address.Address }

func (m message5) Create(to address.Address, initialAmount abi.TokenAmount) (*internal.Message, error) {
	params, aerr := specactors.SerializeParams(&paych5.ConstructorParams{From: m.from, To: to})
	if aerr != nil {
		return nil, aerr
	}
	enc, aerr := specactors.SerializeParams(&init5.ExecParams{
		CodeCID:           builtin5.PaymentChannelActorCodeID,
		ConstructorParams: params,
	})
	if aerr != nil {
		return nil, aerr
	}

	return &internal.Message{
		To:     init_.Address,
		From:   m.from,
		Value:  initialAmount,
		Method: builtin5.MethodsInit.Exec,
		Params: enc,
	}, nil
}

func (m message5) Update(paych address.Address, sv *SignedVoucher, secret []byte) (*internal.Message, error) {
	params, aerr := specactors.SerializeParams(&paych5.UpdateChannelStateParams{
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
		Method: builtin5.MethodsPaych.UpdateChannelState,
		Params: params,
	}, nil
}

func (m message5) Settle(paych address.Address) (*internal.Message, error) {
	return &internal.Message{
		To:     paych,
		From:   m.from,
		Value:  abi.NewTokenAmount(0),
		Method: builtin5.MethodsPaych.Settle,
	}, nil
}

func (m message5) Collect(paych address.Address) (*internal.Message, error) {
	return &internal.Message{
		To:     paych,
		From:   m.from,
		Value:  abi.NewTokenAmount(0),
		Method: builtin5.MethodsPaych.Collect,
	}, nil
}
