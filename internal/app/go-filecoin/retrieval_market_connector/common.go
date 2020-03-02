package retrievalmarketconnector

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	paychActor "github.com/filecoin-project/specs-actors/actors/builtin/paych"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paymentchannel"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
)

// ChainReaderAPI is the subset of the Wallet interface needed by the retrieval client node
type ChainReaderAPI interface {
	// GetBalance gets the balance in AttoFIL for a given address
	Head() block.TipSetKey
	GetTipSet(key block.TipSetKey) (block.TipSet, error)
	GetActorAt(ctx context.Context, tipKey block.TipSetKey, addr address.Address) (*actor.Actor, error)
}

// RetrievalSigner is an interface with the ability to sign data
type RetrievalSigner interface {
	SignBytes(data []byte, addr address.Address) (crypto.Signature, error)
}

// PaychMgrAPI is an API used for communicating with payment channel actor and store.
type PaychMgrAPI interface {
	AllocateLane(paychAddr address.Address) (uint64, error)
	GetPaymentChannelInfo(paychAddr address.Address) (*paymentchannel.ChannelInfo, error)
	GetPaymentChannelByAccounts(payer, payee address.Address) (address.Address, *paymentchannel.ChannelInfo)
	CreatePaymentChannel(payer, payee address.Address) error
	CreateVoucher(paychAddr address.Address, voucher *paychActor.SignedVoucher) error
	SaveVoucher(paychAddr address.Address, voucher *paychActor.SignedVoucher, proof []byte, expected abi.TokenAmount) (actual abi.TokenAmount, err error)
}
