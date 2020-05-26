package testing

import (
	"context"
	"math/big"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/shared_testutil"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
)

// FakePaychActorUtil fulfils the MsgSender and MsgWaiter interfaces for a Manager
// via the specs_actors mock runtime. It executes paych.Actor exports directly.
type FakePaychActorUtil struct {
	*testing.T
	Balance                                         types.AttoFIL
	PaychAddr, PaychIDAddr, Client, ClientID, Miner address.Address
	SendErr                                         error
	result                                          MsgResult
}

// Send stubs a message Sender
func (fai *FakePaychActorUtil) Send(ctx context.Context,
	from, to address.Address,
	value types.AttoFIL,
	gasPrice types.AttoFIL,
	gasLimit gas.Unit,
	bcast bool,
	method abi.MethodNum,
	params interface{}) (mcid cid.Cid, pubErrCh chan error, err error) {

	if fai.result != msgRcptsUndef {
		mcid = fai.result.MsgCid
	}

	fai.doSend(value)
	return mcid, pubErrCh, fai.SendErr
}

// Wait stubs a message Waiter
func (fai *FakePaychActorUtil) Wait(_ context.Context, _ cid.Cid, cb func(*block.Block, *types.SignedMessage, *vm.MessageReceipt) error) error {
	res := fai.result
	return cb(res.Block, res.Msg, res.Rcpt)
}

// StubSendFundsMessage sets expectations for a message that just sends funds to the actor
func (fai *FakePaychActorUtil) StubSendFundsResponse(from address.Address, amt abi.TokenAmount, code exitcode.ExitCode, height int64) cid.Cid {
	newCID := shared_testutil.GenerateCids(1)[0]

	msg := types.NewUnsignedMessage(from, fai.PaychAddr, 1, amt, builtin.MethodSend, []byte{})
	msg.GasPrice = abi.NewTokenAmount(100)
	msg.GasLimit = gas.NewGas(5000)

	emptySig := crypto.Signature{Type: crypto.SigTypeBLS, Data: []byte{'0'}}
	fai.result = MsgResult{
		Block:         &block.Block{Height: abi.ChainEpoch(height)},
		Msg:           &types.SignedMessage{Message: *msg, Signature: emptySig},
		DecodedParams: nil,
		MsgCid:        newCID,
		Rcpt:          &vm.MessageReceipt{ExitCode: code},
	}
	return newCID
}

func (fai *FakePaychActorUtil) doSend(amt abi.TokenAmount) {
	require.Equal(fai, amt, fai.result.Msg.Message.Value)
	nb := big.NewInt(0).Add(fai.Balance.Int, amt.Int)
	fai.Balance = types.NewAttoFIL(nb)
}
