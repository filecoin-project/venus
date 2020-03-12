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
	ChannelExists(paychAddr address.Address) (bool, error)
	GetMinerWorker(ctx context.Context, miner address.Address) (address.Address, error)
	GetPaymentChannelInfo(paychAddr address.Address) (*paymentchannel.ChannelInfo, error)
	GetPaymentChannelByAccounts(payer, payee address.Address) (*paymentchannel.ChannelInfo, error)
	CreatePaymentChannel(payer, payee address.Address) (address.Address, error)
	AddVoucherToChannel(paychAddr address.Address, voucher *paychActor.SignedVoucher) error
	AddVoucher(paychAddr address.Address, voucher *paychActor.SignedVoucher, proof []byte) (abi.TokenAmount, error)
}
