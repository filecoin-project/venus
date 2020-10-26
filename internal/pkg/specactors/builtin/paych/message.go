package paych

import (
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/specactors"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-state-types/abi"
)

func Message(version specactors.Version, from address.Address) MessageBuilder {
	switch version {
	case specactors.Version0:
		return message0{from}
	case specactors.Version2:
		return message2{from}
	default:
		panic(fmt.Sprintf("unsupported actors version: %d", version))
	}
}

type MessageBuilder interface {
	Create(to address.Address, initialAmount abi.TokenAmount) (*types.UnsignedMessage, error)
	Update(paych address.Address, voucher *SignedVoucher, secret []byte) (*types.UnsignedMessage, error)
	Settle(paych address.Address) (*types.UnsignedMessage, error)
	Collect(paych address.Address) (*types.UnsignedMessage, error)
}
