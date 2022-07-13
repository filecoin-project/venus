package messagepool

import (
	"context"
	"errors"
	"fmt"
	"math"
	stdbig "math/big"
	"math/rand"
	"sort"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/exitcode"
	lru "github.com/hashicorp/golang-lru"

	builtin2 "github.com/filecoin-project/go-state-types/builtin"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/fork"
	"github.com/filecoin-project/venus/pkg/vm"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin"
	"github.com/filecoin-project/venus/venus-shared/types"
)

const MinGasPremium = 100e3

// const MaxSpendOnFeeDenom = 100

type GasPriceCache struct {
	c *lru.TwoQueueCache
}

type GasMeta struct {
	Price big.Int
	Limit int64
}

func NewGasPriceCache() *GasPriceCache {
	// 50 because we usually won't access more than 40
	c, err := lru.New2Q(50)
	if err != nil {
		// err only if parameter is bad
		panic(err)
	}

	return &GasPriceCache{
		c: c,
	}
}

func (g *GasPriceCache) GetTSGasStats(ctx context.Context, provider Provider, ts *types.TipSet) ([]GasMeta, error) {
	i, has := g.c.Get(ts.Key())
	if has {
		return i.([]GasMeta), nil
	}

	var prices []GasMeta
	msgs, err := provider.MessagesForTipset(ctx, ts)
	if err != nil {
		return nil, fmt.Errorf("loading messages: %w", err)
	}
	for _, msg := range msgs {
		prices = append(prices, GasMeta{
			Price: msg.VMMessage().GasPremium,
			Limit: msg.VMMessage().GasLimit,
		})
	}

	g.c.Add(ts.Key(), prices)

	return prices, nil
}

func (mp *MessagePool) GasEstimateFeeCap(
	ctx context.Context,
	msg *types.Message,
	maxqueueblks int64,
	tsk types.TipSetKey,
) (big.Int, error) {
	ts, err := mp.api.ChainHead(ctx)
	if err != nil {
		return types.NewGasFeeCap(0), err
	}

	parentBaseFee := ts.Blocks()[0].ParentBaseFee
	increaseFactor := math.Pow(1.+1./float64(constants.BaseFeeMaxChangeDenom), float64(maxqueueblks))

	feeInFuture := types.BigMul(parentBaseFee, types.NewInt(uint64(increaseFactor*(1<<8))))
	out := types.BigDiv(feeInFuture, types.NewInt(1<<8))

	if msg.GasPremium != types.EmptyInt {
		out = types.BigAdd(out, msg.GasPremium)
	}

	return out, nil
}

// finds 55th percntile instead of median to put negative pressure on gas price
func medianGasPremium(prices []GasMeta, blocks int) abi.TokenAmount {
	sort.Slice(prices, func(i, j int) bool {
		// sort desc by price
		return prices[i].Price.GreaterThan(prices[j].Price)
	})

	at := constants.BlockGasTarget * int64(blocks) / 2
	at += constants.BlockGasTarget * int64(blocks) / (2 * 20) // move 5% further
	prev1, prev2 := big.Zero(), big.Zero()
	for _, price := range prices {
		prev1, prev2 = price.Price, prev1
		at -= price.Limit
		if at < 0 {
			break
		}
	}

	premium := prev1
	if prev2.Sign() != 0 {
		premium = big.Div(big.Add(prev1, prev2), big.NewInt(2))
	}

	return premium
}

