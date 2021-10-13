package paychmgr

import (
	"context"
	"errors"
	crypto2 "github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/vm"
	"sync"

	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/types/specactors/builtin/market"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/go-state-types/network"

	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/types/specactors/builtin/paych"
)

type mockManagerAPI struct {
	*mockStateManager
	*mockPaychAPI
}

func (m mockManagerAPI) GetMarketState(ctx context.Context, ts *types.TipSet) (market.State, error) {
	return nil, nil
}

func newMockManagerAPI() *mockManagerAPI {
	return &mockManagerAPI{
		mockStateManager: newMockStateManager(),
		mockPaychAPI:     newMockPaychAPI(),
	}
}

type mockPchState struct {
	actor *types.Actor
	state paych.State
}

type mockStateManager struct {
	lk           sync.Mutex
	accountState map[address.Address]address.Address
	paychState   map[address.Address]mockPchState
	response     *vm.Ret
	lastCall     *types.UnsignedMessage
}

func newMockStateManager() *mockStateManager {
	return &mockStateManager{
		accountState: make(map[address.Address]address.Address),
		paychState:   make(map[address.Address]mockPchState),
	}
}

func (sm *mockStateManager) setAccountAddress(a address.Address, lookup address.Address) {
	sm.lk.Lock()
	defer sm.lk.Unlock()
	sm.accountState[a] = lookup
}

func (sm *mockStateManager) setPaychState(a address.Address, actor *types.Actor, state paych.State) {
	sm.lk.Lock()
	defer sm.lk.Unlock()
	sm.paychState[a] = mockPchState{actor, state}
}

func (sm *mockStateManager) ResolveToKeyAddress(ctx context.Context, addr address.Address, ts *types.TipSet) (address.Address, error) {
	sm.lk.Lock()
	defer sm.lk.Unlock()
	keyAddr, ok := sm.accountState[addr]
	if !ok {
		return address.Undef, errors.New("not found")
	}
	return keyAddr, nil
}

func (sm *mockStateManager) GetPaychState(ctx context.Context, addr address.Address, ts *types.TipSet) (*types.Actor, paych.State, error) {
	sm.lk.Lock()
	defer sm.lk.Unlock()
	info, ok := sm.paychState[addr]
	if !ok {
		return nil, nil, errors.New("not found")
	}
	return info.actor, info.state, nil
}

func (sm *mockStateManager) setCallResponse(response *vm.Ret) {
	sm.lk.Lock()
	defer sm.lk.Unlock()

	sm.response = response
}

func (sm *mockStateManager) getLastCall() *types.UnsignedMessage {
	sm.lk.Lock()
	defer sm.lk.Unlock()

	return sm.lastCall
}

func (sm *mockStateManager) Call(ctx context.Context, msg *types.UnsignedMessage, ts *types.TipSet) (*vm.Ret, error) {
	sm.lk.Lock()
	defer sm.lk.Unlock()

	sm.lastCall = msg

	return sm.response, nil
}

type waitingCall struct {
	response chan types.MessageReceipt
}

type waitingResponse struct {
	receipt types.MessageReceipt
	done    chan struct{}
}

type mockPaychAPI struct {
	lk               sync.Mutex
	messages         map[cid.Cid]*types.SignedMessage
	waitingCalls     map[cid.Cid]*waitingCall
	waitingResponses map[cid.Cid]*waitingResponse
	wallet           map[address.Address]struct{}
	signingKey       []byte
}

func newMockPaychAPI() *mockPaychAPI {
	return &mockPaychAPI{
		messages:         make(map[cid.Cid]*types.SignedMessage),
		waitingCalls:     make(map[cid.Cid]*waitingCall),
		waitingResponses: make(map[cid.Cid]*waitingResponse),
		wallet:           make(map[address.Address]struct{}),
	}
}

