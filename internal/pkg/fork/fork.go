package fork

import (
	//"bytes"
	"context"
	//"encoding/binary"

	//"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	//"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-state-types/abi"
	//"github.com/filecoin-project/go-state-types/big"
	//"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	//"github.com/filecoin-project/specs-actors/actors/builtin"
	//initBuilder "github.com/filecoin-project/specs-actors/actors/builtin/init"
	//"github.com/filecoin-project/specs-actors/actors/builtin/miner"
	//"github.com/filecoin-project/specs-actors/actors/builtin/multisig"
	//power0 "github.com/filecoin-project/specs-actors/actors/builtin/power"
	//"github.com/filecoin-project/specs-actors/actors/util/adt"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cbornode "github.com/ipfs/go-ipld-cbor"
	//xerrors "github.com/pkg/errors"
	//"github.com/prometheus/common/log"
)

// todo 多处引用了lotus

type chainReader interface {
	GetTipSet(tsKey block.TipSetKey) (*block.TipSet, error)
	GetTipSetStateRoot(ctx context.Context, tipKey block.TipSetKey) (cid.Cid, error)
}
type IFork interface {
	HandleStateForks(ctx context.Context, root cid.Cid, height abi.ChainEpoch, ts *block.TipSet) (cid.Cid, error)
}

var _ = IFork((*ChainFork)(nil))

type ChainFork struct {
	ForksAtHeight map[abi.ChainEpoch]func(context.Context, cid.Cid, *block.TipSet) (cid.Cid, error)
	state         chainReader
	bstore        blockstore.Blockstore
	ipldstore     cbornode.IpldStore
}

func NewChainFork(state chainReader, ipldstore cbornode.IpldStore, bstore blockstore.Blockstore) *ChainFork {
	fork := &ChainFork{
		state:     state,
		bstore:    bstore,
		ipldstore: ipldstore,
	}

	fork.ForksAtHeight = map[abi.ChainEpoch]func(context.Context, cid.Cid, *block.TipSet) (cid.Cid, error){
		//  chain.UpgradeBreezeHeight: fork.UpgradeFaucetBurnRecovery,
		//	build.UpgradeIgnitionHeight: UpgradeIgnition,
		//	build.UpgradeLiftoffHeight:  UpgradeLiftoff,
	}
	return fork
}


func (fork *ChainFork) HandleStateForks(ctx context.Context, root cid.Cid, height abi.ChainEpoch, ts *block.TipSet) (cid.Cid, error) {
	retCid := root
	var err error
	f, ok := fork.ForksAtHeight[height]
	if ok {
		retCid, err = f(ctx, root, ts)
		if err != nil {
			return cid.Undef, err
		}
	}

	return retCid, nil
}