func (mp *MessagePool) GasEstimateGasPremium(
	ctx context.Context,
	nblocksincl uint64,
	sender address.Address,
	gaslimit int64,
	_ types.TipSetKey,
	cache *GasPriceCache,
) (big.Int, error) {
	if nblocksincl == 0 {
		nblocksincl = 1
	}

	var prices []GasMeta
	var blocks int

	ts, err := mp.api.ChainHead(ctx)
	if err != nil {
		return big.Int{}, err
	}

	for i := uint64(0); i < nblocksincl*2; i++ {
		if ts.Height() == 0 {
			break // genesis
		}

		pts, err := mp.api.LoadTipSet(ctx, ts.Parents())
		if err != nil {
			return types.BigInt{}, err
		}

		blocks += len(pts.Blocks())
		meta, err := cache.GetTSGasStats(ctx, mp.api, pts)
		if err != nil {
			return types.BigInt{}, err
		}
		prices = append(prices, meta...)

		ts = pts
	}

	premium := medianGasPremium(prices, blocks)

	if big.Cmp(premium, big.NewInt(MinGasPremium)) < 0 {
		switch nblocksincl {
		case 1:
			premium = big.NewInt(2 * MinGasPremium)
		case 2:
			premium = big.NewInt(1.5 * MinGasPremium)
		default:
			premium = big.NewInt(MinGasPremium)
		}
	}

	// add some noise to normalize behaviour of message selection
	const precision = 32
	// mean 1, stddev 0.005 => 95% within +-1%
	noise := 1 + rand.NormFloat64()*0.005
	premium = types.BigMul(premium, types.NewInt(uint64(noise*(1<<precision))+1))
	premium = types.BigDiv(premium, types.NewInt(1<<precision))
	return premium, nil
}

func (mp *MessagePool) GasEstimateGasLimit(ctx context.Context, msgIn *types.Message, tsk types.TipSetKey) (int64, error) {
	if tsk.IsEmpty() {
		ts, err := mp.api.ChainHead(ctx)
		if err != nil {
			return -1, fmt.Errorf("getting head: %v", err)
		}
		tsk = ts.Key()
	}
	currTS, err := mp.api.ChainTipSet(ctx, tsk)
	if err != nil {
		return -1, fmt.Errorf("getting tipset: %w", err)
	}

	msg := *msgIn
	msg.GasLimit = constants.BlockGasLimit
	msg.GasFeeCap = big.NewInt(int64(constants.MinimumBaseFee) + 1)
	msg.GasPremium = big.NewInt(1)

	fromA, err := mp.sm.ResolveToKeyAddress(ctx, msgIn.From, currTS)
	if err != nil {
		return -1, fmt.Errorf("getting key address: %w", err)
	}

	pending, ts := mp.PendingFor(ctx, fromA)
	priorMsgs := make([]types.ChainMsg, 0, len(pending))
	for _, m := range pending {
		if m.Message.Nonce == msg.Nonce {
			break
		}
		priorMsgs = append(priorMsgs, m)
	}

	return mp.evalMessageGasLimit(ctx, msgIn, priorMsgs, ts)
}

func (mp *MessagePool) evalMessageGasLimit(ctx context.Context, msgIn *types.Message, priorMsgs []types.ChainMsg, ts *types.TipSet) (int64, error) {
	msg := *msgIn
	msg.GasLimit = constants.BlockGasLimit
	msg.GasFeeCap = big.NewInt(int64(constants.MinimumBaseFee) + 1)
	msg.GasPremium = big.NewInt(1)

	// Try calling until we find a height with no migration.
	var res *vm.Ret
	var err error
	for {
		res, err = mp.sm.CallWithGas(ctx, &msg, priorMsgs, ts)
		if err != fork.ErrExpensiveFork {
			break
		}

		ts, err = mp.api.ChainTipSet(ctx, ts.Parents())
		if err != nil {
			return -1, fmt.Errorf("getting parent tipset: %v", err)
		}
	}
	if err != nil {
		return -1, fmt.Errorf("CallWithGas failed: %v", err)
	}
	if res.Receipt.ExitCode != exitcode.Ok {
		log.Warnf("message execution failed: from %v, method %d, exit %s, reason: %v", msg.From, msg.Method, res.Receipt.ExitCode, res.ActorErr)
		return -1, fmt.Errorf("message execution failed: exit %s, reason: %v", res.Receipt.ExitCode, res.ActorErr)
	}

	ret := res.Receipt.GasUsed

	transitionalMulti := 1.0
	// Overestimate gas around the upgrade
	if ts.Height() <= mp.forkParams.UpgradeSkyrHeight && (mp.forkParams.UpgradeSkyrHeight-ts.Height() <= 20) {
		transitionalMulti = 2.0

		func() {
			_, st, err := mp.sm.ParentState(ctx, ts)
			if err != nil {
				return
			}
			act, found, err := st.GetActor(ctx, msg.To)
			if err != nil {
				return
			}
			if !found {
				return
			}

			if builtin.IsStorageMinerActor(act.Code) {
				switch msgIn.Method {
				case 5:
					transitionalMulti = 3.954
				case 6:
					transitionalMulti = 4.095
				case 7:
					// skip, stay at 2.0
					//transitionalMulti = 1.289
				case 11:
					transitionalMulti = 17.8758
				case 16:
					transitionalMulti = 2.1704
				case 25:
					transitionalMulti = 3.1177
				case 26:
					transitionalMulti = 2.3322
				default:
				}
			}

			// skip storage market, 80th percentie for everything ~1.9, leave it at 2.0
		}()
		log.Infof("overestimate gas around the upgrade msg: %v, transitional multi: %v", msg, transitionalMulti)
	}
	ret = (ret * int64(transitionalMulti*1024)) >> 10

	// Special case for PaymentChannel collect, which is deleting actor
	// We ignore errors in this special case since they CAN occur,
	// and we just want to detect existing payment channel actors
	_, st, err := mp.sm.ParentState(ctx, ts)
	if err == nil {
		act, found, err := st.GetActor(ctx, msg.To)
		if err == nil && found && builtin.IsPaymentChannelActor(act.Code) && msgIn.Method == builtin2.MethodsPaych.Collect {
			// add the refunded gas for DestroyActor back into the gas used
			ret += 76e3
		}
	}

	return ret, nil
}