func (pchapi *mockPaychAPI) StateWaitMsg(ctx context.Context, mcid cid.Cid, confidence uint64) (*chain.MsgLookup, error) {
	pchapi.lk.Lock()
	response := make(chan types.MessageReceipt)

	if response, ok := pchapi.waitingResponses[mcid]; ok {
		defer pchapi.lk.Unlock()
		defer func() {
			go close(response.done)
		}()

		delete(pchapi.waitingResponses, mcid)
		return &chain.MsgLookup{Receipt: response.receipt}, nil
	}

	pchapi.waitingCalls[mcid] = &waitingCall{response: response}
	pchapi.lk.Unlock()

	receipt := <-response
	return &chain.MsgLookup{Receipt: receipt}, nil
}

func (pchapi *mockPaychAPI) receiveMsgResponse(mcid cid.Cid, receipt types.MessageReceipt) {
	pchapi.lk.Lock()

	if call, ok := pchapi.waitingCalls[mcid]; ok {
		defer pchapi.lk.Unlock()

		delete(pchapi.waitingCalls, mcid)
		call.response <- receipt
		return
	}

	done := make(chan struct{})
	pchapi.waitingResponses[mcid] = &waitingResponse{receipt: receipt, done: done}

	pchapi.lk.Unlock()

	<-done
}

// Send success response for any waiting calls
func (pchapi *mockPaychAPI) close() {
	pchapi.lk.Lock()
	defer pchapi.lk.Unlock()

	success := types.MessageReceipt{
		ExitCode:    0,
		ReturnValue: []byte{},
	}
	for mcid, call := range pchapi.waitingCalls {
		delete(pchapi.waitingCalls, mcid)
		call.response <- success
	}
}

func (pchapi *mockPaychAPI) MpoolPushMessage(ctx context.Context, msg *types.UnsignedMessage, spec *types.MessageSendSpec) (*types.SignedMessage, error) {
	pchapi.lk.Lock()
	defer pchapi.lk.Unlock()

	smsg := &types.SignedMessage{Message: *msg}
	smsgCid := smsg.Cid()
	pchapi.messages[smsgCid] = smsg
	return smsg, nil
}

func (pchapi *mockPaychAPI) pushedMessages(c cid.Cid) *types.SignedMessage {
	pchapi.lk.Lock()
	defer pchapi.lk.Unlock()

	return pchapi.messages[c]
}

func (pchapi *mockPaychAPI) pushedMessageCount() int {
	pchapi.lk.Lock()
	defer pchapi.lk.Unlock()

	return len(pchapi.messages)
}

func (pchapi *mockPaychAPI) StateAccountKey(ctx context.Context, addr address.Address, tsk types.TipSetKey) (address.Address, error) {
	return addr, nil
}

func (pchapi *mockPaychAPI) WalletHas(ctx context.Context, addr address.Address) (bool, error) {
	pchapi.lk.Lock()
	defer pchapi.lk.Unlock()

	_, ok := pchapi.wallet[addr]
	return ok, nil
}

func (pchapi *mockPaychAPI) addWalletAddress(addr address.Address) {
	pchapi.lk.Lock()
	defer pchapi.lk.Unlock()

	pchapi.wallet[addr] = struct{}{}
}

func (pchapi *mockPaychAPI) WalletSign(ctx context.Context, k address.Address, msg []byte) (*crypto.Signature, error) {
	pchapi.lk.Lock()
	defer pchapi.lk.Unlock()

	return crypto2.Sign(msg, pchapi.signingKey, crypto.SigTypeSecp256k1)
}

func (pchapi *mockPaychAPI) addSigningKey(key []byte) {
	pchapi.lk.Lock()
	defer pchapi.lk.Unlock()

	pchapi.signingKey = key
}

func (pchapi *mockPaychAPI) StateNetworkVersion(ctx context.Context, tsk types.TipSetKey) (network.Version, error) {
	return constants.NewestNetworkVersion, nil
}
