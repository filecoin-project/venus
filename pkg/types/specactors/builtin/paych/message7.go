// FETCHED FROM LOTUS: builtin/paych/message.go.template

package paych

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"

	builtin7 "github.com/filecoin-project/specs-actors/v7/actors/builtin"
	init7 "github.com/filecoin-project/specs-actors/v7/actors/builtin/init"
	paych7 "github.com/filecoin-project/specs-actors/v7/actors/builtin/paych"

	actors "github.com/filecoin-project/venus/pkg/types/specactors"
	init_ "github.com/filecoin-project/venus/pkg/types/specactors/builtin/init"
	types "github.com/filecoin-project/venus/pkg/types/internal"
)

type message7 struct{ from address.Address }

func (m message7) Create(to address.Address, initialAmount abi.TokenAmount) (*types.Message, error) {
	params, aerr := actors.SerializeParams(&paych7.ConstructorParams{From: m.from, To: to})
	if aerr != nil {
		return nil, aerr
	}
	enc, aerr := actors.SerializeParams(&init7.ExecParams{
		CodeCID:           builtin7.PaymentChannelActorCodeID,
		ConstructorParams: params,
	})
	if aerr != nil {
		return nil, aerr
	}

	return &types.Message{
		To:     init_.Address,
		From:   m.from,
		Value:  initialAmount,
		Method: builtin7.MethodsInit.Exec,
		Params: enc,
	}, nil
}

func (m message7) Update(paych address.Address, sv *SignedVoucher, secret []byte) (*types.Message, error) {
	params, aerr := actors.SerializeParams(&paych7.UpdateChannelStateParams{
		Sv:     *sv,
		Secret: secret,
	})
	if aerr != nil {
		return nil, aerr
	}

	return &types.Message{
		To:     paych,
		From:   m.from,
		Value:  abi.NewTokenAmount(0),
		Method: builtin7.MethodsPaych.UpdateChannelState,
		Params: params,
	}, nil
}

func (m message7) Settle(paych address.Address) (*types.Message, error) {
	return &types.Message{
		To:     paych,
		From:   m.from,
		Value:  abi.NewTokenAmount(0),
		Method: builtin7.MethodsPaych.Settle,
	}, nil
}

func (m message7) Collect(paych address.Address) (*types.Message, error) {
	return &types.Message{
		To:     paych,
		From:   m.from,
		Value:  abi.NewTokenAmount(0),
		Method: builtin7.MethodsPaych.Collect,
	}, nil
}