func (mp *MessagePool) GasEstimateMessageGas(ctx context.Context, estimateMessage *types.EstimateMessage, _ types.TipSetKey) (*types.Message, error) {
	if estimateMessage == nil || estimateMessage.Msg == nil {
		return nil, fmt.Errorf("estimate message is nil")
	}
	log.Debugf("call GasEstimateMessageGas %v, send spec: %v", estimateMessage.Msg, estimateMessage.Spec)
	if estimateMessage.Msg.GasLimit == 0 {
		gasLimit, err := mp.GasEstimateGasLimit(ctx, estimateMessage.Msg, types.TipSetKey{})
		if err != nil {
			return nil, fmt.Errorf("estimating gas used: %w", err)
		}
		gasLimitOverestimation := mp.GetConfig().GasLimitOverestimation
		if estimateMessage.Spec != nil && estimateMessage.Spec.GasOverEstimation > 0 {
			gasLimitOverestimation = estimateMessage.Spec.GasOverEstimation
		}
		estimateMessage.Msg.GasLimit = int64(float64(gasLimit) * gasLimitOverestimation)
	}

	if estimateMessage.Msg.GasPremium == types.EmptyInt || types.BigCmp(estimateMessage.Msg.GasPremium, types.NewInt(0)) == 0 {
		gasPremium, err := mp.GasEstimateGasPremium(ctx, 10, estimateMessage.Msg.From, estimateMessage.Msg.GasLimit, types.TipSetKey{}, mp.PriceCache)
		if err != nil {
			return nil, fmt.Errorf("estimating gas price: %w", err)
		}
		if estimateMessage.Spec != nil && estimateMessage.Spec.GasOverPremium > 0 {
			olgGasPremium := gasPremium
			newGasPremium, _ := new(stdbig.Float).Mul(new(stdbig.Float).SetInt(stdbig.NewInt(gasPremium.Int64())), stdbig.NewFloat(estimateMessage.Spec.GasOverPremium)).Int(nil)
			gasPremium = big.NewFromGo(newGasPremium)
			log.Debugf("call GasEstimateMessageGas old premium %v, new premium %v, premium ration %f", olgGasPremium, newGasPremium, estimateMessage.Spec.GasOverPremium)
		}
		estimateMessage.Msg.GasPremium = gasPremium
	}

	if estimateMessage.Msg.GasFeeCap == types.EmptyInt || types.BigCmp(estimateMessage.Msg.GasFeeCap, types.NewInt(0)) == 0 {
		feeCap, err := mp.GasEstimateFeeCap(ctx, estimateMessage.Msg, 20, types.EmptyTSK)
		if err != nil {
			return nil, fmt.Errorf("estimating fee cap: %w", err)
		}
		estimateMessage.Msg.GasFeeCap = feeCap
	}

	CapGasFee(mp.GetMaxFee, estimateMessage.Msg, estimateMessage.Spec)

	return estimateMessage.Msg, nil
}