//func doTransfer(tree types.StateTree, from, to address.Address, amt abi.TokenAmount) error {
//	fromAct, err := tree.GetActor(from)
//	if err != nil {
//		return xerrors.Errorf("failed to get 'from' actor for transfer: %w", err)
//	}
//
//	fromAct.Balance = types.BigSub(fromAct.Balance, amt)
//	if fromAct.Balance.Sign() < 0 {
//		return xerrors.Errorf("(sanity) deducted more funds from target account than it had (%s, %s)", from, types.FIL(amt))
//	}
//
//	if err := tree.SetActor(from, fromAct); err != nil {
//		return xerrors.Errorf("failed to persist from actor: %w", err)
//	}
//
//	toAct, err := tree.GetActor(to)
//	if err != nil {
//		return xerrors.Errorf("failed to get 'to' actor for transfer: %w", err)
//	}
//
//	toAct.Balance = types.BigAdd(toAct.Balance, amt)
//
//	if err := tree.SetActor(to, toAct); err != nil {
//		return xerrors.Errorf("failed to persist to actor: %w", err)
//	}
//	return nil
//}
//
//func (fork *ChainFork) UpgradeFaucetBurnRecovery(ctx context.Context, root cid.Cid, ts *block.TipSet) (cid.Cid, error) {
//	// Some initial parameters
//	FundsForMiners := types.FromFil(1_000_000)
//	LookbackEpoch := abi.ChainEpoch(32000)
//	AccountCap := types.FromFil(0)
//	BaseMinerBalance := types.FromFil(20)
//	DesiredReimbursementBalance := types.FromFil(5_000_000)
//
//	isSystemAccount := func(addr address.Address) (bool, error) {
//		id, err := address.IDFromAddress(addr)
//		if err != nil {
//			return false, xerrors.Errorf("id address: %w", err)
//		}
//
//		if id < 1000 {
//			return true, nil
//		}
//		return false, nil
//	}
//
//	minerFundsAlloc := func(pow, tpow abi.StoragePower) abi.TokenAmount {
//		return types.BigDiv(types.BigMul(pow, FundsForMiners), tpow)
//	}
//
//	// Grab lookback state for account checks
//	lbts, err := chain.FindTipsetAtEpoch(ctx, ts, LookbackEpoch, fork.state)
//	if err != nil {
//		return cid.Undef, xerrors.Errorf("failed to get tipset at lookback height: %w", err)
//	}
//
//	lbtsRoot, err := fork.state.GetTipSetStateRoot(ctx, lbts.Key())
//	if err != nil {
//		return cid.Undef, xerrors.Errorf("loading state tree failed: %w", err)
//	}
//
//	lbtree, err := state.LoadStateTree(fork.ipldstore, lbtsRoot)
//	if err != nil {
//		return cid.Undef, xerrors.Errorf("getting state tree: %w", err)
//	}
//
//	ReserveAddress, err := address.NewFromString("t090")
//	if err != nil {
//		return cid.Undef, xerrors.Errorf("failed to parse reserve address: %w", err)
//	}
//
//	tree, err := state.LoadStateTree(fork.ipldstore, root)
//	if err != nil {
//		return cid.Undef, xerrors.Errorf("getting state tree: %w", err)
//	}
//
//	type transfer struct {
//		From address.Address
//		To   address.Address
//		Amt  abi.TokenAmount
//	}
//
//	var transfers []transfer
//
//	// Take all excess funds away, put them into the reserve account
//	err = tree.ForEach(func(addr address.Address, act *types.Actor) error {
//		switch act.Code {
//		case builtin.AccountActorCodeID, builtin.MultisigActorCodeID, builtin.PaymentChannelActorCodeID:
//			sysAcc, err := isSystemAccount(addr)
//			if err != nil {
//				return xerrors.Errorf("checking system account: %w", err)
//			}
//
//			if !sysAcc {
//				transfers = append(transfers, transfer{
//					From: addr,
//					To:   ReserveAddress,
//					Amt:  act.Balance,
//				})
//			}
//		case builtin.StorageMinerActorCodeID:
//			var st miner.State
//			if err := fork.ipldstore.Get(ctx, act.Head, &st); err != nil {
//				return xerrors.Errorf("failed to load miner state: %w", err)
//			}
//
//			var available abi.TokenAmount
//			{
//				defer func() {
//					if err := recover(); err != nil {
//						log.Warnf("Get available balance failed (%s, %s, %s): %s", addr, act.Head, act.Balance, err)
//					}
//					available = abi.NewTokenAmount(0)
//				}()
//				// this panics if the miner doesnt have enough funds to cover their locked pledge
//				available = st.GetAvailableBalance(act.Balance)
//			}
//
//			transfers = append(transfers, transfer{
//				From: addr,
//				To:   ReserveAddress,
//				Amt:  available,
//			})
//		}
//		return nil
//	})
//	if err != nil {
//		return cid.Undef, xerrors.Errorf("foreach over state tree failed: %w", err)
//	}
//
//	// Execute transfers from previous step
//	for _, t := range transfers {
//		if err := doTransfer(tree, t.From, t.To, t.Amt); err != nil {
//			return cid.Undef, xerrors.Errorf("transfer %s %s->%s failed: %w", t.Amt, t.From, t.To, err)
//		}
//	}
//
//	// pull up power table to give miners back some funds proportional to their power
//	var ps power0.State
//	powAct, err := tree.GetActor(builtin.StoragePowerActorAddr)
//	if err != nil {
//		return cid.Undef, xerrors.Errorf("failed to load power actor: %w", err)
//	}
//
//	cst := cbornode.NewCborStore(fork.bstore)
//	if err := cst.Get(ctx, powAct.Head, &ps); err != nil {
//		return cid.Undef, xerrors.Errorf("failed to get power actor state: %w", err)
//	}
//
//	totalPower := ps.TotalBytesCommitted
//
//	var transfersBack []transfer
//	// Now, we return some funds to places where they are needed
//	err = tree.ForEach(func(addr address.Address, act *types.Actor) error {
//		lbact, err := lbtree.GetActor(addr)
//		if err != nil {
//			if !xerrors.Is(err, types.ErrActorNotFound) {
//				return xerrors.Errorf("failed to get actor in lookback state")
//			}
//		}
//
//		prevBalance := abi.NewTokenAmount(0)
//		if lbact != nil {
//			prevBalance = lbact.Balance
//		}
//
//		switch act.Code {
//		case builtin.AccountActorCodeID, builtin.MultisigActorCodeID, builtin.PaymentChannelActorCodeID:
//			nbalance := big.Min(prevBalance, AccountCap)
//			if nbalance.Sign() != 0 {
//				transfersBack = append(transfersBack, transfer{
//					From: ReserveAddress,
//					To:   addr,
//					Amt:  nbalance,
//				})
//			}
//		case builtin.StorageMinerActorCodeID:
//			var st miner.State
//			if err := fork.ipldstore.Get(ctx, act.Head, &st); err != nil {
//				return xerrors.Errorf("failed to load miner state: %w", err)
//			}
//
//			var minfo miner.MinerInfo
//			if err := cst.Get(ctx, st.Info, &minfo); err != nil {
//				return xerrors.Errorf("failed to get miner info: %w", err)
//			}
//
//			sectorsArr, err := adt.AsArray(adt.WrapStore(context.TODO(), fork.ipldstore), st.Sectors)
//			if err != nil {
//				return xerrors.Errorf("failed to load sectors array: %w", err)
//			}
//
//			slen := sectorsArr.Length()
//
//			power := types.BigMul(types.NewInt(slen), types.NewInt(uint64(minfo.SectorSize)))
//
//			mfunds := minerFundsAlloc(power, totalPower)
//			transfersBack = append(transfersBack, transfer{
//				From: ReserveAddress,
//				To:   minfo.Worker,
//				Amt:  mfunds,
//			})
//
//			// Now make sure to give each miner who had power at the lookback some FIL
//			lbact, err := lbtree.GetActor(addr)
//			if err == nil {
//				var lbst miner.State
//				if err := fork.ipldstore.Get(ctx, lbact.Head, &lbst); err != nil {
//					return xerrors.Errorf("failed to load miner state: %w", err)
//				}
//
//				lbsectors, err := adt.AsArray(adt.WrapStore(context.TODO(), fork.ipldstore), lbst.Sectors)
//				if err != nil {
//					return xerrors.Errorf("failed to load lb sectors array: %w", err)
//				}
//
//				if lbsectors.Length() > 0 {
//					transfersBack = append(transfersBack, transfer{
//						From: ReserveAddress,
//						To:   minfo.Worker,
//						Amt:  BaseMinerBalance,
//					})
//				}
//
//			} else {
//				log.Warnf("failed to get miner in lookback state: %s", err)
//			}
//		}
//		return nil
//	})
//	if err != nil {
//		return cid.Undef, xerrors.Errorf("foreach over state tree failed: %w", err)
//	}
//
//	for _, t := range transfersBack {
//		if err := doTransfer(tree, t.From, t.To, t.Amt); err != nil {
//			return cid.Undef, xerrors.Errorf("transfer %s %s->%s failed: %w", t.Amt, t.From, t.To, err)
//		}
//	}
//
//	// transfer all burnt funds back to the reserve account
//	burntAct, err := tree.GetActor(builtin.BurntFundsActorAddr)
//	if err != nil {
//		return cid.Undef, xerrors.Errorf("failed to load burnt funds actor: %w", err)
//	}
//	if err := doTransfer(tree, builtin.BurntFundsActorAddr, ReserveAddress, burntAct.Balance); err != nil {
//		return cid.Undef, xerrors.Errorf("failed to unburn funds: %w", err)
//	}
//
//	// Top up the reimbursement service
//	reimbAddr, err := address.NewFromString("t0111")
//	if err != nil {
//		return cid.Undef, xerrors.Errorf("failed to parse reimbursement service address")
//	}
//
//	reimb, err := tree.GetActor(reimbAddr)
//	if err != nil {
//		return cid.Undef, xerrors.Errorf("failed to load reimbursement account actor: %w", err)
//	}
//
//	difference := types.BigSub(DesiredReimbursementBalance, reimb.Balance)
//	if err := doTransfer(tree, ReserveAddress, reimbAddr, difference); err != nil {
//		return cid.Undef, xerrors.Errorf("failed to top up reimbursement account: %w", err)
//	}
//
//	// Now, a final sanity check to make sure the balances all check out
//	total := abi.NewTokenAmount(0)
//	err = tree.ForEach(func(addr address.Address, act *types.Actor) error {
//		total = types.BigAdd(total, act.Balance)
//		return nil
//	})
//	if err != nil {
//		return cid.Undef, xerrors.Errorf("checking final state balance failed: %w", err)
//	}
//
//	exp := types.FromFil(types.FilBase)
//	if !exp.Equals(total) {
//		return cid.Undef, xerrors.Errorf("resultant state tree account balance was not correct: %s", total)
//	}
//
//	return tree.Flush(ctx)
//}
//
//func (fork *ChainFork) UpgradeIgnition(ctx context.Context, root cid.Cid, ts *block.TipSet) (cid.Cid, error) {
//	/*	store := adt.WrapStore(context.TODO(), fork.state.IpldStore)
//
//		nst, err := nv3.MigrateStateTree(ctx, store, root, build.UpgradeIgnitionHeight)
//		if err != nil {
//			return cid.Undef, xerrors.Errorf("migrating actors state: %w", err)
//		}
//
//		tree, err := sm.StateTree(nst)
//		if err != nil {
//			return cid.Undef, xerrors.Errorf("getting state tree: %w", err)
//		}
//
//		err = setNetworkName(ctx, store, tree, "ignition")
//		if err != nil {
//			return cid.Undef, xerrors.Errorf("setting network name: %w", err)
//		}
//
//		split1, err := address.NewFromString("t0115")
//		if err != nil {
//			return cid.Undef, xerrors.Errorf("first split address: %w", err)
//		}
//
//		split2, err := address.NewFromString("t0116")
//		if err != nil {
//			return cid.Undef, xerrors.Errorf("second split address: %w", err)
//		}
//
//		err = resetGenesisMsigs(ctx, sm, store, tree)
//		if err != nil {
//			return cid.Undef, xerrors.Errorf("resetting genesis msig start epochs: %w", err)
//		}
//
//		err = splitGenesisMultisig(ctx, cb, split1, store, tree, 50)
//		if err != nil {
//			return cid.Undef, xerrors.Errorf("splitting first msig: %w", err)
//		}
//
//		err = splitGenesisMultisig(ctx, cb, split2, store, tree, 50)
//		if err != nil {
//			return cid.Undef, xerrors.Errorf("splitting second msig: %w", err)
//		}
//
//		err = nv3.CheckStateTree(ctx, store, nst, build.UpgradeIgnitionHeight, builtin0.TotalFilecoin)
//		if err != nil {
//			return cid.Undef, xerrors.Errorf("sanity check after ignition upgrade failed: %w", err)
//		}
//
//		return tree.Flush(ctx)*/
//	panic("impl me")
//}
//
//func (fork *ChainFork) UpgradeLiftoff(ctx context.Context, root cid.Cid, ts *block.TipSet) (cid.Cid, error) {
//	tree, err := state.LoadStateTree(fork.ipldstore, root)
//	if err != nil {
//		return cid.Undef, xerrors.Errorf("getting state tree: %w", err)
//	}
//
//	err = setNetworkName(ctx, adt.WrapStore(context.TODO(), fork.ipldstore), tree, "mainnet")
//	if err != nil {
//		return cid.Undef, xerrors.Errorf("setting network name: %w", err)
//	}
//
//	return tree.Flush(ctx)
//}
//
//func setNetworkName(ctx context.Context, store adt.Store, tree *state.StateTree, name string) error {
//	ia, err := tree.GetActor(builtin.InitActorAddr)
//	if err != nil {
//		return xerrors.Errorf("getting init actor: %w", err)
//	}
//
//	var initState initBuilder.State
//	if err := store.Get(ctx, ia.Head, &initState); err != nil {
//		return xerrors.Errorf("reading init state: %w", err)
//	}
//
//	initState.NetworkName = name
//
//	ia.Head, err = store.Put(ctx, &initState)
//	if err != nil {
//		return xerrors.Errorf("writing new init state: %w", err)
//	}
//
//	if err := tree.SetActor(builtin.InitActorAddr, ia); err != nil {
//		return xerrors.Errorf("setting init actor: %w", err)
//	}
//
//	return nil
//}
//
//func splitGenesisMultisig(ctx context.Context, addr address.Address, store adt.Store, tree *state.StateTree, portions uint64) error {
//	if portions < 1 {
//		return xerrors.Errorf("cannot split into 0 portions")
//	}
//
//	mact, err := tree.GetActor(addr)
//	if err != nil {
//		return xerrors.Errorf("getting msig actor: %w", err)
//	}
//
//	mst := multisig.State{} //todo multi version
//	err = store.Get(store.Context(), mact.Head, &mst)
//	if err != nil {
//		return err
//	}
//
//	/*	thresh, err := mst.Threshold()
//		if err != nil {
//			return xerrors.Errorf("getting msig threshold: %w", err)
//		}
//
//		ibal, err := mst.InitialBalance()
//		if err != nil {
//			return xerrors.Errorf("getting msig initial balance: %w", err)
//		}
//
//		se, err := mst.StartEpoch()
//		if err != nil {
//			return xerrors.Errorf("getting msig start epoch: %w", err)
//		}
//
//		ud, err := mst.UnlockDuration()
//		if err != nil {
//			return xerrors.Errorf("getting msig unlock duration: %w", err)
//		}
//	*/
//	pending, err := adt.MakeEmptyMap(store).Root()
//	if err != nil {
//		return xerrors.Errorf("failed to create empty map: %w", err)
//	}
//
//	newIbal := big.Div(mst.InitialBalance, types.NewInt(portions))
//	newState := &multisig.State{
//		Signers:               mst.Signers,
//		NumApprovalsThreshold: mst.NumApprovalsThreshold,
//		NextTxnID:             0,
//		InitialBalance:        newIbal,
//		StartEpoch:            mst.StartEpoch,
//		UnlockDuration:        mst.UnlockDuration,
//		PendingTxns:           pending,
//	}
//
//	scid, err := store.Put(ctx, newState)
//	if err != nil {
//		return xerrors.Errorf("storing new state: %w", err)
//	}
//
//	newActor := types.Actor{
//		Code:    builtin.MultisigActorCodeID,
//		Head:    scid,
//		Nonce:   0,
//		Balance: big.Zero(),
//	}
//
//	i := uint64(0)
//	for i < portions {
//		keyAddr, err := makeKeyAddr(addr, i)
//		if err != nil {
//			return xerrors.Errorf("creating key address: %w", err)
//		}
//
//		idAddr, err := tree.RegisterNewAddress(keyAddr)
//		if err != nil {
//			return xerrors.Errorf("registering new address: %w", err)
//		}
//
//		err = tree.SetActor(idAddr, &newActor)
//		if err != nil {
//			return xerrors.Errorf("setting new msig actor state: %w", err)
//		}
//
//		if err := doTransfer(tree, addr, idAddr, newIbal); err != nil {
//			return xerrors.Errorf("transferring split msig balance: %w", err)
//		}
//
//		i++
//	}
//
//	return nil
//}
//
//func makeKeyAddr(splitAddr address.Address, count uint64) (address.Address, error) {
//	var b bytes.Buffer
//	if err := splitAddr.MarshalCBOR(&b); err != nil {
//		return address.Undef, xerrors.Errorf("marshalling split address: %w", err)
//	}
//
//	if err := binary.Write(&b, binary.BigEndian, count); err != nil {
//		return address.Undef, xerrors.Errorf("writing count into a buffer: %w", err)
//	}
//
//	if err := binary.Write(&b, binary.BigEndian, []byte("Ignition upgrade")); err != nil {
//		return address.Undef, xerrors.Errorf("writing fork name into a buffer: %w", err)
//	}
//
//	addr, err := address.NewActorAddress(b.Bytes())
//	if err != nil {
//		return address.Undef, xerrors.Errorf("create actor address: %w", err)
//	}
//
//	return addr, nil
//}
//
///*
//func resetGenesisMsigs(ctx context.Context, store adt.Store, tree *state.StateTree) error {
//	gb, err := sm.cs.GetGenesis()
//	if err != nil {
//		return xerrors.Errorf("getting genesis block: %w", err)
//	}
//
//	gts, err := types.NewTipSet([]*types.BlockHeader{gb})
//	if err != nil {
//		return xerrors.Errorf("getting genesis tipset: %w", err)
//	}
//
//	cst := cbor.NewCborStore(sm.cs.Blockstore())
//	genesisTree, err := state.LoadStateTree(cst, gts.ParentState())
//	if err != nil {
//		return xerrors.Errorf("loading state tree: %w", err)
//	}
//
//	err = genesisTree.ForEach(func(addr address.Address, genesisActor *types.Actor) error {
//		if genesisActor.Code == builtin0.MultisigActorCodeID {
//			currActor, err := tree.GetActor(addr)
//			if err != nil {
//				return xerrors.Errorf("loading actor: %w", err)
//			}
//
//			var currState multisig0.state
//			if err := store.Get(ctx, currActor.Head, &currState); err != nil {
//				return xerrors.Errorf("reading multisig state: %w", err)
//			}
//
//			currState.StartEpoch = build.UpgradeLiftoffHeight
//
//			currActor.Head, err = store.Put(ctx, &currState)
//			if err != nil {
//				return xerrors.Errorf("writing new multisig state: %w", err)
//			}
//
//			if err := tree.SetActor(addr, currActor); err != nil {
//				return xerrors.Errorf("setting multisig actor: %w", err)
//			}
//		}
//		return nil
//	})
//
//	if err != nil {
//		return xerrors.Errorf("iterating over genesis actors: %w", err)
//	}
//
//	return nil
//}*/