func (mp *MessagePool) GasBatchEstimateMessageGas(ctx context.Context, estimateMessages []*types.EstimateMessage, fromNonce uint64, tsk types.TipSetKey) ([]*types.EstimateResult, error) {
	if len(estimateMessages) == 0 {
		return nil, errors.New("estimate messages are empty")
	}

	// ChainTipSet will determine if tsk is empty
	currTS, err := mp.api.ChainTipSet(ctx, tsk)
	if err != nil {
		return nil, fmt.Errorf("getting tipset: %w", err)
	}

	fromA, err := mp.sm.ResolveToKeyAddress(ctx, estimateMessages[0].Msg.From, currTS)
	if err != nil {
		return nil, fmt.Errorf("getting key address: %w", err)
	}

	pending, ts := mp.PendingFor(ctx, fromA)
	priorMsgs := make([]types.ChainMsg, 0, len(pending))
	for _, m := range pending {
		priorMsgs = append(priorMsgs, m)
	}

	var estimateResults []*types.EstimateResult
	for _, estimateMessage := range estimateMessages {
		estimateMsg := estimateMessage.Msg
		estimateMsg.Nonce = fromNonce

		log.Debugf("call GasBatchEstimateMessageGas msg %v, spec %v", estimateMsg, estimateMessage.Spec)

		if estimateMsg.GasLimit == 0 {
			gasUsed, err := mp.evalMessageGasLimit(ctx, estimateMsg, priorMsgs, ts)
			if err != nil {
				estimateMsg.Nonce = 0
				estimateResults = append(estimateResults, &types.EstimateResult{
					Msg: estimateMsg,
					Err: fmt.Sprintf("estimating gas limit: %v", err),
				})
				continue
			}
			estimateMsg.GasLimit = int64(float64(gasUsed) * estimateMessage.Spec.GasOverEstimation)
		}

		if estimateMsg.GasPremium == types.EmptyInt || types.BigCmp(estimateMsg.GasPremium, types.NewInt(0)) == 0 {
			gasPremium, err := mp.GasEstimateGasPremium(ctx, 10, estimateMsg.From, estimateMsg.GasLimit, types.TipSetKey{}, mp.PriceCache)
			if err != nil {
				estimateMsg.Nonce = 0
				estimateResults = append(estimateResults, &types.EstimateResult{
					Msg: estimateMsg,
					Err: fmt.Sprintf("estimating gas premium: %v", err),
				})
				continue
			}
			if estimateMessage.Spec != nil && estimateMessage.Spec.GasOverPremium > 0 {
				olgGasPremium := gasPremium
				newGasPremium, _ := new(stdbig.Float).Mul(new(stdbig.Float).SetInt(stdbig.NewInt(gasPremium.Int64())), stdbig.NewFloat(estimateMessage.Spec.GasOverPremium)).Int(nil)
				gasPremium = big.NewFromGo(newGasPremium)
				log.Debugf("call GasBatchEstimateMessageGas old premium %v, new premium %v, premium ration %f", olgGasPremium, newGasPremium, estimateMessage.Spec.GasOverPremium)
			}
			estimateMsg.GasPremium = gasPremium
		}

		if estimateMsg.GasFeeCap == types.EmptyInt || types.BigCmp(estimateMsg.GasFeeCap, types.NewInt(0)) == 0 {
			feeCap, err := mp.GasEstimateFeeCap(ctx, estimateMsg, 20, types.EmptyTSK)
			if err != nil {
				estimateMsg.Nonce = 0
				estimateResults = append(estimateResults, &types.EstimateResult{
					Msg: estimateMsg,
					Err: fmt.Sprintf("estimating fee cap: %v", err),
				})
				continue
			}
			estimateMsg.GasFeeCap = feeCap
		}

		CapGasFee(mp.GetMaxFee, estimateMsg, estimateMessage.Spec)

		estimateResults = append(estimateResults, &types.EstimateResult{
			Msg: estimateMsg,
		})
		priorMsgs = append(priorMsgs, estimateMsg)
		fromNonce++
	}
	return estimateResults, nil
}
